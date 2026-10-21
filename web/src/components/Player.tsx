import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { AnimatePresence, motion } from 'motion/react';
import type HlsType from 'hls.js';
import {
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
import { api, ApiError, type SubtitleStyle, type Chapter, type Item, type PlaybackPlan, type PlayerCaps, type SubtitleCue, type SubtitleTrack, type TrickplayInfo } from '@/lib/api';
import { episodeCode } from './WatchState';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import * as fmt from '@/lib/format';
import { audioLabel, chooseSubtitle, fallbackCaps, methodLabel, playerCaps, qualities, qualityName, readQuality, saveQuality, subtitleLabel, versionLabel } from '@/lib/playback';
import { cn } from '@/lib/utils';
import { Button } from './ui/button';
import { SubtitleSearch } from './SubtitleSearch';
import { CastSession, chooseChromecast, useCastAvailable } from './Cast';
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
  const qc = useQueryClient();
  const [queue, setQueue] = useState<{ items: NowPlaying[]; index: number } | null>(null);
  const play = useCallback((item: NowPlaying, list?: NowPlaying[]) => {
    const start = list?.length ? { items: list, index: Math.max(0, list.indexOf(item)) } : { items: [item], index: 0 };
    // A movie played from the start gets the pre-roll videos set in Settings first.
    if (!item.itemId || item.preroll || (item.startMS ?? 0) > 0) {
      setQueue(start);
      return;
    }
    api
      .preroll(item.itemId)
      .catch(() => [])
      .then((clips) => {
        if (!clips.length) return setQueue(start);
        const prerolls: NowPlaying[] = clips.map((c) => ({ title: c.title, subtitle: `Before ${item.title}`, src: c.url, preroll: true }));
        setQueue({ items: [...start.items.slice(0, start.index), ...prerolls, ...start.items.slice(start.index)], index: start.index });
      });
  }, []);
  const go = useCallback((delta: number) => setQueue((q) => (q && q.items[q.index + delta] ? { ...q, index: q.index + delta } : q)), []);
  // Auto-Play: an episode with nothing queued after it carries on with the next one in the
  // profile's episode order (the countdown, Next and Shift+N then work as in a playlist).
  const autoPlay = usePrefs().data?.auto_play !== false;
  const current = queue?.items[queue.index];
  const hasNext = !!queue?.items[queue.index + 1];
  useEffect(() => {
    if (!current?.itemId || hasNext || !autoPlay) return;
    let cancelled = false;
    api
      .nextEpisode(current.itemId)
      .then((ep) => {
        if (cancelled || !ep?.stream_url) return;
        setQueue((q) => (q && q.items[q.index] === current && !q.items[q.index + 1] ? { ...q, items: [...q.items, nowPlayingFor(ep, true)] } : q));
      })
      .catch(() => undefined); // playing on is a nicety; the episode itself plays regardless
    return () => {
      cancelled = true;
    };
  }, [current, hasNext, autoPlay]);
  const close = useCallback(() => {
    setQueue(null);
    // Watched state and Continue Watching have changed.
    window.setTimeout(() => {
      for (const key of ['item', 'items', 'seasons', 'continue', 'next-up']) qc.invalidateQueries({ queryKey: [key] });
    }, 300);
  }, [qc]);
  return (
    <PlayerContext.Provider value={play}>
      {children}
      <AnimatePresence>
        {queue && (
          <PlayerOverlay
            key={queue.index}
            item={queue.items[queue.index]}
            next={queue.items[queue.index + 1]}
            onNext={() => go(1)}
            onPrevious={queue.index > 0 && !queue.items[queue.index - 1].preroll ? () => go(-1) : undefined}
            onClose={close}
          />
        )}
      </AnimatePresence>
    </PlayerContext.Provider>
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
}: {
  item: NowPlaying;
  /** What plays after this one, in a queue. */
  next?: NowPlaying;
  onNext: () => void;
  onPrevious?: () => void;
  onClose: () => void;
}) {
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
  const burn = useRef<string | undefined>(undefined);
  // The quality limit chosen on this device (0 for the original); restarts keep it.
  const quality = useRef(readQuality());
  const [qualityState, setQualityState] = useState(quality.current);
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
  const [subtitleError, setSubtitleError] = useState<string | null>(null);
  const subStyle = prefs.data?.subtitle_style ?? {};
  const subtitleSize: SubtitleSize = subStyle.size ?? readSubtitleSize();
  const setSubtitleSize = (size: SubtitleSize) => updatePrefs.mutate({ subtitle_style: { ...subStyle, size } });
  const [cueText, setCueText] = useState('');
  const autoChosen = useRef(false);
  // Thumbnails for the timeline, if the library makes them.
  const [thumbs, setThumbs] = useState<{ info: TrickplayInfo; sheets: string[] } | null>(null);
  const lastSubtitle = useRef<SubtitleTrack | null>(null);
  // The panel for finding subtitles online.
  const [searching, setSearching] = useState(false);
  // Seconds until the next item in the queue starts, once this one has ended.
  const [upNextIn, setUpNextIn] = useState<number | null>(null);
  const hideTimer = useRef<number>(undefined);
  const lastReport = useRef(0);
  // Casting to a Chromecast: where it started (the cast panel takes over until it stops).
  const castAvailable = useCastAvailable();
  const [castFrom, setCastFrom] = useState<number | null>(null);
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
      api.reportProgress(item.itemId, absMS(), dur, state).catch((e) => {
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
    async (startMS: number, audioStream?: number, withCaps: PlayerCaps = caps.current) => {
      const generation = ++loadGeneration.current;
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
          p = await api.startPlayback(item.itemId, { caps: withCaps, start_ms: Math.round(startMS), audio_stream: audioStream, file_id: plan.current?.file_id, burn_subtitle: burn.current, edition: item.edition, max_kbps: quality.current || undefined });
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
          v.play().catch(() => {});
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
            v.play().catch(() => {});
            return;
          }
          const h = new Hls({ startPosition: Math.max(0, (startMS - p.offset_ms) / 1000), maxBufferLength: 60, backBufferLength: 120 });
          h.on(Hls.Events.ERROR, (_e, data) => {
            if (data.fatal) void onStreamError(generation);
          });
          h.loadSource(p.url);
          h.attachMedia(v);
          hls.current = h;
        }
        v.play().catch(() => {});
        // Some browsers accept a format and then never show a frame: give up on it after a while.
        window.clearTimeout(watchdog.current);
        watchdog.current = window.setTimeout(() => {
          if (video.current && video.current.readyState < 2) void onStreamError(generation);
        }, 15_000);
      } finally {
        release();
      }
    },
    // onStreamError is stable through refs; listing it would recreate load on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [item.itemId, item.src, detach],
  );

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
          setStopped(body.error ?? "The server's admin stopped this.");
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
    setError(item.itemId ? "Arc couldn't play this file." : "This file can't be played in the browser.");
  }, [absMS, detach, item.itemId, item.src, item.preroll, next, onNext, load]);

  // Show a subtitle track (null for none). Text ones are drawn here; picture ones are burned into
  // the video, which restarts it converted. remember saves the language for next time.
  const selectSubtitle = useCallback(
    async (t: SubtitleTrack | null, remember: boolean) => {
      const p = plan.current;
      if (!p || !item.itemId) return;
      // Turning them off forgets the language only where the language is what turns them on.
      if (remember && t && !t.forced) updatePrefs.mutate({ subtitle_language: t.language ?? '' });
      else if (remember && !t && !prefs.data?.subtitle_mode) updatePrefs.mutate({ subtitle_language: '' });
      if (t) lastSubtitle.current = t;
      setSubtitle(t);
      setCues([]);
      setSubtitleError(null);
      const wantBurn = t?.image ? t.id : undefined;
      if (wantBurn !== burn.current) {
        burn.current = wantBurn;
        await load(absMS(), p.audio_stream);
      }
      if (!t || t.image) return;
      try {
        const res = await api.subtitleCues(item.itemId, t.id, p.file_id);
        setCues(res.cues);
      } catch (e) {
        setSubtitleError(e instanceof Error ? e.message : String(e));
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
      plan.current = { ...p, file_id: fileId };
      burn.current = undefined;
      setSubtitle(null);
      setCues([]);
      autoChosen.current = false;
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
      .then((t) => !cancelled && setThumbs(t))
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
    const t = chooseSubtitle(planState.subtitle_tracks, prefs.data.subtitle_language, audioLang, prefs.data.subtitle_mode);
    if (t) void selectSubtitle(t, false);
  }, [planState, prefs.data, selectSubtitle]);

  // The cue on screen, checked every frame so it appears on time.
  useEffect(() => {
    if (!cues.length) {
      setCueText('');
      return;
    }
    let raf = 0;
    const tick = () => {
      const t = absMS();
      const text = cues
        .filter((c) => c.start_ms <= t && t < c.end_ms)
        .map((c) => c.text)
        .join('\n');
      setCueText((prev) => (prev === text ? prev : text));
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [cues, absMS]);

  // Save while the video ref still exists. Passive effect cleanup runs after React detaches
  // the element, losing the final position if no periodic report has been sent yet.
  useLayoutEffect(() => () => report('stopped'), [report]);

  // Start playing when the player opens; end the conversion when it closes.
  useEffect(() => {
    void load(item.startMS ?? 0);
    const onHide = () => report('stopped');
    window.addEventListener('pagehide', onHide);
    return () => {
      loadGeneration.current++;
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
      void load(absMS());
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
    const v = video.current;
    v?.pause();
    detach();
    v?.removeAttribute('src');
    v?.load();
    setCastFrom(at);
  }, [absMS, detach]);

  // Keep a converting session alive while paused.
  useEffect(() => {
    const sid = planState?.session_id;
    if (!sid) return;
    const t = window.setInterval(() => {
      api.pingTranscode(sid).catch(() => {});
    }, pingEveryMS);
    return () => window.clearInterval(t);
  }, [planState?.session_id]);

  const seekTo = useCallback(
    (target: number) => {
      const v = video.current;
      const p = plan.current;
      if (!v) return;
      const dur = durationMS || (v.duration || 0) * 1000;
      const t = Math.max(0, dur ? Math.min(target, dur - 500) : target);
      setPositionMS(t);
      if (!p?.hls) {
        v.currentTime = t / 1000;
        return;
      }
      if (requestedPosition.current !== null) {
        void load(t, p.audio_stream);
        return;
      }
      const rel = (t - offset.current) / 1000;
      const end = v.seekable.length ? v.seekable.end(v.seekable.length - 1) : 0;
      if (rel >= 0 && rel <= end - 1) {
        v.currentTime = rel;
      } else {
        // Not converted yet (or before where this conversion started): start again from there,
        // the same way as now.
        void load(t, p.audio_stream);
      }
    },
    [durationMS, load],
  );

  const togglePlay = useCallback(() => {
    const v = video.current;
    if (!v) return;
    if (v.paused) v.play().catch(() => {});
    else v.pause();
  }, []);

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
    const track = v.addTextTrack('subtitles', 'Subtitles');
    track.mode = 'showing';
    const offsetS = offset.current / 1000;
    for (const c of cues) {
      const start = c.start_ms / 1000 - offsetS;
      const end = c.end_ms / 1000 - offsetS;
      if (end > 0) track.addCue(new VTTCue(Math.max(0, start), end, c.text));
    }
    return () => {
      for (const c of Array.from(track.cues ?? [])) track.removeCue(c);
      track.mode = 'disabled';
    };
  }, [pip, cues, planState]);

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
            const t = lastSubtitle.current ?? plan.current?.subtitle_tracks.find((x) => !x.image);
            if (t) void selectSubtitle(t, true);
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
    document.body.style.overflow = 'hidden';
    poke();
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
      window.clearTimeout(hideTimer.current);
    };
  }, [onClose, poke, togglePlay, seekTo, toggleFullscreen, absMS, durationMS, menuOpen, subtitle, selectSubtitle, searching, next, onNext, onPrevious]);

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
    setSkippedNote(skippable.kind === 'intro' ? 'Skipped the intro' : 'Skipped the recap');
    const t = window.setTimeout(() => setSkippedNote(null), 2500);
    return () => window.clearTimeout(t);
  }, [skippable, prefs.data?.auto_skip, paused, seekTo]);
  useEffect(() => {
    // When the credits start, Up Next counts down (as Plex does); Cancel lets them play.
    if (inCredits && next && !creditsShown.current && upNextIn === null) {
      creditsShown.current = true;
      setUpNextIn(10);
    }
  }, [inCredits, next, upNextIn]);

  const showChrome = chromeVisible || paused || menuOpen || !!error || stopped !== null;
  const dur = durationMS;

  return (
    <motion.div
      className={cn('fixed inset-0 z-[100] flex items-center justify-center bg-black', !showChrome && 'cursor-none')}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.25 }}
      onMouseMove={poke}
    >
      {castFrom !== null && item.itemId ? (
        <CastSession
          itemId={item.itemId}
          title={item.title}
          subtitle={item.subtitle}
          startMS={castFrom}
          onEnd={(ms) => {
            setCastFrom(null);
            void load(ms);
          }}
        />
      ) : stopped !== null ? (
        <Message icon={<OctagonX className="text-white/70" />} title="Playback was stopped" onClose={onClose}>
          {stopped}
        </Message>
      ) : error ? (
        <Message icon={<AlertCircle className="text-warning" />} title="Can't play this" onClose={onClose}>
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
            }}
            onError={() => {
              if (installedGeneration.current === loadGeneration.current) void onStreamError();
            }}
            onTimeUpdate={onTimeUpdate}
            onWaiting={() => setLoading(true)}
            onPlaying={() => {
              if (installedGeneration.current !== loadGeneration.current) return;
              requestedPosition.current = null;
              setLoading(false);
              setPaused(false);
              report('playing');
            }}
            onCanPlay={() => {
              if (installedGeneration.current === loadGeneration.current) setLoading(false);
            }}
            onPlay={() => setPaused(false)}
            onPause={() => {
              if (installedGeneration.current !== loadGeneration.current) return;
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
        <div className="text-white">
          <p className="text-lg font-semibold">{item.title}</p>
          {item.subtitle && <p className="text-sm text-white/60">{item.subtitle}</p>}
        </div>
        <Button variant="glass" size="icon" className="pointer-events-auto" aria-label="Close player" onClick={onClose}>
          <X />
        </Button>
      </div>

      {skippable && !skippedNote && (
        <Button variant="primary" size="lg" className="absolute bottom-28 right-6 z-10 shadow-2xl shadow-black/50" onClick={() => seekTo(skippable.end_ms)}>
          {skippable.kind === 'intro' ? 'Skip Intro' : 'Skip Recap'} <SkipForward />
        </Button>
      )}
      {inCredits && upNextIn === null && credits && (
        <Button variant="primary" size="lg" className="absolute bottom-28 right-6 z-10 shadow-2xl shadow-black/50" onClick={() => (next ? onNext() : seekTo(credits.end_ms))}>
          {next ? 'Play Next' : 'Skip Credits'} <SkipForward />
        </Button>
      )}
      {skippedNote && (
        <div className="pointer-events-none absolute bottom-28 right-6 z-10 rounded-full bg-black/70 px-4 py-2 text-sm text-white ring-1 ring-white/15">{skippedNote}</div>
      )}
      {item.preroll && next && (
        <Button variant="glass" className="absolute bottom-28 right-6 z-10" onClick={onNext}>
          Skip <SkipForward />
        </Button>
      )}
      {upNextIn !== null && next && (
        <UpNext item={next} seconds={upNextIn} onTick={() => setUpNextIn((n) => (n === null ? null : n - 1))} onPlay={onNext} onCancel={() => setUpNextIn(null)} />
      )}
      {searching && planState && item.itemId && (
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
      {stopped === null && !error && castFrom === null && (
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
          <div className="mt-3 flex items-center gap-1">
            <ControlButton label={paused ? 'Play (Space)' : 'Pause (Space)'} onClick={togglePlay}>
              {paused ? <Play className="fill-current" /> : <Pause className="fill-current" />}
            </ControlButton>
            {onPrevious && (
              <ControlButton label="Previous (Shift+P)" onClick={onPrevious}>
                <SkipBack className="fill-current" />
              </ControlButton>
            )}
            {next && (
              <ControlButton label={`Next: ${next.title} (Shift+N)`} onClick={onNext}>
                <SkipForward className="fill-current" />
              </ControlButton>
            )}
            <ControlButton label="Back 10 seconds (J)" onClick={() => seekTo(absMS() - 10_000)}>
              <RotateCcw />
            </ControlButton>
            <ControlButton label="Forward 10 seconds (L)" onClick={() => seekTo(absMS() + 10_000)}>
              <RotateCw />
            </ControlButton>
            <VolumeControl volume={volume} muted={muted} onVolume={(v) => (setVolume(v), setMuted(false))} onMute={() => setMuted((m) => !m)} />
            <span className="ml-2 text-[13px] tabular-nums text-white/80">
              {fmt.clock(positionMS)} <span className="text-white/40">/ {dur ? fmt.clock(dur) : '–:––'}</span>
            </span>
            <div className="flex-1" />
            {planState && planState.audio_tracks.length > 1 && (
              <PlayerMenu icon={<Languages />} label="Audio" onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>Audio</DropdownMenuLabel>
                {planState.audio_tracks.map((t, i) => {
                  const { name, detail } = audioLabel(t, i);
                  return (
                    <DropdownMenuItem
                      key={t.index}
                      onSelect={() => {
                        if (t.index === planState.audio_stream) return;
                        void load(absMS(), t.index);
                        // Remembered for the profile, like subtitles: played by default from now on.
                        if (t.language && t.language !== prefs.data?.audio_language) updatePrefs.mutate({ audio_language: t.language });
                      }}
                    >
                      <span className="w-4">{t.index === planState.audio_stream && <Check className="size-4 text-accent" />}</span>
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
              <PlayerMenu icon={<Captions />} label="Subtitles (C)" onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>Subtitles</DropdownMenuLabel>
                <DropdownMenuItem onSelect={() => void selectSubtitle(null, true)}>
                  <span className="w-4">{!subtitle && <Check className="size-4 text-accent" />}</span>
                  Off
                </DropdownMenuItem>
                {planState.subtitle_tracks.map((t, i) => {
                  const { name, detail } = subtitleLabel(t, i);
                  return (
                    <DropdownMenuItem key={t.id} onSelect={() => t.id !== subtitle?.id && void selectSubtitle(t, true)}>
                      <span className="w-4">{t.id === subtitle?.id && <Check className="size-4 text-accent" />}</span>
                      <span className="min-w-0">
                        <span className="block">{name}</span>
                        {detail && <span className="block text-xs text-subtle">{detail}</span>}
                      </span>
                    </DropdownMenuItem>
                  );
                })}
                {subtitleError && <p className="max-w-64 px-2.5 py-1.5 text-xs text-warning">{subtitleError}</p>}
                {item.itemId && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      onSelect={() => {
                        video.current?.pause();
                        setSearching(true);
                      }}
                    >
                      <span className="w-4" />
                      Find Online…
                    </DropdownMenuItem>
                  </>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuLabel>Size</DropdownMenuLabel>
                {(['small', 'medium', 'large', 'huge'] as const).map((sz) => (
                  <DropdownMenuItem
                    key={sz}
                    onSelect={(e) => {
                      e.preventDefault();
                      setSubtitleSize(sz);
                    }}
                  >
                    <span className="w-4">{sz === subtitleSize && <Check className="size-4 text-accent" />}</span>
                    {sz[0].toUpperCase() + sz.slice(1)}
                  </DropdownMenuItem>
                ))}
              </PlayerMenu>
            )}
            {planState && planState.versions.length > 1 && (
              <PlayerMenu icon={<Film />} label="Version" onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>Version</DropdownMenuLabel>
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
              <PlayerMenu icon={<SlidersHorizontal />} label="Quality" onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>Quality</DropdownMenuLabel>
                {[0, ...qualities.filter((q) => !planState.source_kbps || q < planState.source_kbps)].map((q) => (
                  <DropdownMenuItem
                    key={q}
                    onSelect={() => {
                      if (q === quality.current) return;
                      quality.current = q;
                      setQualityState(q);
                      saveQuality(q);
                      void load(absMS(), planState.audio_stream);
                    }}
                  >
                    <span className="w-4">{q === qualityState && <Check className="size-4 text-accent" />}</span>
                    {q ? qualityName(q) : 'Original'}
                  </DropdownMenuItem>
                ))}
              </PlayerMenu>
            )}
            {planState && planState.chapters.length > 1 && (
              <PlayerMenu icon={<ListOrdered />} label="Chapters" onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>Chapters</DropdownMenuLabel>
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
            <PlayerMenu icon={<Gauge />} label="Speed" onOpenChange={setMenuOpen}>
              <DropdownMenuLabel>Speed</DropdownMenuLabel>
              {speeds.map((s) => (
                <DropdownMenuItem key={s} onSelect={() => setRate(s)}>
                  <span className="w-4">{s === rate && <Check className="size-4 text-accent" />}</span>
                  {s === 1 ? 'Normal' : `${s}×`}
                </DropdownMenuItem>
              ))}
            </PlayerMenu>
            {planState && (
              <PlayerMenu icon={<Info />} label="Playback info" onOpenChange={setMenuOpen}>
                <DropdownMenuLabel>{methodLabel(planState).title}</DropdownMenuLabel>
                <p className="max-w-64 px-2.5 pb-2 text-xs leading-relaxed text-muted">{methodLabel(planState).detail}</p>
                <DropdownMenuSeparator />
                <p className="max-w-64 px-2.5 py-1.5 text-xs leading-relaxed text-subtle">
                  Space play/pause · ←/→ 5s · J/L 10s · Shift+←/→ chapters · ↑/↓ volume · M mute · C subtitles · Shift+N/P next/previous · F full screen · I picture-in-picture · &lt;/&gt; speed · 0–9 jump
                </p>
              </PlayerMenu>
            )}
            {castAvailable && item.itemId && (
              <ControlButton label="Cast to a Chromecast" onClick={() => void startCast()}>
                <Cast />
              </ControlButton>
            )}
            {airplayAvailable && item.itemId && (
              <ControlButton
                label="AirPlay"
                onClick={() => (video.current as HTMLVideoElement & { webkitShowPlaybackTargetPicker?: () => void } | null)?.webkitShowPlaybackTargetPicker?.()}
              >
                <Airplay />
              </ControlButton>
            )}
            {pipSupported && (
              <ControlButton label={pip ? 'Back from Picture-in-Picture (I)' : 'Picture-in-Picture (I)'} onClick={togglePip}>
                <PictureInPicture2 />
              </ControlButton>
            )}
            <ControlButton label={fullscreen ? 'Exit full screen (F)' : 'Full screen (F)'} onClick={toggleFullscreen}>
              {fullscreen ? <Minimize /> : <Maximize />}
            </ControlButton>
          </div>
        </div>
      )}
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
function UpNext({ item, seconds, onTick, onPlay, onCancel }: { item: NowPlaying; seconds: number; onTick: () => void; onPlay: () => void; onCancel: () => void }) {
  // The callbacks change with every render of the player; the countdown only follows the seconds.
  const callbacks = useRef({ onTick, onPlay });
  callbacks.current = { onTick, onPlay };
  useEffect(() => {
    if (seconds <= 0) {
      callbacks.current.onPlay();
      return;
    }
    const t = window.setTimeout(() => callbacks.current.onTick(), 1000);
    return () => window.clearTimeout(t);
  }, [seconds]);
  return (
    <div className="absolute bottom-28 right-6 z-10 w-80 rounded-2xl bg-black/80 p-4 text-white shadow-2xl ring-1 ring-white/15 backdrop-blur-xl">
      <p className="text-xs font-semibold uppercase tracking-wider text-white/50">Up next in {seconds}</p>
      <p className="mt-1 truncate font-semibold">{item.title}</p>
      {item.subtitle && <p className="truncate text-sm text-white/60">{item.subtitle}</p>}
      <div className="mt-3 flex gap-2">
        <Button variant="primary" size="sm" onClick={onPlay}>
          <Play className="fill-current" /> Play Now
        </Button>
        <Button variant="glass" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  );
}

function Message({ icon, title, children, onClose }: { icon: ReactNode; title: string; children: ReactNode; onClose: () => void }) {
  return (
    <div className="max-w-md px-6 text-center text-white">
      <div className="mx-auto mb-4 flex justify-center [&_svg]:size-10 [&_svg]:stroke-[1.5]">{icon}</div>
      <h2 className="text-xl font-semibold">{title}</h2>
      <p className="mt-2 text-sm text-white/60">{children}</p>
      <Button variant="glass" className="mt-6" onClick={onClose}>
        Close
      </Button>
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
      className="flex size-10 items-center justify-center rounded-full text-white/90 outline-none transition-colors hover:bg-white/10 hover:text-white focus-visible:ring-2 focus-visible:ring-white/60 [&_svg]:size-5"
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
          className="flex size-10 items-center justify-center rounded-full text-white/90 outline-none transition-colors hover:bg-white/10 hover:text-white focus-visible:ring-2 focus-visible:ring-white/60 data-[state=open]:bg-white/15 [&_svg]:size-5"
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
  const level = muted ? 0 : volume;
  return (
    <div className="group/vol flex items-center">
      <ControlButton label={muted ? 'Unmute (M)' : 'Mute (M)'} onClick={onMute}>
        {level === 0 ? <VolumeX /> : level < 0.5 ? <Volume1 /> : <Volume2 />}
      </ControlButton>
      <input
        type="range"
        min={0}
        max={1}
        step={0.05}
        value={level}
        aria-label="Volume"
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
      aria-label="Position"
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
  const t = chapters[i]?.title?.trim();
  return t && !/^(chapter\s*)?[\d:.]+$/i.test(t) ? t : `Chapter ${i + 1}`;
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
