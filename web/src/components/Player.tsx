import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { AnimatePresence, motion } from 'motion/react';
import type HlsType from 'hls.js';
import {
  AlertCircle,
  Check,
  Captions,
  Film,
  Gauge,
  Info,
  Languages,
  Loader2,
  Maximize,
  Minimize,
  OctagonX,
  Pause,
  Play,
  RotateCcw,
  RotateCw,
  Volume1,
  Volume2,
  VolumeX,
  X,
} from 'lucide-react';
import { api, ApiError, type PlaybackPlan, type PlayerCaps, type SubtitleCue, type SubtitleTrack, type TrickplayInfo } from '@/lib/api';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import * as fmt from '@/lib/format';
import { audioLabel, chooseSubtitle, fallbackCaps, methodLabel, playerCaps, subtitleLabel, versionLabel } from '@/lib/playback';
import { cn } from '@/lib/utils';
import { Button } from './ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu';

interface NowPlaying {
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
}

const PlayerContext = createContext<(item: NowPlaying) => void>(() => {});

/** Opens the full-screen player from anywhere in the app. */
export function usePlayer() {
  return useContext(PlayerContext);
}

// How often progress is saved while playing, and a converting session is kept alive.
const reportEveryMS = 10_000;
const pingEveryMS = 30_000;
const speeds = [0.5, 0.75, 1, 1.25, 1.5, 2];

export function PlayerProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const [current, setCurrent] = useState<NowPlaying | null>(null);
  const play = useCallback((item: NowPlaying) => setCurrent(item), []);
  const close = useCallback(() => {
    setCurrent(null);
    // Watched state and Continue Watching have changed.
    window.setTimeout(() => {
      for (const key of ['item', 'items', 'seasons', 'continue', 'next-up']) qc.invalidateQueries({ queryKey: [key] });
    }, 300);
  }, [qc]);
  return (
    <PlayerContext.Provider value={play}>
      {children}
      <AnimatePresence>{current && <PlayerOverlay item={current} onClose={close} />}</AnimatePresence>
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

function PlayerOverlay({ item, onClose }: { item: NowPlaying; onClose: () => void }) {
  const video = useRef<HTMLVideoElement>(null);
  const hls = useRef<HlsType | null>(null);
  const plan = useRef<PlaybackPlan | null>(null);
  const [planState, setPlanState] = useState<PlaybackPlan | null>(null);
  // Where the video element's clock starts in the file (converted streams begin at the seek point).
  const offset = useRef(0);
  const pendingSeek = useRef<number | null>(null);
  // How many times this playback has fallen back to a heavier method after failing.
  const fallbacks = useRef(0);
  const watchdog = useRef<number>(undefined);
  // What the current stream was asked for with: seeks and audio changes keep it.
  const caps = useRef<PlayerCaps>(playerCaps());
  // A picture subtitle track being burned into the video; restarts keep it.
  const burn = useRef<string | undefined>(undefined);
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
  const [subtitleSize, setSubtitleSize] = useState(readSubtitleSize);
  const [cueText, setCueText] = useState('');
  const autoChosen = useRef(false);
  // Thumbnails for the timeline, if the library makes them.
  const [thumbs, setThumbs] = useState<{ info: TrickplayInfo; sheets: string[] } | null>(null);
  const lastSubtitle = useRef<SubtitleTrack | null>(null);
  const hideTimer = useRef<number>(undefined);
  const lastReport = useRef(0);

  const absMS = useCallback(() => offset.current + (video.current?.currentTime ?? 0) * 1000, []);

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
      caps.current = withCaps;
      const v = video.current;
      if (!v) return;
      setLoading(true);
      setError(null);
      detach();
      if (!item.itemId) {
        // An extra: play the file directly.
        offset.current = 0;
        plan.current = null;
        pendingSeek.current = startMS / 1000;
        v.src = item.src ?? '';
        v.play().catch(() => {});
        return;
      }
      let p: PlaybackPlan;
      try {
        p = await api.startPlayback(item.itemId, { caps: withCaps, start_ms: Math.round(startMS), audio_stream: audioStream, file_id: plan.current?.file_id, burn_subtitle: burn.current, edition: item.edition });
      } catch (e) {
        setLoading(false);
        if (e instanceof ApiError && e.status === 403) setStopped(e.message);
        else setError(e instanceof Error ? e.message : String(e));
        return;
      }
      plan.current = p;
      setPlanState(p);
      setDurationMS(p.duration_ms);
      offset.current = p.hls ? p.offset_ms : 0;
      pendingSeek.current = p.hls ? null : startMS / 1000;
      setReadyMS(offset.current);
      if (!p.hls) {
        v.src = p.url;
      } else {
        const { default: Hls } = await import('hls.js');
        if (!Hls.isSupported()) {
          // No Media Source Extensions (iPhone): the browser plays HLS itself.
          v.src = p.url;
          v.play().catch(() => {});
          return;
        }
        const h = new Hls({ startPosition: 0, maxBufferLength: 60, backBufferLength: 120 });
        h.on(Hls.Events.ERROR, (_e, data) => {
          if (data.fatal) void onStreamError();
        });
        h.loadSource(p.url);
        h.attachMedia(v);
        hls.current = h;
      }
      v.play().catch(() => {});
      // Some browsers accept a format and then never show a frame: give up on it after a while.
      window.clearTimeout(watchdog.current);
      watchdog.current = window.setTimeout(() => {
        if (video.current && video.current.readyState < 2) void onStreamError();
      }, 15_000);
    },
    // onStreamError is stable through refs; listing it would recreate load on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [item.itemId, item.src, detach],
  );

  // Something went wrong mid-stream: the admin stopped it, the session expired, or the browser
  // couldn't decode what it said it could.
  const onStreamError = useCallback(async () => {
    const p = plan.current;
    const at = absMS();
    const url = p?.url ?? item.src;
    if (url) {
      try {
        const res = await fetch(url, { headers: { Range: 'bytes=0-0' } });
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
    setError(item.itemId ? "Arc couldn't play this file." : "This file can't be played in the browser.");
  }, [absMS, detach, item.itemId, item.src, load]);

  // Show a subtitle track (null for none). Text ones are drawn here; picture ones are burned into
  // the video, which restarts it converted. remember saves the language for next time.
  const selectSubtitle = useCallback(
    async (t: SubtitleTrack | null, remember: boolean) => {
      const p = plan.current;
      if (!p || !item.itemId) return;
      if (remember) updatePrefs.mutate({ subtitle_language: t && !t.forced ? (t.language ?? '') : '' });
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
    const t = chooseSubtitle(planState.subtitle_tracks, prefs.data.subtitle_language, audioLang);
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

  useEffect(() => {
    try {
      localStorage.setItem('arc-subtitle-size', subtitleSize);
    } catch {
      // not remembered
    }
  }, [subtitleSize]);

  // Start playing when the player opens; end the conversion and save progress when it closes.
  useEffect(() => {
    void load(item.startMS ?? 0);
    const onHide = () => report('stopped');
    window.addEventListener('pagehide', onHide);
    return () => {
      window.removeEventListener('pagehide', onHide);
      onHide();
      detach();
      window.clearTimeout(watchdog.current);
      const sid = plan.current?.session_id;
      if (sid) api.stopTranscode(sid).catch(() => {});
    };
    // Runs once per opened item.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
      if (e.target instanceof HTMLInputElement || e.ctrlKey || e.metaKey || e.altKey) return;
      const v = video.current;
      switch (e.key) {
        case 'Escape':
          if (!document.fullscreenElement && !menuOpen) onClose();
          return;
        case ' ':
        case 'k':
          togglePlay();
          break;
        case 'ArrowLeft':
        case 'j':
          seekTo(absMS() - (e.key === 'j' ? 10_000 : 5_000));
          break;
        case 'ArrowRight':
        case 'l':
          seekTo(absMS() + (e.key === 'l' ? 10_000 : 5_000));
          break;
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
        case 'm':
          setMuted((x) => !x);
          break;
        case 'f':
          toggleFullscreen();
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
  }, [onClose, poke, togglePlay, seekTo, toggleFullscreen, absMS, durationMS, menuOpen, subtitle, selectSubtitle]);

  const onTimeUpdate = () => {
    const v = video.current;
    if (!v) return;
    setPositionMS(absMS());
    if (v.buffered.length) setBufferedMS(offset.current + v.buffered.end(v.buffered.length - 1) * 1000);
    if (plan.current?.hls && v.seekable.length) setReadyMS(offset.current + v.seekable.end(v.seekable.length - 1) * 1000);
    if (!v.paused && Date.now() - lastReport.current >= reportEveryMS) report();
  };

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
      {stopped !== null ? (
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
              const v = e.currentTarget;
              if (!plan.current) setDurationMS((v.duration || 0) * 1000);
              if (pendingSeek.current) v.currentTime = pendingSeek.current;
              pendingSeek.current = null;
            }}
            onError={() => void onStreamError()}
            onTimeUpdate={onTimeUpdate}
            onWaiting={() => setLoading(true)}
            onPlaying={() => {
              setLoading(false);
              setPaused(false);
              report('playing');
            }}
            onCanPlay={() => setLoading(false)}
            onPlay={() => setPaused(false)}
            onPause={() => {
              setPaused(true);
              report('paused');
            }}
            onEnded={() => {
              setPaused(true);
              report('paused');
            }}
          />
          {cueText && (
            <div
              className={cn(
                'pointer-events-none absolute inset-x-0 flex flex-col items-center gap-1 px-[8%] text-center transition-[bottom] duration-300',
                showChrome ? 'bottom-36' : 'bottom-[7%]',
              )}
              style={{ fontSize: subtitleSizes[subtitleSize] }}
              data-subtitles
            >
              {cueText.split('\n').map((line, i) => (
                <span key={i} className="rounded-md bg-black/65 px-[0.4em] py-[0.05em] font-medium leading-snug text-white [text-shadow:0_1px_2px_rgb(0_0_0/0.6)]">
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

      {stopped === null && !error && (
        <div
          className={cn(
            'absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 via-black/40 to-transparent px-6 pb-5 pt-16 text-white transition-opacity duration-500',
            showChrome ? 'opacity-100' : 'pointer-events-none opacity-0',
          )}
        >
          <Scrubber positionMS={positionMS} durationMS={dur} bufferedMS={bufferedMS} readyMS={planState?.hls ? readyMS : 0} onSeek={seekTo} thumbs={thumbs} />
          <div className="mt-3 flex items-center gap-1">
            <ControlButton label={paused ? 'Play (Space)' : 'Pause (Space)'} onClick={togglePlay}>
              {paused ? <Play className="fill-current" /> : <Pause className="fill-current" />}
            </ControlButton>
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
                    <DropdownMenuItem key={t.index} onSelect={() => t.index !== planState.audio_stream && load(absMS(), t.index)}>
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
            {planState && planState.subtitle_tracks.length > 0 && (
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
                <DropdownMenuSeparator />
                <DropdownMenuLabel>Size</DropdownMenuLabel>
                {(['small', 'medium', 'large'] as const).map((sz) => (
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
                  Space play/pause · ←/→ 5s · J/L 10s · ↑/↓ volume · M mute · C subtitles · F full screen · &lt;/&gt; speed · 0–9 jump
                </p>
              </PlayerMenu>
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

type SubtitleSize = 'small' | 'medium' | 'large';
const subtitleSizes: Record<SubtitleSize, string> = {
  small: 'clamp(14px, 2.4vh, 28px)',
  medium: 'clamp(16px, 3.2vh, 38px)',
  large: 'clamp(18px, 4.3vh, 52px)',
};

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
}: {
  positionMS: number;
  durationMS: number;
  bufferedMS: number;
  readyMS: number;
  onSeek: (ms: number) => void;
  thumbs: { info: TrickplayInfo; sheets: string[] } | null;
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
          {fmt.clock(drag ?? hover)}
        </div>
      )}
    </div>
  );
}
