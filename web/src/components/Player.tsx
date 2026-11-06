import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AnimatePresence, motion } from 'motion/react';
import type HlsType from 'hls.js';
import { AdaptiveQuality } from '@/lib/adaptive-quality';
import {
  ListVideo, ArrowUp, ArrowDown, Trash2,
  AlertCircle,
  Airplay,
  Cast,
  Check,
  Captions,
  Film,
  Gauge,
  ListOrdered,
  SlidersHorizontal,
  Info,
  Languages,
  Loader2,
  Maximize,
  PictureInPicture2,
  Minimize,
  OctagonX,
  Pause,
  Play,
  RotateCcw,
  RotateCw,
  SkipBack,
  SkipForward,
  Volume1,
  Volume2,
  VolumeX,
  X,
} from 'lucide-react';
import { api, ApiError, type PlayQueue, type PlayQueueEntry, type SubtitleStyle, type Chapter, type Item, type PlaybackPlan, type PlayerCaps, type SubtitleCue, type SubtitleTrack, type TrickplayInfo } from '@/lib/api';
import { episodeCode } from './WatchState';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import * as fmt from '@/lib/format';
import { audioLabel, chooseSubtitle, rememberedSubtitleLanguage, fallbackCaps, methodLabel, playerCaps, qualities, qualityName, readQuality, saveQuality, subtitleLabel, versionLabel } from '@/lib/playback';
import { cn } from '@/lib/utils';
import { Dialog, DialogContent } from './ui/dialog';
import { ErrorNotice } from './Page';
import { Button } from './ui/button';
import { SubtitleSearch } from './SubtitleSearch';
import { useAuth, useIsAdmin } from '@/lib/auth';
import { maxQueueEntries, QueueMirror } from '@/lib/play-queue';
import { correction } from '@/lib/watch-together';
import { CastSession, chooseChromecast, useCastAvailable, type CastText } from './Cast';
import { TogetherMenu, TogetherWaiting, useTogetherNotice, useWatchTogether } from './WatchTogether';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu';

export interface NowPlaying {
  title: string;
  subtitle?: string;
  /** The file to play directly (trailers and extras). Movies and episodes ask the server instead. */
  src?: string;
  /** The movie or episode, so the server can choose how to send it and progress is saved. */
  itemId?: string;
  /** Where to start, in milliseconds (a resume point). */
  startMS?: number;
  /** The edition to play when the title has several (e.g. "Director's Cut"). */
  edition?: string;
  /** A pre-roll video before a movie: it carries straight on, and can be skipped. */
  preroll?: boolean;
  /** Explicit preplay choices belong to this source file, never another episode/version. */
  selection?: { fileId: number; audioStream?: number; subtitle?: string | null; burnSubtitle?: string };
  /** Played in a Watch Together room: the room decides where and whether it plays. */
  together?: boolean;
}

interface QueuedItem extends NowPlaying {
  queueId: string;
  /** The server queue's entry for this title, once the server has answered. */
  entryId?: string;
  /** Sent to the server's queue (its entry id may still be on the way). */
  mirrored?: boolean;
  /** The server's queue holds a title that is gone or no longer allowed: shown, skipped, removable. */
  unavailable?: boolean;
}
interface QueueState { items: QueuedItem[]; index: number }
interface QueueControls {
  state: QueueState | null;
  /** Set when the server's queue changed elsewhere and the upcoming list was reloaded from it. */
  notice: string;
  enqueue: (items: NowPlaying[], next: boolean) => void;
  move: (id: string, delta: number) => void;
  remove: (id: string) => void;
  clear: () => void;
  jump: (id: string) => void;
  stop: () => Promise<void>;
}
const QueueContext = createContext<QueueControls>({ state: null, notice: '', enqueue: () => {}, move: () => {}, remove: () => {}, clear: () => {}, jump: () => {}, stop: async () => {} });
export const usePlaybackQueue = () => useContext(QueueContext);
let queueSequence = 0;
const queued = (item: NowPlaying): QueuedItem => ({ ...item, queueId: 'queue-' + (++queueSequence) });
const mirrorable = (item: NowPlaying) => !!item.itemId && !item.preroll && !item.together;

/** A server queue entry as something the player can play (or skip, when it is unavailable). */
function entryItem(entry: PlayQueueEntry): QueuedItem {
  const item = entry.available && entry.item ? nowPlayingFor(entry.item) : { title: i18n.t('player:player.unavailableEntry') };
  return { ...queued(item), entryId: entry.id, mirrored: true, unavailable: !entry.available || !entry.item };
}

/** Gives entry ids to the titles sent to the server that don't have one yet, in order. */
function withEntries(q: QueueState | null, server: PlayQueue): QueueState | null {
  if (!q) return q;
  const known = new Set(q.items.map((i) => i.entryId).filter(Boolean));
  const fresh = server.entries.filter((e) => !known.has(e.id));
  if (!fresh.length || !q.items.some((i) => i.mirrored && !i.entryId)) return q;
  let next = 0;
  return { ...q, items: q.items.map((i) => (i.mirrored && !i.entryId && fresh[next] ? { ...i, entryId: fresh[next++].id } : i)) };
}

/** Follows the server's queue for what comes after the current title, which carries on playing. */
function reconciled(q: QueueState | null, server: PlayQueue | null): QueueState | null {
  if (!q || !server) return q;
  const current = q.items[q.index];
  const anchor = current?.entryId ?? server.current_entry_id;
  const at = server.entries.findIndex((e) => e.id === anchor);
  const upcoming = at >= 0 ? server.entries.slice(at + 1) : server.entries;
  return { ...q, items: [...q.items.slice(0, q.index + 1), ...upcoming.map(entryItem)] };
}

const PlayerContext = createContext<(item: NowPlaying, queue?: NowPlaying[]) => void>(() => {});

/** Opens the full-screen player from anywhere in the app; with a queue, it carries on through it. */
export function usePlayer() {
  return useContext(PlayerContext);
}

// How often progress is saved while playing, and a converting session is kept alive.
const reportEveryMS = 10_000;
const pingEveryMS = 30_000;
const speeds = [0.5, 0.75, 1, 1.25, 1.5, 2];

export function PlayerProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation('player');
  const qc = useQueryClient();
  const [queue, setQueue] = useState<QueueState | null>(null);
  const [notice, setNotice] = useState('');
  const [recovered, setRecovered] = useState<PlayQueue | null>(null);
  // The queue is kept on the server too (for this device and profile) when it supports that,
  // so a reload can pick it up again; otherwise it lives in this page only.
  const profileId = useAuth().data?.profile?.id;
  const capabilities = useQuery({ queryKey: ['capabilities', profileId], queryFn: api.capabilities, enabled: !!profileId, staleTime: Infinity, retry: false });
  const playQueues = capabilities.data?.features.play_queues;
  const serverQueues = !!(playQueues?.available && playQueues.allowed);
  const serverQueuesRef = useRef(serverQueues);
  serverQueuesRef.current = serverQueues;
  const [mirror] = useState(() => new QueueMirror(
    (server) => setQueue((q) => withEntries(q, server)),
    (server) => {
      setQueue((q) => reconciled(q, server));
      setNotice(server ? i18n.t('player:player.queueReloaded') : i18n.t('player:player.queueExpired'));
    },
  ));
  // Lists the server should hold: every title one it can play, within its size limit.
  const track = useCallback((items: QueuedItem[]) => {
    const ok = serverQueuesRef.current && items.length > 0 && items.length <= maxQueueEntries && items.every(mirrorable);
    return { items: ok ? items.map((i) => ({ ...i, mirrored: true })) : items, ok };
  }, []);
  const [minimized, setMinimized] = useState(false);
  const [blockedNextFor, blockNext] = useState('');
  const playGeneration = useRef(0);
  const flushReport = useRef<(() => void | Promise<unknown>) | null>(null);
  const registerFlush = useCallback((fn: (() => void | Promise<unknown>) | null) => { flushReport.current = fn; }, []);
  const play = useCallback((item: NowPlaying, list?: NowPlaying[]) => {
    const generation = ++playGeneration.current;
    setMinimized(false); blockNext(''); setNotice(''); setRecovered(null);
    const tracked = track(list?.length ? list.map(queued) : [queued(item)]);
    const start = { items: tracked.items, index: list?.length ? Math.max(0, list.indexOf(item)) : 0 };
    let created: PlayQueue | null = null;
    if (tracked.ok) {
      mirror.start(start.items.map((i) => i.itemId!)).then((server) => {
        if (generation !== playGeneration.current) return;
        created = server;
        if (server) setQueue((q) => withEntries(q, server));
        else setQueue((q) => q && { ...q, items: q.items.map((i) => ({ ...i, mirrored: false })) }); // carries on in this page only
      });
    } else mirror.end();
    // A movie played from the start gets the pre-roll videos set in Settings first.
    if (!item.itemId || item.preroll || item.together || (item.startMS ?? 0) > 0) {
      setQueue(start);
      return;
    }
    api
      .preroll(item.itemId)
      .catch(() => [])
      .then((clips) => {
        if (generation !== playGeneration.current) return;
        const prerolls = clips.map((c) => queued({ title: c.title, subtitle: t('player:player.beforeTitle', { title: item.title }), src: c.url, preroll: true }));
        const state = { items: [...start.items.slice(0, start.index), ...prerolls, ...start.items.slice(start.index)], index: start.index };
        // The server may have answered first: its entries are given out now instead.
        setQueue(created ? withEntries(state, created) : state);
      });
  }, [mirror, track]);
  const go = useCallback((delta: number) => setQueue((q) => (q && q.items[q.index + delta] ? { ...q, index: q.index + delta } : q)), []);
  // Watch Together: the player plays the room's title, from where the room is.
  const together = useWatchTogether();
  const roomItem = together.room?.available ? together.room.item_id : undefined;
  const followed = useRef<string | undefined>(undefined);
  useEffect(() => {
    const session = together.session;
    if (!session || !roomItem) { followed.current = undefined; return; }
    const key = session.room.id + ':' + roomItem + ':' + session.room.generation;
    if (followed.current?.startsWith(session.room.id + ':' + roomItem + ':')) return; // already playing it
    followed.current = key;
    let cancelled = false;
    api.item(roomItem).then((it) => {
      if (cancelled) return;
      play({ ...nowPlayingFor(it, true), startMS: session.expected(), together: true });
    }).catch(() => undefined);
    return () => { cancelled = true; };
  }, [together.session, roomItem, play]);
  // Auto-Play: an episode with nothing queued after it carries on with the next one in the
  // profile's episode order (the countdown, Next and Shift+N then work as in a playlist).
  const autoPlay = usePrefs().data?.auto_play !== false;
  const current = queue?.items[queue.index];
  const hasNext = !!queue?.items[queue.index + 1];
  // The server's queue follows the title playing here (Advance after one ends, Select for a jump).
  useEffect(() => {
    if (current?.entryId) mirror.moveTo(current.entryId);
  }, [current?.entryId, mirror]);
  // After a reload, this device's queue can be picked up where it was.
  useEffect(() => {
    setRecovered(null);
    if (!serverQueues || mirror.active) return;
    let cancelled = false;
    api
      .currentPlayQueue()
      .then((q) => {
        const at = q.entries.findIndex((e) => e.id === q.current_entry_id);
        if (!cancelled && !mirror.active && at >= 0 && q.entries.slice(at).some((e) => e.available)) setRecovered(q);
      })
      .catch(() => undefined); // 404: nothing to pick up
    return () => {
      cancelled = true;
    };
  }, [serverQueues, profileId, mirror]);
  const resume = (q: PlayQueue) => {
    playGeneration.current++;
    setRecovered(null); setMinimized(false); blockNext(''); setNotice('');
    mirror.adopt(q);
    setQueue({ items: q.entries.map(entryItem), index: Math.max(0, q.entries.findIndex((e) => e.id === q.current_entry_id)) });
  };
  const discard = (q: PlayQueue) => {
    setRecovered(null);
    api.deletePlayQueue(q.id, q.revision).catch(() => undefined);
  };
  useEffect(() => {
    if (!current?.itemId || hasNext || !autoPlay || current.queueId === blockedNextFor) return;
    let cancelled = false;
    api
      .nextEpisode(current.itemId)
      .then((ep) => {
        if (cancelled || !ep?.stream_url) return;
        const added = { ...queued(nowPlayingFor(ep, true)), mirrored: mirror.active };
        setQueue((q) => (q && q.items[q.index] === current && !q.items[q.index + 1] ? { ...q, items: [...q.items, added] } : q));
        if (added.mirrored) mirror.edit({ operation: 'append', item_ids: [ep.id] });
      })
      .catch(() => undefined); // playing on is a nicety; the episode itself plays regardless
    return () => {
      cancelled = true;
    };
  }, [current, hasNext, autoPlay, blockedNextFor, mirror]);
  // Closing the player discards the queue; leaving the profile keeps it to pick up later.
  const leaveRoom = together.leave;
  const inRoom = !!together.session;
  const close = useCallback((keep = false) => {
    playGeneration.current++;
    // Closing the player leaves the Watch Together room (the others carry on).
    if (inRoom) void leaveRoom();
    setQueue(null);
    setNotice('');
    if (keep) mirror.release();
    else mirror.end();
    // Watched state and Continue Watching have changed.
    window.setTimeout(() => {
      for (const key of ['item', 'items', 'seasons', 'continue', 'next-up']) qc.invalidateQueries({ queryKey: [key] });
    }, 300);
  }, [qc, mirror, inRoom, leaveRoom]);
  // An entry the server can no longer play is skipped (it stays listed until it's removed).
  useEffect(() => {
    if (!current?.unavailable) return;
    if (hasNext) go(1);
    else close();
  }, [current, hasNext, go, close]);
  // In a room, Up Next is the host's: everyone else follows when the host moves on.
  const roomNext = (current: QueuedItem, nextItem: QueuedItem | undefined) => {
    if (!current.together || !together.session) return nextItem;
    return together.room?.host_id === together.room?.self_id ? nextItem : undefined;
  };
  const controls: QueueControls = {
    state: queue,
    notice,
    stop: async () => { await flushReport.current?.(); close(true); },
    enqueue: (items, next) => {
      if (!items.length) return;
      if (!queue) { play(items[0], items); return; }
      const added = items.map(queued);
      const tracked = mirror.active ? track(added) : { items: added, ok: false };
      setQueue(q => {
        if (!q) return q;
        const updated = [...q.items];
        updated.splice(next ? q.index + 1 : updated.length, 0, ...tracked.items);
        return { ...q, items: updated };
      });
      if (tracked.ok) mirror.edit({ operation: next ? 'next' : 'append', item_ids: tracked.items.map(i => i.itemId!) });
    },
    move: (id, delta) => {
      if (!queue) return;
      const from = queue.items.findIndex(i => i.queueId === id), to = from + delta;
      if (from <= queue.index || to <= queue.index || to >= queue.items.length) return;
      const items = [...queue.items]; [items[from], items[to]] = [items[to], items[from]];
      setQueue({ ...queue, items });
      mirror.reorder(items.flatMap(i => (i.entryId ? [i.entryId] : [])));
    },
    remove: id => {
      if (current) blockNext(current.queueId);
      const entry = queue?.items.find((i, at) => i.queueId === id && at > queue.index);
      if (!entry) return;
      setQueue(q => q && { ...q, items: q.items.filter(i => i.queueId !== id) });
      if (entry.entryId) mirror.edit({ operation: 'remove', entry_ids: [entry.entryId] });
    },
    clear: () => {
      if (current) blockNext(current.queueId);
      setQueue(q => q ? { ...q, items: q.items.slice(0, q.index + 1) } : q);
      mirror.edit({ operation: 'clear' });
    },
    jump: id => setQueue(q => { const index = q?.items.findIndex(i => i.queueId === id) ?? -1; return q && index >= 0 ? { ...q, index } : q; }),
  };
  return (
    <QueueContext.Provider value={controls}>
    <PlayerContext.Provider value={play}>
      {children}
      <AnimatePresence>
        {queue && !queue.items[queue.index].unavailable && (
          <PlayerOverlay
            key={queue.items[queue.index].queueId}
            item={queue.items[queue.index]}
            next={roomNext(queue.items[queue.index], queue.items[queue.index + 1])}
            onNext={() => {
              const nextItem = queue.items[queue.index + 1];
              // In a room the host moves everyone to the next title; the room then plays it here.
              if (queue.items[queue.index].together && together.session && nextItem?.itemId) void together.session.changeItem(nextItem.itemId).catch(() => undefined);
              else go(1);
            }}
            onPrevious={queue.index > 0 && !queue.items[queue.index - 1].preroll ? () => go(-1) : undefined}
            onClose={() => close()}
            registerFlush={registerFlush}
            minimized={minimized}
            onMinimize={() => setMinimized(m => !m)}
          />
        )}
      </AnimatePresence>
      {recovered && !queue && <RecoveredQueue queue={recovered} onResume={() => resume(recovered)} onDismiss={() => discard(recovered)} />}
    </PlayerContext.Provider>
    </QueueContext.Provider>
  );
}

function readVolume() {
  try {
    const v = Number(localStorage.getItem('arc-volume'));
    return Number.isFinite(v) && localStorage.getItem('arc-volume') !== null ? Math.min(1, Math.max(0, v)) : 1;
  } catch {
    return 1;
  }
}

function PlayerOverlay({
  item,
  next,
  onNext,
  onPrevious,
  onClose,
  minimized, onMinimize, registerFlush,
}: {
  item: NowPlaying;
  /** What plays after this one, in a queue. */
  next?: NowPlaying;
  onNext: () => void;
  onPrevious?: () => void;
  onClose: () => void;
  registerFlush: (fn: (() => void | Promise<unknown>) | null) => void;
  minimized: boolean;
  onMinimize: () => void;
}) {
  const { t } = useTranslation('player');
  const queue = usePlaybackQueue();
  // Watch Together: in a room, Play/Pause and seeking go to the room, and this player follows it.
  const together = useWatchTogether();
  const session = item.together ? together.session : null;
  const room = item.together ? together.room : null;
  const togetherNotice = useTogetherNotice(room);
  const overlayRoot = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (minimized) return;
    const previous = document.activeElement;
    overlayRoot.current?.focus();
    return () => { if (previous instanceof HTMLElement && previous.isConnected) previous.focus(); };
  }, [minimized]);
  const [queueOpen, setQueueOpen] = useState(false);
  const [retryRequest, requestRetry] = useState(0);
  const skipFinalReport = useRef(false);
  const video = useRef<HTMLVideoElement>(null);
  const hls = useRef<HlsType | null>(null);
  const plan = useRef<PlaybackPlan | null>(null);
  const [planState, setPlanState] = useState<PlaybackPlan | null>(null);
  // Where the video element's clock starts in the file (converted streams begin at the seek point).
  const offset = useRef(0);
  const pendingSeek = useRef<number | null>(null);
  const loadGeneration = useRef(0);
  const installedGeneration = useRef(0);
  const loadQueue = useRef(Promise.resolve());
  const requestedPosition = useRef<number | null>(null);
  // How many times this playback has fallen back to a heavier method after failing.
  const fallbacks = useRef(0);
  const watchdog = useRef<number>(undefined);
  // What the current stream was asked for with: seeks and audio changes keep it.
  const caps = useRef<PlayerCaps>(playerCaps());
  // A picture subtitle track being burned into the video; restarts keep it.
  const burn = useRef<string | undefined>(item.selection?.burnSubtitle);
  // The quality limit chosen on this device (0 for the original); restarts keep it.
  const qualityMode = useRef(readQuality());
  const quality = useRef(Math.max(0, qualityMode.current));
  const [qualityState, setQualityState] = useState(qualityMode.current);
  const [automaticLimit, setAutomaticLimit] = useState(0);
  // Away from home the server caps quality (its remote limits and the profile's internet streaming
  // quality): choices above it are left out and Maximum names it, unless the file is already within it.
  const serverCap = planState?.server_limit_kbps || 0;
  const qualityCap = serverCap && (!planState?.source_kbps || planState.source_kbps > serverCap) ? serverCap : 0;
  const shownQuality = qualityCap && qualityState >= qualityCap ? 0 : qualityState;
  const adaptive = useRef(new AdaptiveQuality(qualities));
  const originalRate = useRef(8000);
  const adaptiveBusy = useRef(false);
  const adaptiveLoading = useRef(false);
  const playIntent = useRef(true);
  const [qualityNotice, setQualityNotice] = useState('');
  const prefs = usePrefs();
  const updatePrefs = useUpdatePrefs();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Why the server's admin stopped playback, if they did.
  const [stopped, setStopped] = useState<string | null>(null);
  const [positionMS, setPositionMS] = useState(item.startMS ?? 0);
  const [durationMS, setDurationMS] = useState(0);
  // How far a converted stream is ready, in the file's time: seeking within it is instant.
  const [readyMS, setReadyMS] = useState(0);
  const [bufferedMS, setBufferedMS] = useState(0);
  const [paused, setPaused] = useState(false);
  const [volume, setVolume] = useState(readVolume);
  const [muted, setMuted] = useState(false);
  const [rate, setRate] = useState(1);
  const [fullscreen, setFullscreen] = useState(false);
  const [chromeVisible, setChromeVisible] = useState(true);
  const [menuOpen, setMenuOpen] = useState(false);
  const [subtitle, setSubtitle] = useState<SubtitleTrack | null>(null);
  const [cues, setCues] = useState<SubtitleCue[]>([]);
  // Timing belongs to this queue item's playback session and exact source/track.
  // Switching tracks remembers their offsets until the player closes; no source writes.
  const [subtitleOffsets, setSubtitleOffsets] = useState<Record<string, number>>({});
  const subtitleTimingKey = `${planState?.file_id}:${subtitle?.id}`;
  const subtitleOffsetMS = subtitle && !subtitle.image ? subtitleOffsets[subtitleTimingKey] ?? 0 : 0;
  const shiftSubtitles = (delta: number | null) => setSubtitleOffsets(previous => ({
    ...previous, [subtitleTimingKey]: delta === null ? 0 : Math.max(-10000, Math.min(10000, (previous[subtitleTimingKey] ?? 0) + delta)),
  }));
  const subtitleGeneration = useRef(0);
  const [subtitleError, setSubtitleError] = useState<string | null>(null);
  const subStyle = prefs.data?.subtitle_style ?? {};
  const subtitleSize: SubtitleSize = subStyle.size ?? readSubtitleSize();
  const setSubtitleSize = (size: SubtitleSize) => updatePrefs.mutate({ subtitle_style: { ...subStyle, size } });
  const [cueText, setCueText] = useState('');
  const autoChosen = useRef(false);
  const preplayApplied = useRef(false);
  const [trackNotice, setTrackNotice] = useState<string | null>(null);
  // Thumbnails for the timeline, if the library makes them.
  const [thumbs, setThumbs] = useState<{ info: TrickplayInfo; sheets: string[] } | null>(null);
  const lastSubtitle = useRef<SubtitleTrack | null>(null);
  // The panel for finding subtitles online.
  const [searching, setSearching] = useState(false);
  // Downloading a subtitle adds it for every profile (and uses the server's OpenSubtitles quota).
  const admin = useIsAdmin();
  // Seconds until the next item in the queue starts, once this one has ended.
  const [upNextIn, setUpNextIn] = useState<number | null>(null);
  const hideTimer = useRef<number>(undefined);
  const lastReport = useRef(0);
  // Casting to a Chromecast: where it started (the cast panel takes over until it stops).
  const castAvailable = useCastAvailable();
  const [castFrom, setCastFrom] = useState<{ startMS: number; fileId: number; audioStream: number; burnSubtitle?: string; text?: CastText } | null>(null);
  const castReturn = useRef<{ positionMS: number; audioStream: number } | null>(null);
  const castOn = useRef(false);
  castOn.current = castFrom !== null;
  // AirPlay (Safari): the video element plays a cast link the Apple TV can open itself.
  const airplay = useRef(false);
  const [airplayAvailable, setAirplayAvailable] = useState(false);

  const absMS = useCallback(() => requestedPosition.current ?? offset.current + (video.current?.currentTime ?? 0) * 1000, []);

  const report = useCallback(
    (state: 'playing' | 'paused' | 'stopped' = 'playing') => {
      const p = plan.current;
      if (!item.itemId || !p || !video.current) return;
      const dur = p.duration_ms || (video.current.duration || 0) * 1000;
      if (!dur) return;
      lastReport.current = Date.now();
      return api.reportProgress(item.itemId, absMS(), dur, state).catch((e) => {
        if (e instanceof ApiError && e.needs === 'stopped') setStopped(e.message);
      });
    },
    [item.itemId, absMS],
  );

  const detach = useCallback(() => {
    hls.current?.destroy();
    hls.current = null;
  }, []);

  // Start (or restart) playback at a point in the file, optionally with another audio track.
  const load = useCallback(
    async (startMS: number, audioStream?: number, withCaps: PlayerCaps = caps.current, automatic = false) => {
      const generation = ++loadGeneration.current;
      adaptiveLoading.current = automatic;
      adaptive.current.reset(Date.now());
      requestedPosition.current = startMS;
      installedGeneration.current = 0;
      setPositionMS(startMS);
      setLoading(true);
      window.clearTimeout(watchdog.current);
      // Serialize admission so superseded conversions release their slot before
      // the newest seek starts. Queued intermediate seeks need no server request.
      const previous = loadQueue.current;
      let release!: () => void;
      loadQueue.current = new Promise<void>(resolve => { release = resolve; });
      await previous;
      try {
        if (generation !== loadGeneration.current || !video.current) return;
        caps.current = withCaps;
        const v = video.current;
        if (!v) return;
        v.autoplay = !automatic || playIntent.current;
        const startVideo = () => { if (!automatic || playIntent.current) v.play().catch(() => {}); };
        setError(null);
        detach();
        const oldSession = plan.current?.session_id;
        if (oldSession) await api.stopTranscode(oldSession).catch(() => {});
        if (generation !== loadGeneration.current) return;
        if (!item.itemId) {
          // An extra: play the file directly.
          offset.current = 0;
          plan.current = null;
          installedGeneration.current = generation;
          pendingSeek.current = startMS / 1000;
          v.src = item.src ?? '';
          v.play().catch(() => {});
          return;
        }
        let p: PlaybackPlan;
        try {
          p = await api.startPlayback(item.itemId, { caps: withCaps, start_ms: Math.round(startMS), audio_stream: audioStream ?? (plan.current ? undefined : item.selection?.audioStream), file_id: plan.current?.file_id ?? item.selection?.fileId, burn_subtitle: burn.current, edition: item.edition, max_kbps: quality.current || undefined });
        } catch (e) {
          if (generation !== loadGeneration.current) return;
          setLoading(false);
          if (e instanceof ApiError && e.status === 403) setStopped(e.message);
          else setError(e instanceof Error ? e.message : String(e));
          return;
        }
        if (generation !== loadGeneration.current) {
          if (p.session_id) await api.stopTranscode(p.session_id).catch(() => {});
          return;
        }
        plan.current = p;
        if (qualityMode.current === -1) {
          const effective = quality.current ? Math.min(quality.current, p.server_limit_kbps || Infinity) : p.server_limit_kbps;
          setQualityNotice(effective ? t('player:player.automaticLimit', { quality: qualityName(effective) }) : t('player:player.originalAllowed'));
        }
        if (!quality.current) originalRate.current = p.optimized?.bitrate ? p.optimized.bitrate / 1000 : p.source_kbps || 8000;
        setPlanState(p);
        setDurationMS(p.duration_ms);
        offset.current = p.hls ? p.offset_ms : 0;
        // A remux can start at an earlier keyframe. Begin at the requested point
        // within that output, rather than replaying from the measured keyframe.
        pendingSeek.current = p.hls ? Math.max(0, (startMS - p.offset_ms) / 1000) : startMS / 1000;
        setReadyMS(offset.current);
        if (airplay.current) {
          // The Apple TV fetches the stream itself (HLS natively), without the sign-in cookie.
          let source = p.url;
          try {
            source = (await api.castLink(p.url)).url;
          } catch { /* Fall back to the authenticated stream. */ }
          if (generation !== loadGeneration.current) return;
          installedGeneration.current = generation;
          v.src = source;
          startVideo();
          return;
        }
        if (!p.hls) {
          installedGeneration.current = generation;
          v.src = p.url;
        } else {
          const { default: Hls } = await import('hls.js');
          if (generation !== loadGeneration.current) return;
          installedGeneration.current = generation;
          if (!Hls.isSupported()) {
            // No Media Source Extensions (iPhone): the browser plays HLS itself.
            v.src = p.url;
            startVideo();
            return;
          }
          const h = new Hls({ startPosition: Math.max(0, (startMS - p.offset_ms) / 1000), maxBufferLength: 60, backBufferLength: 120 });
          h.on(Hls.Events.FRAG_LOADED, (_e, data) => {
            if (generation !== loadGeneration.current || data.frag.type !== 'main') return;
            const { stats } = data.frag;
            adaptive.current.observe(Date.now(), stats.loaded, stats.loading.end - stats.loading.start, data.frag.duration);
            if (!quality.current && adaptive.current.mediaKbps) originalRate.current = adaptive.current.mediaKbps;
          });
          h.on(Hls.Events.ERROR, (_e, data) => {
            if (data.fatal) void onStreamError(generation);
          });
          h.loadSource(p.url);
          h.attachMedia(v);
          hls.current = h;
        }
        startVideo();
        // Some browsers accept a format and then never show a frame: give up on it after a while.
        window.clearTimeout(watchdog.current);
        watchdog.current = window.setTimeout(() => {
          if (video.current && video.current.readyState < 2) void onStreamError(generation);
        }, qualityMode.current === -1 ? 25_000 : 15_000);
      } finally {
        release();
      }
    },
    // onStreamError is stable through refs; listing it would recreate load on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [item.itemId, item.src, detach],
  );

  const adaptiveState = useRef({ error, stopped });
  adaptiveState.current = { error, stopped };
  useEffect(() => {
    const timer = window.setInterval(() => {
      const v = video.current, p = plan.current;
      if (!v || !p || qualityMode.current !== -1) return;
      let buffer = 0;
      for (let i = 0; i < v.buffered.length; i++) {
        if (v.buffered.start(i) <= v.currentTime + 0.1 && v.buffered.end(i) >= v.currentTime) buffer = v.buffered.end(i) - v.currentTime;
      }
      const active = !v.paused && !v.ended && !v.seeking && !document.hidden && !castOn.current && !airplay.current &&
        !adaptiveBusy.current && installedGeneration.current === loadGeneration.current &&
        !adaptiveState.current.error && adaptiveState.current.stopped === null && !!item.itemId && !item.preroll;
      const target = adaptive.current.decide({ now: Date.now(), active, bufferSeconds: buffer, waiting: v.readyState < 3,
        remainingSeconds: (p.duration_ms - absMS()) / 1000, playbackRate: v.playbackRate,
        currentLimit: quality.current, originalKbps: originalRate.current, ceilingKbps: p.server_limit_kbps || 0 });
      if (target === null) return;
      quality.current = target; setAutomaticLimit(target);
      setQualityNotice(target ? t('player:player.automaticLimit', { quality: qualityName(target) }) : t('player:player.originalAllowed'));
      adaptiveBusy.current = true;
      void load(absMS(), p.audio_stream, caps.current, true).finally(() => { adaptiveBusy.current = false; });
    }, 1000);
    return () => window.clearInterval(timer);
  }, [item.itemId, item.preroll, absMS, load]);

  // Something went wrong mid-stream: the admin stopped it, the session expired, or the browser
  // couldn't decode what it said it could.
  const onStreamError = useCallback(async (generation = loadGeneration.current) => {
    if (generation !== loadGeneration.current) return;
    const p = plan.current;
    const at = absMS();
    const url = p?.url ?? item.src;
    if (url) {
      try {
        const res = await fetch(url, { headers: { Range: 'bytes=0-0' } });
        if (generation !== loadGeneration.current) return;
        if (res.status === 403) {
          const body = await res.json().catch(() => ({}));
          detach();
          setStopped(body.error ?? t('player:player.adminStopped'));
          return;
        }
      } catch {
        // fall through
      }
    }
    if (generation !== loadGeneration.current) return;
    if (p && p.method !== 'transcode' && fallbacks.current < 2) {
      // The browser couldn't play what it said it could: try the next heavier method.
      fallbacks.current++;
      await load(at, p.audio_stream, fallbackCaps(p.method));
      return;
    }
    if (p?.hls && fallbacks.current < 3) {
      // A conversion that ended (e.g. after a long pause): start it again where it was.
      fallbacks.current++;
      await load(at, p.audio_stream);
      return;
    }
    detach();
    setLoading(false);
    // A pre-roll that won't play is skipped rather than holding up the movie.
    if (item.preroll && next) return onNext();
    setError(item.itemId ? t('player:player.fileError') : t('player:player.browserError'));
  }, [absMS, detach, item.itemId, item.src, item.preroll, next, onNext, load]);

  // Show a subtitle track (null for none). Text ones are drawn here; picture ones are burned into
  // the video, which restarts it converted. remember saves the language for next time.
  const selectSubtitle = useCallback(
    async (tabId: SubtitleTrack | null, remember: boolean) => {
      const generation = ++subtitleGeneration.current;
      if (remember) setTrackNotice(null);
      const p = plan.current;
      if (!p || !item.itemId) return;
      // Turning them off forgets the language only where the language is what turns them on.
      const remembered = rememberedSubtitleLanguage(tabId, prefs.data?.subtitle_mode);
      if (remember && remembered !== undefined) updatePrefs.mutate({ subtitle_language: remembered });
      if (tabId) lastSubtitle.current = tabId;
      setSubtitle(tabId);
      setCues([]);
      setSubtitleError(null);
      const wantBurn = tabId?.image ? tabId.id : undefined;
      if (wantBurn !== burn.current) {
        burn.current = wantBurn;
        await load(absMS(), p.audio_stream);
      }
      if (!tabId || tabId.image || generation !== subtitleGeneration.current) return;
      try {
        const res = await api.subtitleCues(item.itemId, tabId.id, p.file_id);
        if (generation === subtitleGeneration.current) setCues(res.cues);
      } catch (e) {
        if (generation === subtitleGeneration.current) setSubtitleError(e instanceof Error ? e.message : String(e));
      }
    },
    // updatePrefs.mutate is stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [item.itemId, load, absMS],
  );

  // Switch to another of the title's files (an edition or version) where playback is now. Its
  // audio and subtitle tracks are different, so they're chosen again.
  const selectVersion = useCallback(
    async (fileId: number) => {
      const p = plan.current;
      if (!p || p.file_id === fileId) return;
      const at = absMS();
      adaptive.current = new AdaptiveQuality(qualities);
      if (qualityMode.current === -1) { quality.current = 0; setAutomaticLimit(0); }
      subtitleGeneration.current++;
      plan.current = { ...p, file_id: fileId };
      burn.current = undefined;
      setSubtitle(null);
      setCues([]);
      autoChosen.current = false;
      setTrackNotice(null);
      await load(at);
    },
    [absMS, load],
  );

  // The playing file's timeline thumbnails (none if the library doesn't make them, or not yet).
  const thumbFile = planState?.file_id;
  useEffect(() => {
    setThumbs(null);
    if (!item.itemId || !thumbFile) return;
    let cancelled = false;
    api
      .trickplay(item.itemId, thumbFile)
      .then((tabId) => !cancelled && setThumbs(tabId))
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [item.itemId, thumbFile]);

  // Turn subtitles on by themselves once: in the profile's language, or forced ones.
  useEffect(() => {
    if (autoChosen.current || !planState || !prefs.data) return;
    autoChosen.current = true;
    const audioLang = planState.audio_tracks.find((a) => a.index === planState.audio_stream)?.language;
    const selection = !preplayApplied.current && item.selection?.fileId === planState.file_id ? item.selection : undefined;
    preplayApplied.current = true;
    let localValue = chooseSubtitle(planState.subtitle_tracks, prefs.data.subtitle_language, audioLang, prefs.data.subtitle_mode);
    const patch: { audio_language?: string; subtitle_language?: string } = {};
    if (selection?.audioStream !== undefined && selection.audioStream === planState.audio_stream && audioLang && audioLang !== prefs.data.audio_language) patch.audio_language = audioLang;
    if (selection?.subtitle !== undefined) {
      const explicit = selection.subtitle === null ? null : planState.subtitle_tracks.find(track => track.id === selection.subtitle);
      if (explicit !== undefined) {
        localValue = explicit ?? undefined;
        const remembered = rememberedSubtitleLanguage(explicit, prefs.data.subtitle_mode);
        if (remembered !== undefined) patch.subtitle_language = remembered;
      } else {
        setTrackNotice(t('player:player.missingSubtitles'));
      }
    }
    if (Object.keys(patch).length) updatePrefs.mutate(patch);
    if (localValue) void selectSubtitle(localValue, false);
  }, [planState, prefs.data, selectSubtitle]);

  // The cue on screen, checked every frame so it appears on time.
  useEffect(() => {
    if (!cues.length) {
      setCueText('');
      return;
    }
    let raf = 0;
    const tick = () => {
      const localValue = absMS() - subtitleOffsetMS;
      const text = cues
        .filter((c) => c.start_ms <= localValue && localValue < c.end_ms)
        .map((c) => c.text)
        .join('\n');
      setCueText((prev) => (prev === text ? prev : text));
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [cues, absMS, subtitleOffsetMS]);

  // Save while the video ref still exists. Passive effect cleanup runs after React detaches
  // the element, losing the final position if no periodic report has been sent yet.
  useLayoutEffect(() => () => { if (!skipFinalReport.current) void report('stopped'); }, [report]);
  useEffect(() => {
    registerFlush(async () => { video.current?.pause(); await report('stopped'); skipFinalReport.current = true; });
    return () => registerFlush(null);
  }, [registerFlush, report]);
  useEffect(() => { if (retryRequest) { fallbacks.current = 0; void load(absMS(), plan.current?.audio_stream); } }, [retryRequest, load, absMS]);

  // Start playing when the player opens; end the conversion when it closes.
  useEffect(() => {
    void load(item.startMS ?? 0, item.selection?.audioStream);
    const onHide = () => report('stopped');
    window.addEventListener('pagehide', onHide);
    return () => {
      loadGeneration.current++;
      subtitleGeneration.current++;
      window.removeEventListener('pagehide', onHide);
      detach();
      window.clearTimeout(watchdog.current);
      const sid = plan.current?.session_id;
      if (sid) api.stopTranscode(sid).catch(() => {});
    };
    // Runs once per opened item.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // AirPlay: offered when Safari finds an Apple TV; the stream switches to a cast link when one is
  // chosen, and back when it's left.
  useEffect(() => {
    const v = video.current as (HTMLVideoElement & { webkitCurrentPlaybackTargetIsWireless?: boolean }) | null;
    if (!v || !('WebKitPlaybackTargetAvailabilityEvent' in window)) return;
    const onAvailability = (e: Event) => setAirplayAvailable((e as Event & { availability: string }).availability === 'available');
    const onTarget = () => {
      const wireless = !!v.webkitCurrentPlaybackTargetIsWireless;
      if (wireless === airplay.current || !item.itemId) return;
      airplay.current = wireless;
      void load(absMS(), plan.current?.audio_stream);
    };
    v.addEventListener('webkitplaybacktargetavailabilitychanged', onAvailability);
    v.addEventListener('webkitcurrentplaybacktargetiswirelesschanged', onTarget);
    return () => {
      v.removeEventListener('webkitplaybacktargetavailabilitychanged', onAvailability);
      v.removeEventListener('webkitcurrentplaybacktargetiswirelesschanged', onTarget);
    };
  }, [item.itemId, load, absMS]);

  const startCast = useCallback(async () => {
    const at = absMS();
    if (!(await chooseChromecast())) return;
    const source = plan.current;
    if (!source) return;
    ++loadGeneration.current;
    const v = video.current;
    v?.pause();
    detach();
    v?.removeAttribute('src');
    v?.load();
    if (source.session_id) await api.stopTranscode(source.session_id).catch(() => {});
    // A text subtitle goes with it, shown by the device itself with the same timing adjustment.
    const text: CastText | undefined = subtitle && !subtitle.image
      ? { id: subtitle.id, language: subtitle.language, name: subtitleLabel(subtitle, 0).name, shiftMS: subtitleOffsetMS }
      : undefined;
    setCastFrom({ startMS: at, fileId: source.file_id, audioStream: source.audio_stream, burnSubtitle: burn.current, text });
  }, [absMS, detach, subtitle, subtitleOffsetMS]);

  // Stop/disconnect can finish asynchronously. Resume only after the browser's
  // video element has remounted, rather than racing React's batched state update.
  useEffect(() => {
    if (castFrom !== null || !castReturn.current) return;
    const target = castReturn.current;
    castReturn.current = null;
    if (video.current) void load(target.positionMS, target.audioStream);
  }, [castFrom, load]);

  // Keep a converting session alive while paused.
  useEffect(() => {
    const sid = planState?.session_id;
    if (!sid) return;
    const localValue = window.setInterval(() => {
      api.pingTranscode(sid).catch(() => {});
    }, pingEveryMS);
    return () => window.clearInterval(localValue);
  }, [planState?.session_id]);

  const seekLocal = useCallback(
    (target: number) => {
      const v = video.current;
      const p = plan.current;
      if (!v) return;
      adaptive.current.reset(Date.now());
      const dur = durationMS || (v.duration || 0) * 1000;
      const localValue = Math.max(0, dur ? Math.min(target, dur - 500) : target);
      setPositionMS(localValue);
      if (!p?.hls) {
        v.currentTime = localValue / 1000;
        return;
      }
      if (requestedPosition.current !== null) {
        void load(localValue, p.audio_stream);
        return;
      }
      const rel = (localValue - offset.current) / 1000;
      const end = v.seekable.length ? v.seekable.end(v.seekable.length - 1) : 0;
      if (rel >= 0 && rel <= end - 1) {
        v.currentTime = rel;
      } else {
        // Not converted yet (or before where this conversion started): start again from there,
        // the same way as now.
        void load(localValue, p.audio_stream);
      }
    },
    [durationMS, load],
  );
  // In a room a seek moves everyone: this player goes there at once (so repeated presses add up),
  // and the room brings the others; if the room refuses, following it brings this one back.
  const seekTo = useCallback(
    (target: number) => {
      if (!session) return seekLocal(target);
      const dur = durationMS || (video.current?.duration || 0) * 1000;
      const to = Math.max(0, dur ? Math.min(target, dur - 500) : target);
      seekLocal(to);
      void session.seek(to).catch(() => undefined);
    },
    [session, seekLocal, durationMS],
  );

  const toggleLocal = useCallback(() => {
    const v = video.current;
    if (!v) return;
    if (adaptiveLoading.current && requestedPosition.current !== null) {
      playIntent.current = !playIntent.current;
      setPaused(!playIntent.current);
      v.autoplay = playIntent.current;
      if (!playIntent.current) v.pause();
      else if (installedGeneration.current === loadGeneration.current) v.play().catch(() => {});
      return;
    }
    playIntent.current = v.paused;
    if (v.paused) v.play().catch(() => {});
    else v.pause();
  }, []);
  const togglePlay = useCallback(() => {
    if (!session || !room) return toggleLocal();
    void (room.state === 'paused' ? session.play() : session.pause()).catch(() => undefined);
  }, [session, room, toggleLocal]);

  // Follow the room: play or pause with it, and stay within half a second of where it should be
  // (a small speed change for small drift, a jump for large) (docs/WATCH-TOGETHER-API.md).
  const roomRef = useRef(room);
  roomRef.current = room;
  useEffect(() => {
    if (!session) return;
    const follow = () => {
      const v = video.current;
      const r = roomRef.current;
      if (!v || !r || !r.available || loading || castOn.current) return;
      const expected = session.expected();
      const local = absMS();
      if (r.state === 'playing') {
        if (v.paused) { playIntent.current = true; v.play().catch(() => {}); }
        const c = correction(local, expected);
        if (c.kind === 'seek') { v.playbackRate = 1; seekLocal(c.to); }
        else v.playbackRate = c.kind === 'rate' ? c.rate : 1;
      } else {
        v.playbackRate = 1;
        if (!v.paused) { playIntent.current = false; v.pause(); }
        if (Math.abs(local - expected) > 1000) seekLocal(expected);
      }
    };
    follow();
    const timer = window.setInterval(follow, 500);
    return () => window.clearInterval(timer);
  }, [session, room?.revision, room?.generation, loading, absMS, seekLocal]);

  // What this player tells the room: where it is, and whether it's ready or still loading.
  useEffect(() => {
    if (!session) return;
    session.setReporter(() => {
      const v = video.current;
      if (!v) return null;
      const buffering = loading || v.seeking || v.readyState < 3;
      return { positionMS: absMS(), state: buffering ? 'waiting' : v.paused ? 'paused' : 'playing', ready: !buffering };
    });
    return () => session.setReporter(undefined);
  }, [session, loading, absMS]);
  useEffect(() => {
    session?.reportNow();
  }, [session, loading]);

  // Picture-in-Picture: the video in a small floating window.
  const [pip, setPip] = useState(false);
  const pipSupported = typeof document !== 'undefined' && document.pictureInPictureEnabled;
  const togglePip = useCallback(() => {
    const v = video.current;
    if (!v) return;
    if (document.pictureInPictureElement) document.exitPictureInPicture().catch(() => {});
    else v.requestPictureInPicture().catch(() => {});
  }, []);
  useEffect(() => {
    const v = video.current;
    if (!v) return;
    const enter = () => setPip(true);
    const leave = () => setPip(false);
    v.addEventListener('enterpictureinpicture', enter);
    v.addEventListener('leavepictureinpicture', leave);
    return () => {
      v.removeEventListener('enterpictureinpicture', enter);
      v.removeEventListener('leavepictureinpicture', leave);
      // Closing the player closes its floating window too.
      if (document.pictureInPictureElement === v) document.exitPictureInPicture().catch(() => {});
    };
  }, []);
  // The player draws subtitles over the page, which the floating window doesn't show, so while
  // it's open they're given to the browser as a text track (in the video's own clock).
  useEffect(() => {
    const v = video.current;
    if (!v || !pip || !cues.length) return;
    const track = v.addTextTrack('subtitles', t('player:player.nativeSubtitleLabel'));
    track.mode = 'showing';
    const offsetS = offset.current / 1000;
    for (const c of cues) {
      const start = (c.start_ms + subtitleOffsetMS) / 1000 - offsetS;
      const end = (c.end_ms + subtitleOffsetMS) / 1000 - offsetS;
      if (end > 0) track.addCue(new VTTCue(Math.max(0, start), end, c.text));
    }
    return () => {
      for (const c of Array.from(track.cues ?? [])) track.removeCue(c);
      track.mode = 'disabled';
    };
  }, [pip, cues, planState, subtitleOffsetMS]);

  const toggleFullscreen = useCallback(() => {
    // The whole page, so menus (rendered outside the player) stay visible.
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
    else document.documentElement.requestFullscreen().catch(() => {});
  }, []);

  useEffect(() => {
    const onChange = () => setFullscreen(!!document.fullscreenElement);
    document.addEventListener('fullscreenchange', onChange);
    return () => {
      document.removeEventListener('fullscreenchange', onChange);
      if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
    };
  }, []);

  useEffect(() => {
    const v = video.current;
    if (!v) return;
    v.volume = volume;
    v.muted = muted;
    try {
      localStorage.setItem('arc-volume', String(volume));
    } catch {
      // not remembered
    }
  }, [volume, muted]);

  useEffect(() => {
    if (video.current) video.current.playbackRate = rate;
  }, [rate]);

  const poke = useCallback(() => {
    setChromeVisible(true);
    window.clearTimeout(hideTimer.current);
    hideTimer.current = window.setTimeout(() => {
      if (!video.current?.paused) setChromeVisible(false);
    }, 3000);
  }, []);

  // Keyboard shortcuts.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (queueOpen || e.defaultPrevented || (e.target instanceof Element && e.target.closest('[role=dialog], [role=menu]'))) return;
      if (minimized && !(e.target instanceof Node && overlayRoot.current?.contains(e.target))) return;
      if (!minimized && !menuOpen && e.key === 'Tab') {
        const buttons = Array.from(overlayRoot.current?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),[tabindex="0"]') ?? []).filter(el => el.offsetParent !== null);
        const first = buttons[0], last = buttons[buttons.length - 1];
        if (first && (!overlayRoot.current?.contains(document.activeElement) || (!e.shiftKey && document.activeElement === last) || (e.shiftKey && document.activeElement === first))) { e.preventDefault(); (e.shiftKey ? last : first).focus(); }
        return;
      }
      if (e.target instanceof Element && e.target.closest('button,a') && (e.key === ' ' || e.key === 'Enter')) return;
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLSelectElement || e.ctrlKey || e.metaKey || e.altKey) return;
      if (castOn.current && e.key !== 'Escape') return; // the cast panel has its own controls
      const v = video.current;
      switch (e.key) {
        case 'Escape':
          if (searching) setSearching(false);
          else if (!document.fullscreenElement && !menuOpen) onClose();
          return;
        case ' ':
        case 'k':
          togglePlay();
          break;
        case 'ArrowLeft':
        case 'j': {
          const ch = plan.current?.chapters ?? [];
          if (e.shiftKey && ch.length) {
            // Back to the start of this chapter, or the one before if just past its start.
            const i = chapterAt(ch, absMS());
            seekTo(ch[absMS() - ch[i].start_ms < 3000 ? Math.max(0, i - 1) : i].start_ms);
          } else seekTo(absMS() - (e.key === 'j' ? 10_000 : 5_000));
          break;
        }
        case 'ArrowRight':
        case 'l': {
          const ch = plan.current?.chapters ?? [];
          if (e.shiftKey && ch.length) {
            const nextChapter = ch[chapterAt(ch, absMS()) + 1];
            if (nextChapter) seekTo(nextChapter.start_ms);
          } else seekTo(absMS() + (e.key === 'l' ? 10_000 : 5_000));
          break;
        }
        case 'ArrowUp':
          setVolume((x) => Math.min(1, Math.round((x + 0.1) * 10) / 10));
          setMuted(false);
          break;
        case 'ArrowDown':
          setVolume((x) => Math.max(0, Math.round((x - 0.1) * 10) / 10));
          break;
        case 'c':
          // Subtitles on (the last ones, or the first text ones) and off.
          if (subtitle) void selectSubtitle(null, true);
          else {
            const localValue = lastSubtitle.current ?? plan.current?.subtitle_tracks.find((x) => !x.image);
            if (localValue) void selectSubtitle(localValue, true);
          }
          break;
        case 'N':
          if (next) onNext();
          break;
        case 'P':
          onPrevious?.();
          break;
        case 'm':
          setMuted((x) => !x);
          break;
        case 'f':
          toggleFullscreen();
          break;
        case 'i':
          if (pipSupported) togglePip();
          break;
        case '>':
          setRate((r) => speeds[Math.min(speeds.length - 1, speeds.indexOf(r) + 1)] ?? r);
          break;
        case '<':
          setRate((r) => speeds[Math.max(0, speeds.indexOf(r) - 1)] ?? r);
          break;
        default:
          if (/^[0-9]$/.test(e.key) && v) {
            const dur = durationMS || (v.duration || 0) * 1000;
            if (dur) seekTo((dur * Number(e.key)) / 10);
            break;
          }
          return;
      }
      e.preventDefault();
      poke();
    };
    window.addEventListener('keydown', onKey);
    document.body.style.overflow = minimized ? '' : 'hidden';
    poke();
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
      window.clearTimeout(hideTimer.current);
    };
  }, [onClose, poke, togglePlay, seekTo, toggleFullscreen, absMS, durationMS, menuOpen, subtitle, selectSubtitle, searching, next, onNext, onPrevious, minimized, queueOpen]);

  const onTimeUpdate = () => {
    const v = video.current;
    if (!v || installedGeneration.current !== loadGeneration.current) return;
    setPositionMS(absMS());
    if (v.buffered.length) setBufferedMS(offset.current + v.buffered.end(v.buffered.length - 1) * 1000);
    if (plan.current?.hls && v.seekable.length) setReadyMS(offset.current + v.seekable.end(v.seekable.length - 1) * 1000);
    if (!v.paused && Date.now() - lastReport.current >= reportEveryMS) report();
  };

  // Skip Intro and Skip Credits.
  const markers = planState?.markers ?? [];
  const skippable = item.preroll ? undefined : markers.find((m) => m.kind !== 'credits' && positionMS >= m.start_ms && positionMS < m.end_ms - 1500);
  const credits = item.preroll ? undefined : markers.find((m) => m.kind === 'credits');
  const inCredits = !!credits && positionMS >= credits.start_ms && positionMS < credits.end_ms - 1500;
  const autoSkipped = useRef(new Set<string>());
  const creditsShown = useRef(false);
  const [skippedNote, setSkippedNote] = useState<string | null>(null);
  useEffect(() => {
    // "Skip intros automatically": once per intro or recap, so seeking back into one lets it play.
    if (!skippable || !prefs.data?.auto_skip || autoSkipped.current.has(skippable.kind) || paused) return;
    autoSkipped.current.add(skippable.kind);
    seekTo(skippable.end_ms);
    setSkippedNote(skippable.kind === 'intro' ? t('player:player.skippedIntro') : t('player:player.skippedRecap'));
    const timer = window.setTimeout(() => setSkippedNote(null), 2500);
    return () => window.clearTimeout(timer);
  }, [skippable, prefs.data?.auto_skip, paused, seekTo]);
  useEffect(() => {
    // When the credits start, Up Next counts down (as Plex does); Cancel lets them play.
    if (inCredits && next && !creditsShown.current && upNextIn === null) {
      creditsShown.current = true;
      setUpNextIn(10);
    }
  }, [inCredits, next, upNextIn]);

  const showChrome = minimized || chromeVisible || paused || menuOpen || !!error || stopped !== null;
  const dur = durationMS;

  return (
    <motion.div
      ref={overlayRoot}
      role="region" aria-label={t("player:player.mediaPlayer")} tabIndex={-1}
      className={cn(minimized ? 'fixed bottom-4 right-4 z-[100] flex h-48 w-80 max-w-[calc(100vw-32px)] items-center justify-center overflow-hidden rounded-2xl bg-black shadow-2xl' : 'fixed inset-0 z-[100] flex items-center justify-center bg-black', !showChrome && 'cursor-none')}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.25 }}
      onMouseMove={poke}
    >
      {trackNotice && <p role="status" className="absolute left-4 right-4 top-20 z-20 rounded-xl bg-black/80 p-3 text-sm text-white">{trackNotice}</p>}
      {room?.state === 'waiting' && session && !minimized && <TogetherWaiting room={room} serverNow={() => session.serverNow()} />}
      {room && !room.available && !minimized && (
        <p role="status" className="absolute inset-x-4 top-24 z-20 mx-auto max-w-md rounded-xl bg-black/80 p-3 text-center text-sm text-white">{t('player:together.unavailable')}</p>
      )}
      {togetherNotice && !minimized && (
        <p role="status" className="pointer-events-none absolute inset-x-0 top-24 z-20 mx-auto w-fit rounded-full bg-black/75 px-4 py-2 text-sm text-white ring-1 ring-white/15">{togetherNotice}</p>
      )}
      {item.together && !together.session && together.ended && !minimized && (
        <p role="status" className="pointer-events-none absolute inset-x-0 top-24 z-20 mx-auto w-fit rounded-full bg-black/75 px-4 py-2 text-sm text-white ring-1 ring-white/15">
          {together.ended === 'ended' ? t('player:together.endedByHost') : t('player:together.lost')}
        </p>
      )}
      {castFrom !== null && item.itemId ? (
        <CastSession
          itemId={item.itemId}
          title={item.title}
          subtitle={item.subtitle}
          startMS={castFrom.startMS}
          fileId={castFrom.fileId}
          audioStream={castFrom.audioStream}
          burnSubtitle={castFrom.burnSubtitle}
          text={castFrom.text}
          onEnd={(ms) => {
            castReturn.current = { positionMS: ms, audioStream: castFrom.audioStream };
            setCastFrom(null);
          }}
        />
      ) : stopped !== null ? (
        <Message icon={<OctagonX className="text-white/70" />} title={t("player:player.playbackWasStopped")} onClose={onClose}>
          {stopped}
        </Message>
      ) : error ? (
        <Message icon={<AlertCircle className="text-warning" />} title={t("player:player.canTPlayThis")} onClose={onClose} onRetry={() => { setError(null); requestRetry(n => n + 1); }}>
          {error}
        </Message>
      ) : (
        <>
          <video
            ref={video}
            autoPlay
            playsInline
            className="h-full w-full object-contain"
            onClick={togglePlay}
            onDoubleClick={toggleFullscreen}
            onLoadedMetadata={(e) => {
              if (installedGeneration.current !== loadGeneration.current) return;
              const v = e.currentTarget;
              if (!plan.current) setDurationMS((v.duration || 0) * 1000);
              if (pendingSeek.current) v.currentTime = pendingSeek.current;
              pendingSeek.current = null;
              if (adaptiveLoading.current && !playIntent.current) v.pause();
              v.playbackRate = rate;
            }}
            onError={() => {
              if (installedGeneration.current === loadGeneration.current) void onStreamError();
            }}
            onTimeUpdate={onTimeUpdate}
            onSeeking={() => adaptive.current.reset(Date.now())}
            onWaiting={() => setLoading(true)}
            onPlaying={() => {
              if (installedGeneration.current !== loadGeneration.current) return;
              if (adaptiveLoading.current && !playIntent.current) { video.current?.pause(); setPaused(true); return; }
              requestedPosition.current = null;
              adaptiveLoading.current = false;
              setLoading(false);
              setPaused(false);
              report('playing');
            }}
            onCanPlay={() => {
              if (installedGeneration.current === loadGeneration.current) setLoading(false);
            }}
            onPlay={() => { if (!adaptiveLoading.current) playIntent.current = true; setPaused(false); }}
            onPause={() => {
              if (installedGeneration.current !== loadGeneration.current) return;
              if (requestedPosition.current === null) playIntent.current = false;
              setPaused(true);
              report('paused');
            }}
            onEnded={() => {
              if (installedGeneration.current !== loadGeneration.current) return;
              setPaused(true);
              report('paused');
              // A pre-roll goes straight on to the movie; anything else counts down first.
              if (next && item.preroll) onNext();
              else if (next) setUpNextIn(10);
            }}
          />
          {cueText && (
            <div
              className={cn(
                'pointer-events-none absolute inset-x-0 flex flex-col items-center gap-1 px-[8%] text-center transition-[bottom,top] duration-300',
                subStyle.position === 'top' ? (showChrome ? 'top-28' : 'top-[7%]') : showChrome ? 'bottom-36' : 'bottom-[7%]',
              )}
              style={{ fontSize: subtitleSizes[subtitleSize] }}
              data-subtitles
            >
              {cueText.split('\n').map((line, i) => (
                <span key={i} className={cn('rounded-md px-[0.4em] py-[0.05em] font-medium leading-snug', subtitleLook(subStyle))}>
                  <CueLine text={line} />
                </span>
              ))}
            </div>
          )}
          {loading && (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
              <Loader2 className="size-12 animate-spin text-white/80" strokeWidth={1.5} />
            </div>
          )}
        </>
      )}

      <div
        className={cn(
          'pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between bg-gradient-to-b from-black/70 to-transparent p-6 transition-opacity duration-500',
          showChrome ? 'opacity-100' : 'opacity-0',
        )}
      >
        <div className="min-w-0 text-white">
          <p className="truncate text-lg font-semibold">{item.title}</p>
          {item.subtitle && <p className="text-sm text-white/60">{item.subtitle}</p>}
        </div>
        <div className="pointer-events-auto flex shrink-0 gap-2">
        {room && !minimized && (
          <TogetherMenu room={room} invite={together.invite} onOpenChange={setMenuOpen}
            onLeave={onClose} onEnd={() => void together.end().catch(() => undefined)} onRenew={() => void together.renew().catch(() => undefined)} />
        )}
        <Button variant="glass" size="icon" aria-label={minimized ? t("player:player.expandPlayer") : t("player:player.minimizePlayer")} onClick={() => { if (!minimized && document.fullscreenElement) void document.exitFullscreen(); onMinimize(); }}>{minimized ? <Maximize /> : <Minimize />}</Button>
        <Button variant="glass" size="icon" className="pointer-events-auto" aria-label={t("player:player.closePlayer")} onClick={onClose}>
          <X />
        </Button>
        </div>
      </div>

      {!!updatePrefs.error && <div className="absolute inset-x-6 top-24 z-10"><ErrorNotice error={updatePrefs.error} onRetry={() => { if (updatePrefs.variables) updatePrefs.mutate(updatePrefs.variables); }} /></div>}
      {!minimized && skippable && !skippedNote && (
        <Button variant="primary" size="lg" className="absolute bottom-72 right-6 sm:bottom-28 z-10 shadow-2xl shadow-black/50" onClick={() => seekTo(skippable.end_ms)}>
          {skippable.kind === 'intro' ? t("player:player.skipIntro") : t("player:player.skipRecap")} <SkipForward />
        </Button>
      )}
      {!minimized && inCredits && upNextIn === null && credits && (
        <Button variant="primary" size="lg" className="absolute bottom-72 right-6 sm:bottom-28 z-10 shadow-2xl shadow-black/50" onClick={() => (next ? onNext() : seekTo(credits.end_ms))}>
          {next ? t("player:player.playNext") : t("player:player.skipCredits")} <SkipForward />
        </Button>
      )}
      {skippedNote && (
        <div className="pointer-events-none absolute bottom-72 right-6 sm:bottom-28 z-10 rounded-full bg-black/70 px-4 py-2 text-sm text-white ring-1 ring-white/15">{skippedNote}</div>
      )}
      {!minimized && item.preroll && next && (
        <Button variant="glass" className="absolute bottom-72 right-6 sm:bottom-28 z-10" onClick={onNext}>{t("player:player.skip")} <SkipForward />
        </Button>
      )}
      {upNextIn !== null && next && (
        <UpNext item={next} seconds={upNextIn} compact={minimized} onTick={() => setUpNextIn((n) => (n === null ? null : n - 1))} onPlay={onNext} onCancel={() => setUpNextIn(null)} />
      )}
      {admin && searching && planState && item.itemId && (
        <SubtitleSearch
          itemId={item.itemId}
          fileId={planState.file_id}
          defaultLanguage={prefs.data?.subtitle_language || planState.audio_tracks.find((a) => a.index === planState.audio_stream)?.language || 'eng'}
          onClose={() => setSearching(false)}
          onAdded={async (track) => {
            setSearching(false);
            const p = plan.current;
            if (p && item.itemId) {
              const tracks = await api.subtitleTracks(item.itemId, p.file_id).catch(() => [...p.subtitle_tracks, track]);
              plan.current = { ...p, subtitle_tracks: tracks };
              setPlanState(plan.current);
            }
            await selectSubtitle(track, true);
            video.current?.play().catch(() => {});
          }}
        />
      )}
      {!minimized && stopped === null && !error && castFrom === null && (
        <div
          className={cn(
            'absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 via-black/40 to-transparent px-6 pb-5 pt-16 text-white transition-opacity duration-500',
            showChrome ? 'opacity-100' : 'pointer-events-none opacity-0',
          )}
        >
          <Scrubber
            positionMS={positionMS}
            durationMS={dur}
            bufferedMS={bufferedMS}
            readyMS={planState?.hls ? readyMS : 0}
            onSeek={seekTo}
            thumbs={thumbs}
            chapters={planState?.chapters ?? []}
          />
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <div className="flex w-full flex-wrap items-center gap-1 sm:w-auto sm:flex-1">
            <ControlButton label={paused ? t("player:player.playSpace") : t("player:player.pauseSpace")} onClick={togglePlay}>
              {paused ? <Play className="fill-current" /> : <Pause className="fill-current" />}
            </ControlButton>
            {onPrevious && (
              <ControlButton label={t("player:player.previousShiftP")} onClick={onPrevious}>
                <SkipBack className="fill-current" />
              </ControlButton>
            )}
            {next && (
              <ControlButton label={t('player:player.nextTitleShortcut', { title: next.title })} onClick={onNext}>
                <SkipForward className="fill-current" />
              </ControlButton>
            )}
            <ControlButton label={t("player:player.back10SecondsJ")} onClick={() => seekTo(absMS() - 10_000)}>
              <RotateCcw />
            </ControlButton>
            <ControlButton label={t("player:player.forward10SecondsL")} onClick={() => seekTo(absMS() + 10_000)}>
              <RotateCw />
            </ControlButton>
            <VolumeControl volume={volume} muted={muted} onVolume={(v) => (setVolume(v), setMuted(false))} onMute={() => setMuted((m) => !m)} />
            <span className="ml-2 text-[13px] tabular-nums text-white/80">
              {fmt.clock(positionMS)} <span className="text-white/40">/ {dur ? fmt.clock(dur) : '–:––'}</span>
            </span>
            </div>
            <div className="flex w-full flex-wrap items-center gap-1 sm:w-auto">
            {planState && planState.audio_tracks.length > 1 && (
              <PlayerMenu icon={<Languages />} label={t("player:player.audio")} onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{t("player:player.audio")}</DropdownMenuLabel>
                {planState.audio_tracks.map((tabId, i) => {
                  const { name, detail } = audioLabel(tabId, i);
                  return (
                    <DropdownMenuItem
                      key={tabId.index}
                      onSelect={() => {
                        if (tabId.index === planState.audio_stream) return;
                        void load(absMS(), tabId.index);
                        // Remembered for the profile, like subtitles: played by default from now on.
                        if (tabId.language && tabId.language !== prefs.data?.audio_language) updatePrefs.mutate({ audio_language: tabId.language });
                      }}
                    >
                      <span className="w-4">{tabId.index === planState.audio_stream && <Check className="size-4 text-accent" />}</span>
                      <span className="min-w-0">
                        <span className="block">{name}</span>
                        <span className="block text-xs text-subtle">{detail}</span>
                      </span>
                    </DropdownMenuItem>
                  );
                })}
              </PlayerMenu>
            )}
            {planState && (planState.subtitle_tracks.length > 0 || item.itemId) && (
              <PlayerMenu icon={<Captions />} label={t("player:player.subtitlesC")} onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{t("player:player.subtitles")}</DropdownMenuLabel>
                <DropdownMenuItem onSelect={() => void selectSubtitle(null, true)}>
                  <span className="w-4">{!subtitle && <Check className="size-4 text-accent" />}</span>{t("player:player.off")}</DropdownMenuItem>
                {planState.subtitle_tracks.map((tabId, i) => {
                  const { name, detail } = subtitleLabel(tabId, i);
                  return (
                    <DropdownMenuItem key={tabId.id} onSelect={() => tabId.id !== subtitle?.id && void selectSubtitle(tabId, true)}>
                      <span className="w-4">{tabId.id === subtitle?.id && <Check className="size-4 text-accent" />}</span>
                      <span className="min-w-0">
                        <span className="block">{name}</span>
                        {detail && <span className="block text-xs text-subtle">{detail}</span>}
                      </span>
                    </DropdownMenuItem>
                  );
                })}
                {subtitleError && <p className="max-w-64 px-2.5 py-1.5 text-xs text-warning">{subtitleError}</p>}
                {subtitle && !subtitle.image && <>
                  <DropdownMenuSeparator />
                  <DropdownMenuLabel>{subtitleOffsetMS > 0 ? t('player:player.timingLater', { seconds: fmt.fixed(subtitleOffsetMS / 1000, 1) }) : subtitleOffsetMS < 0 ? t('player:player.timingEarlier', { seconds: fmt.fixed(subtitleOffsetMS / 1000, 1) }) : t('player:player.timingZero')}</DropdownMenuLabel>
                  <DropdownMenuItem disabled={subtitleOffsetMS <= -10000} onSelect={(e) => { e.preventDefault(); shiftSubtitles(-500); }}>{t("player:player.show05sEarlier")}</DropdownMenuItem>
                  <DropdownMenuItem disabled={subtitleOffsetMS >= 10000} onSelect={(e) => { e.preventDefault(); shiftSubtitles(500); }}>{t("player:player.show05sLater")}</DropdownMenuItem>
                  <DropdownMenuItem disabled={subtitleOffsetMS === 0} onSelect={(e) => { e.preventDefault(); shiftSubtitles(null); }}>{t("player:player.resetSubtitleTiming")}</DropdownMenuItem>
                </>}
                {subtitle?.image && <p className="max-w-64 px-2.5 py-1.5 text-xs text-white/60">{t("player:player.timingAdjustmentIsAvailable")}</p>}
                {admin && item.itemId && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      onSelect={() => {
                        video.current?.pause();
                        setSearching(true);
                      }}
                    >
                      <span className="w-4" />{t("player:player.findOnline")}</DropdownMenuItem>
                  </>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuLabel>{t("player:player.size")}</DropdownMenuLabel>
                {(['small', 'medium', 'large', 'huge'] as const).map((sz) => (
                  <DropdownMenuItem
                    key={sz}
                    onSelect={(e) => {
                      e.preventDefault();
                      setSubtitleSize(sz);
                    }}
                  >
                    <span className="w-4">{sz === subtitleSize && <Check className="size-4 text-accent" />}</span>
                    {{ small: t('player:player.sizeSmall'), medium: t('player:player.sizeMedium'), large: t('player:player.sizeLarge'), huge: t('player:player.sizeHuge') }[sz]}
                  </DropdownMenuItem>
                ))}
              </PlayerMenu>
            )}
            {planState && planState.versions.length > 1 && (
              <PlayerMenu icon={<Film />} label={t("player:player.version")} onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{t("player:player.version")}</DropdownMenuLabel>
                {planState.versions.map((v) => {
                  const { name, detail } = versionLabel(v);
                  return (
                    <DropdownMenuItem key={v.file_id} onSelect={() => void selectVersion(v.file_id)}>
                      <span className="w-4">{v.file_id === planState.file_id && <Check className="size-4 text-accent" />}</span>
                      <span className="min-w-0">
                        <span className="block">{name}</span>
                        <span className="block text-xs text-subtle">{detail}</span>
                      </span>
                    </DropdownMenuItem>
                  );
                })}
              </PlayerMenu>
            )}
            {planState && item.itemId && (
              <PlayerMenu icon={<SlidersHorizontal />} label={t("player:player.quality")} onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{t("player:player.quality")}</DropdownMenuLabel>
                {[-1, 0, ...qualities.filter((q) => (!planState.source_kbps || q < planState.source_kbps) && (!qualityCap || q < qualityCap))].map((q) => (
                  <DropdownMenuItem
                    key={q}
                    role="menuitemradio"
                    aria-checked={q === shownQuality}
                    onSelect={() => {
                      if (q === qualityMode.current) return;
                      const previousLimit = quality.current;
                      qualityMode.current = q;
                      quality.current = Math.max(0, q);
                      adaptive.current = new AdaptiveQuality(qualities);
                      adaptive.current.reset(Date.now());
                      setAutomaticLimit(0); setQualityNotice('');
                      setQualityState(q);
                      saveQuality(q);
                      if (quality.current === previousLimit && installedGeneration.current === loadGeneration.current) return;
                      void load(absMS(), planState.audio_stream, caps.current, true);
                    }}
                  >
                    <span className="w-4">{q === shownQuality && <Check className="size-4 text-accent" />}</span>
                    {q === -1 ? t("player:player.automatic") : q ? qualityName(q) : qualityCap ? t("player:player.maximumCapped", { quality: qualityName(qualityCap) }) : t("player:player.original")}
                  </DropdownMenuItem>
                ))}
                {qualityCap > 0 && <p className="max-w-64 px-2.5 py-1.5 text-xs text-subtle">{t("player:player.awayFromHomeLimit", { quality: qualityName(qualityCap) })}</p>}
                {qualityState === -1 && <DropdownMenuLabel className="max-w-64 whitespace-normal normal-case font-normal">{automaticLimit > 0 ? t('player:player.adaptsWithLimit', { quality: qualityName(Math.min(automaticLimit, planState.server_limit_kbps || Infinity)) }) : t("player:player.adaptsToPlaybackDelivery")}
                </DropdownMenuLabel>}
                {qualityNotice && <p role="status" className="max-w-64 px-2.5 py-2 text-xs text-subtle">{qualityNotice}</p>}
              </PlayerMenu>
            )}
            {planState && planState.chapters.length > 1 && (
              <PlayerMenu icon={<ListOrdered />} label={t("player:player.chapters")} onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{t("player:player.chapters")}</DropdownMenuLabel>
                <div className="max-h-80 overflow-y-auto">
                  {planState.chapters.map((c, i) => (
                    <DropdownMenuItem key={c.start_ms} onSelect={() => seekTo(c.start_ms)}>
                      <span className="w-4">{i === chapterAt(planState.chapters, positionMS) && <Check className="size-4 text-accent" />}</span>
                      <span className="min-w-0 flex-1 truncate">{chapterName(planState.chapters, i)}</span>
                      <span className="ml-4 text-xs tabular-nums text-subtle">{fmt.clock(c.start_ms)}</span>
                    </DropdownMenuItem>
                  ))}
                </div>
              </PlayerMenu>
            )}
            <ControlButton label={t("player:player.playbackQueue")} onClick={() => setQueueOpen(true)}><ListVideo /></ControlButton>
            {!session && (
              <PlayerMenu icon={<Gauge />} label={t("player:player.speed")} onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{t("player:player.speed")}</DropdownMenuLabel>
                {speeds.map((s) => (
                  <DropdownMenuItem key={s} onSelect={() => setRate(s)}>
                    <span className="w-4">{s === rate && <Check className="size-4 text-accent" />}</span>
                    {s === 1 ? t("player:player.normal") : `${s}×`}
                  </DropdownMenuItem>
                ))}
              </PlayerMenu>
            )}
            {planState && (
              <PlayerMenu icon={<Info />} label={t("player:player.playbackInfo")} onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{methodLabel(planState).title}</DropdownMenuLabel>
                <p className="max-w-64 px-2.5 pb-2 text-xs leading-relaxed text-muted">{methodLabel(planState).detail}</p>
                <DropdownMenuSeparator />
                <p className="max-w-64 px-2.5 py-1.5 text-xs leading-relaxed text-subtle">{t("player:player.spacePlayPause5s")}</p>
              </PlayerMenu>
            )}
            {castAvailable && item.itemId && (
              <ControlButton label={t("player:player.castToAChromecast")} onClick={() => void startCast()}>
                <Cast />
              </ControlButton>
            )}
            {airplayAvailable && item.itemId && (
              <ControlButton
                label={t("player:player.airplay")}
                onClick={() => {
                  if (subtitle && !subtitle.image) {
                    setTrackNotice(t('player:player.airplayTextWarning'));
                    return;
                  }
                  (video.current as HTMLVideoElement & { webkitShowPlaybackTargetPicker?: () => void } | null)?.webkitShowPlaybackTargetPicker?.();
                }}
              >
                <Airplay />
              </ControlButton>
            )}
            {pipSupported && (
              <ControlButton label={pip ? t("player:player.backFromPictureIn") : t("player:player.pictureInPictureI")} onClick={togglePip}>
                <PictureInPicture2 />
              </ControlButton>
            )}
            <ControlButton label={fullscreen ? t("player:player.exitFullScreenF") : t("player:player.fullScreenF")} onClick={toggleFullscreen}>
              {fullscreen ? <Minimize /> : <Maximize />}
            </ControlButton>
            </div>
          </div>
        </div>
      )}
      {minimized && <div className="absolute inset-x-0 bottom-0 flex justify-center gap-2 bg-black/70 p-2"><ControlButton label={paused ? t("player:player.playSpace") : t("player:player.pauseSpace")} onClick={togglePlay}>{paused ? <Play /> : <Pause />}</ControlButton>{next && <ControlButton label={t('player:player.nextTitle', { title: next.title })} onClick={onNext}><SkipForward /></ControlButton>}<ControlButton label={t("player:player.playbackQueue")} onClick={() => setQueueOpen(true)}><ListVideo /></ControlButton></div>}
      <Dialog open={queueOpen} onOpenChange={setQueueOpen}>
        <DialogContent title={t("player:player.playbackQueue")} description={t("player:player.arrangeWhatPlaysNext")} overlayClassName="z-[120]" className="z-[121]" footer={<Button variant="secondary" disabled={!next} onClick={queue.clear}>{t("player:player.clearUpcoming")}</Button>}>
          {queue.notice && <p role="status" className="mb-3 text-sm text-muted">{queue.notice}</p>}
          <ol className="space-y-3">
            {queue.state?.items.slice(queue.state.index).map((entry, i, items) => <li key={entry.queueId} className={cn('flex flex-wrap items-center gap-2 rounded-xl bg-surface-2 p-3', entry.unavailable && 'opacity-60')}>
              <div className="min-w-0 flex-1 basis-40"><p className="break-words text-sm font-medium">{entry.title}</p><p className="text-xs text-muted">{i === 0 ? t("player:player.nowPlaying") : t("player:player.upNext")}{entry.subtitle ? ' · ' + entry.subtitle : ''}</p></div>
              {i > 0 && <div className="flex gap-1"><Button size="icon" variant="ghost" aria-label={t('player:player.moveUp', { title: entry.title })} disabled={i === 1} onClick={() => queue.move(entry.queueId, -1)}><ArrowUp /></Button><Button size="icon" variant="ghost" aria-label={t('player:player.moveDown', { title: entry.title })} disabled={i === items.length - 1} onClick={() => queue.move(entry.queueId, 1)}><ArrowDown /></Button><Button size="icon" variant="ghost" aria-label={t('player:player.playNowTitle', { title: entry.title })} onClick={() => { queue.jump(entry.queueId); setQueueOpen(false); }}><Play /></Button><Button size="icon" variant="ghost" aria-label={t('player:player.removeQueue', { title: entry.title })} onClick={() => queue.remove(entry.queueId)}><Trash2 /></Button></div>}
            </li>)}
          </ol>
        </DialogContent>
      </Dialog>
    </motion.div>
  );
}

type SubtitleSize = NonNullable<SubtitleStyle['size']>;
const subtitleSizes: Record<SubtitleSize, string> = {
  small: 'clamp(14px, 2.4vh, 28px)',
  medium: 'clamp(16px, 3.2vh, 38px)',
  large: 'clamp(18px, 4.3vh, 52px)',
  huge: 'clamp(22px, 5.6vh, 68px)',
};

/** A cue line's colour and background, from the profile's subtitle style. */
export function subtitleLook(st: SubtitleStyle) {
  return cn(
    st.color === 'yellow' ? 'text-[#ffe14d]' : 'text-white',
    st.background === 'solid'
      ? 'bg-black'
      : st.background === 'none'
        ? '[text-shadow:0_0_3px_#000,0_0_3px_#000,0_1px_2px_#000,0_0_8px_rgb(0_0_0/0.8)]'
        : 'bg-black/65 [text-shadow:0_1px_2px_rgb(0_0_0/0.6)]',
  );
}

/** The size chosen on this device before sizes were kept with the profile. */
function readSubtitleSize(): SubtitleSize {
  try {
    const v = localStorage.getItem('arc-subtitle-size');
    if (v === 'small' || v === 'medium' || v === 'large') return v;
  } catch {
    // the default below
  }
  return 'medium';
}

/** One line of a cue, with its italic, bold and underline tags (and nothing else) applied. */
function CueLine({ text }: { text: string }) {
  const parts: ReactNode[] = [];
  const style = { i: 0, b: 0, u: 0 };
  text.split(/(<\/?[ibu]>)/).forEach((tok, i) => {
    const m = /^<(\/?)([ibu])>$/.exec(tok);
    if (m) {
      style[m[2] as 'i' | 'b' | 'u'] += m[1] ? -1 : 1;
      return;
    }
    if (!tok) return;
    parts.push(
      <span key={i} className={cn(style.i > 0 && 'italic', style.b > 0 && 'font-bold', style.u > 0 && 'underline')}>
        {tok}
      </span>,
    );
  });
  return <>{parts}</>;
}

/** The timeline thumbnail for a moment: one tile of a sheet, scaled to 192px wide. */
function Thumb({ ms, thumbs }: { ms: number; thumbs: { info: TrickplayInfo; sheets: string[] } }) {
  const { info, sheets } = thumbs;
  const index = Math.min(info.count - 1, Math.max(0, Math.floor(ms / info.interval_ms)));
  const perSheet = info.columns * info.rows;
  const sheet = sheets[Math.floor(index / perSheet)];
  if (!sheet) return null;
  const pos = index % perSheet;
  const scale = 192 / info.width;
  return (
    <div
      className="mb-1.5 rounded-md bg-black bg-no-repeat ring-1 ring-white/20"
      style={{
        width: 192,
        height: Math.round(info.height * scale),
        backgroundImage: `url(${sheet})`,
        backgroundSize: `${info.columns * 192}px ${info.rows * info.height * scale}px`,
        backgroundPosition: `-${(pos % info.columns) * 192}px -${Math.floor(pos / info.columns) * info.height * scale}px`,
      }}
    />
  );
}

/** The next item in a queue, counting down to it. */
function UpNext({ item, seconds, compact = false, onTick, onPlay, onCancel }: { item: NowPlaying; seconds: number; compact?: boolean; onTick: () => void; onPlay: () => void; onCancel: () => void }) {
  const { t } = useTranslation('player');
  // The callbacks change with every render of the player; the countdown only follows the seconds.
  const callbacks = useRef({ onTick, onPlay });
  callbacks.current = { onTick, onPlay };
  useEffect(() => {
    if (seconds <= 0) {
      callbacks.current.onPlay();
      return;
    }
    const timer = window.setTimeout(() => callbacks.current.onTick(), 1000);
    return () => window.clearTimeout(timer);
  }, [seconds]);
  if (compact) return (
    <div className="absolute inset-x-2 bottom-16 flex items-center justify-between rounded-xl bg-black/80 px-3 py-1 text-white">
      <p className="text-xs">{t('player:player.upNextCountdown', { count: seconds })}</p>
      <Button variant="glass" size="sm" onClick={onCancel}>{t("player:player.cancel")}</Button>
    </div>
  );
  return (
    <div className="absolute bottom-72 right-6 sm:bottom-28 z-10 w-80 rounded-2xl bg-black/80 p-4 text-white shadow-2xl ring-1 ring-white/15 backdrop-blur-xl">
      <p className="text-xs font-semibold uppercase tracking-wider text-white/50">{t('player:player.upNextCountdown', { count: seconds })}</p>
      <p className="mt-1 truncate font-semibold">{item.title}</p>
      {item.subtitle && <p className="truncate text-sm text-white/60">{item.subtitle}</p>}
      <div className="mt-3 flex gap-2">
        <Button variant="primary" size="sm" onClick={onPlay}>
          <Play className="fill-current" /> {t("player:player.playNow")}</Button>
        <Button variant="glass" size="sm" onClick={onCancel}>{t("player:player.cancel")}</Button>
      </div>
    </div>
  );
}

/** After a reload, the queue this device was playing, to carry on with or put away. */
function RecoveredQueue({ queue, onResume, onDismiss }: { queue: PlayQueue; onResume: () => void; onDismiss: () => void }) {
  const { t } = useTranslation('player');
  const at = queue.entries.findIndex((e) => e.id === queue.current_entry_id);
  const upcoming = queue.entries.slice(at);
  const first = upcoming.find((e) => e.available && e.item)?.item;
  return (
    <div role="region" aria-label={t('player:player.recoveredQueue')} className="fixed bottom-4 right-4 left-4 z-[90] flex flex-wrap items-center gap-3 rounded-2xl border border-border bg-surface p-4 shadow-2xl shadow-black/40 sm:left-auto sm:max-w-sm">
      <ListVideo className="size-5 shrink-0 text-accent" />
      <div className="min-w-0 flex-1 basis-40">
        <p className="text-sm font-medium">{t('player:player.recoveredQueue')}</p>
        <p className="truncate text-xs text-muted">{first && (upcoming.length > 1 ? t('player:player.recoveredQueueMore', { title: first.title, count: upcoming.length - 1 }) : first.title)}</p>
      </div>
      <div className="flex gap-2">
        <Button size="sm" variant="accent" onClick={onResume}><Play className="size-4" />{t('player:player.resumeQueue')}</Button>
        <Button size="icon" variant="ghost" aria-label={t('player:player.dismissQueue')} onClick={onDismiss}><X /></Button>
      </div>
    </div>
  );
}

function Message({ icon, title, children, onClose, onRetry }: { icon: ReactNode; title: string; children: ReactNode; onClose: () => void; onRetry?: () => void }) {
  const { t } = useTranslation('player');
  return (
    <div className="max-w-md px-6 text-center text-white">
      <div className="mx-auto mb-4 flex justify-center [&_svg]:size-10 [&_svg]:stroke-[1.5]">{icon}</div>
      <h2 className="text-xl font-semibold">{title}</h2>
      <p className="mt-2 text-sm text-white/60">{children}</p>
      {onRetry && <Button variant="glass" className="mr-3 mt-6" onClick={onRetry}>{t("player:player.retryPlayback")}</Button>}
      <Button variant="glass" className="mt-6" onClick={onClose}>{t("player:player.close")}</Button>
    </div>
  );
}

function ControlButton({ label, onClick, children }: { label: string; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className="flex size-11 shrink-0 items-center justify-center rounded-full text-white/90 outline-none transition-colors hover:bg-white/10 hover:text-white focus-visible:ring-2 focus-visible:ring-white/60 [&_svg]:size-5"
    >
      {children}
    </button>
  );
}

function PlayerMenu({ icon, label, children, onOpenChange }: { icon: ReactNode; label: string; children: ReactNode; onOpenChange: (open: boolean) => void }) {
  return (
    <DropdownMenu onOpenChange={onOpenChange}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={label}
          title={label}
          className="flex size-11 shrink-0 items-center justify-center rounded-full text-white/90 outline-none transition-colors hover:bg-white/10 hover:text-white focus-visible:ring-2 focus-visible:ring-white/60 data-[state=open]:bg-white/15 [&_svg]:size-5"
        >
          {icon}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" side="top" className="z-[110] min-w-52">
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function VolumeControl({ volume, muted, onVolume, onMute }: { volume: number; muted: boolean; onVolume: (v: number) => void; onMute: () => void }) {
  const { t } = useTranslation('player');
  const level = muted ? 0 : volume;
  return (
    <div className="group/vol hidden items-center sm:flex">
      <ControlButton label={muted ? t("player:player.unmuteM") : t("player:player.muteM")} onClick={onMute}>
        {level === 0 ? <VolumeX /> : level < 0.5 ? <Volume1 /> : <Volume2 />}
      </ControlButton>
      <input
        type="range"
        min={0}
        max={1}
        step={0.05}
        value={level}
        aria-label={t("player:player.volume")}
        onChange={(e) => onVolume(Number(e.target.value))}
        className="w-0 cursor-pointer accent-white opacity-0 transition-all duration-200 group-hover/vol:w-24 group-hover/vol:opacity-100 focus:w-24 focus:opacity-100"
      />
    </div>
  );
}

/** The timeline: played, buffered, and (for converted streams) how far Arc has converted. */
function Scrubber({
  positionMS,
  durationMS,
  bufferedMS,
  readyMS,
  onSeek,
  thumbs,
  chapters,
}: {
  positionMS: number;
  durationMS: number;
  bufferedMS: number;
  readyMS: number;
  onSeek: (ms: number) => void;
  thumbs: { info: TrickplayInfo; sheets: string[] } | null;
  chapters: Chapter[];
}) {
  const { t } = useTranslation('player');
  const bar = useRef<HTMLDivElement>(null);
  const [hover, setHover] = useState<number | null>(null);
  const [drag, setDrag] = useState<number | null>(null);
  const pct = (ms: number) => (durationMS ? `${Math.min(100, Math.max(0, (ms / durationMS) * 100))}%` : '0%');
  const at = (clientX: number) => {
    const r = bar.current!.getBoundingClientRect();
    return Math.min(1, Math.max(0, (clientX - r.left) / r.width)) * durationMS;
  };
  const shown = drag ?? positionMS;

  return (
    <div
      ref={bar}
      role="slider"
      aria-label={t("player:player.position")}
      aria-valuemin={0}
      aria-valuemax={Math.round(durationMS / 1000)}
      aria-valuenow={Math.round(shown / 1000)}
      tabIndex={-1}
      className="group/scrub relative flex h-5 cursor-pointer touch-none items-center"
      onPointerMove={(e) => {
        if (!durationMS) return;
        setHover(at(e.clientX));
        if (drag !== null) setDrag(at(e.clientX));
      }}
      onPointerLeave={() => setHover(null)}
      onPointerDown={(e) => {
        if (!durationMS) return;
        e.currentTarget.setPointerCapture(e.pointerId);
        setDrag(at(e.clientX));
      }}
      onPointerUp={(e) => {
        if (drag === null) return;
        onSeek(at(e.clientX));
        setDrag(null);
      }}
    >
      <div className="relative h-1 w-full overflow-hidden rounded-full bg-white/20 transition-[height] duration-150 group-hover/scrub:h-1.5">
        {readyMS > 0 && <div className="absolute inset-y-0 left-0 bg-white/15" style={{ width: pct(readyMS) }} />}
        <div className="absolute inset-y-0 left-0 bg-white/30" style={{ width: pct(bufferedMS) }} />
        <div className="absolute inset-y-0 left-0 bg-white" style={{ width: pct(shown) }} />
        {chapters.slice(1).map((c) => (
          <div key={c.start_ms} className="absolute inset-y-0 w-[3px] -translate-x-1/2 bg-black/70" style={{ left: pct(c.start_ms) }} />
        ))}
      </div>
      <div
        className="absolute size-3.5 -translate-x-1/2 rounded-full bg-white shadow opacity-0 transition-opacity group-hover/scrub:opacity-100"
        style={{ left: pct(shown) }}
      />
      {hover !== null && (
        <div
          className="pointer-events-none absolute bottom-6 flex -translate-x-1/2 flex-col items-center rounded-lg bg-black/80 px-1.5 py-1 text-xs tabular-nums text-white"
          style={{ left: pct(drag ?? hover) }}
        >
          {thumbs && <Thumb ms={drag ?? hover} thumbs={thumbs} />}
          {chapters.length > 0 && (
            <span className="max-w-48 truncate font-medium">{chapterName(chapters, chapterAt(chapters, drag ?? hover))}</span>
          )}
          {fmt.clock(drag ?? hover)}
        </div>
      )}
    </div>
  );
}

/** The index of the chapter a time (ms) falls in: the last one starting at or before it. */
function chapterAt(chapters: Chapter[], ms: number) {
  let i = 0;
  while (i + 1 < chapters.length && chapters[i + 1].start_ms <= ms) i++;
  return i;
}

/** A chapter's name: its title, unless that's missing or just a number or time. */
function chapterName(chapters: Chapter[], i: number) {
  const localValue = chapters[i]?.title?.trim();
  return localValue && !/^(chapter\s*)?[\d:.]+$/i.test(localValue) ? localValue : i18n.t('player:player.chapterNumber', { number: i + 1 });
}

/** What the player needs to play a movie or episode, from where the profile left off. */
export function nowPlayingFor(item: Item, fromStart = false): NowPlaying {
  const episode = item.kind === 'episode';
  return {
    title: item.title,
    subtitle: episode
      ? [item.series_title, episodeCode(item.season_number, item.episode_number)].filter(Boolean).join(' · ')
      : item.year
        ? String(item.year)
        : undefined,
    src: item.stream_url,
    itemId: item.id,
    startMS: fromStart || item.user_data?.played ? 0 : item.user_data?.position_ms,
  };
}
