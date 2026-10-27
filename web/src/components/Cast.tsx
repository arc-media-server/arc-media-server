import { useCallback, useEffect, useRef, useState } from 'react';
import { Cast as CastIcon, Loader2, Pause, Play, RotateCcw, RotateCw } from 'lucide-react';
import { api, type PlayerCaps } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Button } from './ui/button';

// Casting to a Chromecast with Google's Cast SDK (Chrome and Edge, on https or this computer), as
// Plex Web does. The device plays a cast link to the stream (see internal/api/cast.go); the page
// stays the remote control and saves progress. AirPlay is handled by the player itself (Safari).

/* eslint-disable @typescript-eslint/no-explicit-any */
declare global {
  interface Window {
    __onGCastApiAvailable?: (ok: boolean) => void;
    cast?: any;
    chrome?: any;
  }
}

/**
 * What a Chromecast plays. Google's receiver can't report its own abilities, so Arc first offers
 * what newer devices (Chromecast with Google TV, Ultra, TVs with it built in) play: the original
 * quality, up to 4K, HEVC and Dolby/DTS-free surround. If the device can't play that, Arc tries
 * again with what every Chromecast plays (H.264 or VP9 up to 1080p) and remembers that for the
 * device.
 */
const castCaps: Record<CastQuality, PlayerCaps> = {
  original: { containers: ['mp4', 'webm'], video: ['h264', 'hevc', 'vp9', 'av1'], audio: ['aac', 'mp3', 'opus', 'vorbis', 'flac', 'ac3', 'eac3'], max_height: 0 },
  basic: { containers: ['mp4', 'webm'], video: ['h264', 'vp9'], audio: ['aac', 'mp3', 'opus', 'vorbis', 'flac'], max_height: 1080 },
};
type CastQuality = 'original' | 'basic';

/** What worked on a device before, by its name (this browser only). */
function rememberedQuality(device: string): CastQuality | null {
  try {
    const q = localStorage.getItem('arc-cast-quality:' + device);
    return q === 'original' || q === 'basic' ? q : null;
  } catch {
    return null;
  }
}
function rememberQuality(device: string, q: CastQuality) {
  try {
    localStorage.setItem('arc-cast-quality:' + device, q);
  } catch {
    /* only a convenience */
  }
}

/** A text subtitle track the device shows itself (WebVTT through a cast link). */
export interface CastText {
  id: string;
  language?: string;
  name: string;
  /** The viewer's timing adjustment (positive is later). */
  shiftMS: number;
}

let sdk: Promise<boolean> | undefined;

/** Loads the Cast SDK once, in browsers that have it. */
function loadCastSDK(): Promise<boolean> {
  sdk ??= new Promise((resolve) => {
    // Chrome and Edge only, and they only allow it on secure pages (https or localhost).
    if (!window.chrome || !window.isSecureContext) return resolve(false);
    window.__onGCastApiAvailable = (ok) => {
      if (ok) {
        window.cast.framework.CastContext.getInstance().setOptions({
          receiverApplicationId: window.chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID,
          autoJoinPolicy: window.chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED,
        });
      }
      resolve(ok);
    };
    const s = document.createElement('script');
    s.src = 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1';
    s.onerror = () => resolve(false);
    document.head.appendChild(s);
  });
  return sdk;
}

/** Whether there's a Chromecast to cast to. */
export function useCastAvailable() {
  const [available, setAvailable] = useState(false);
  useEffect(() => {
    let ctx: any;
    const onState = (e: { castState: string }) => setAvailable(e.castState !== 'NO_DEVICES_AVAILABLE');
    void loadCastSDK().then((ok) => {
      if (!ok) return;
      ctx = window.cast.framework.CastContext.getInstance();
      setAvailable(ctx.getCastState() !== 'NO_DEVICES_AVAILABLE');
      ctx.addEventListener(window.cast.framework.CastContextEventType.CAST_STATE_CHANGED, onState);
    });
    return () => {
      ctx?.removeEventListener(window.cast.framework.CastContextEventType.CAST_STATE_CHANGED, onState);
    };
  }, []);
  return available;
}

/** Shows the Chromecast picker; resolves once one is chosen (false if the picker was closed). */
export async function chooseChromecast(): Promise<boolean> {
  if (!(await loadCastSDK())) return false;
  try {
    await window.cast.framework.CastContext.getInstance().requestSession();
    return true;
  } catch {
    return false;
  }
}

/**
 * Playing on a Chromecast: starts the title there at a point, then is its remote control. onEnd
 * gets where it got to when casting stops, to carry on in the browser.
 */
export function CastSession({
  itemId,
  title,
  subtitle,
  startMS,
  fileId,
  audioStream,
  burnSubtitle,
  text,
  onEnd,
}: {
  itemId: string;
  title: string;
  subtitle?: string;
  startMS: number;
  fileId: number;
  audioStream: number;
  burnSubtitle?: string;
  text?: CastText;
  onEnd: (positionMS: number) => void;
}) {
  const [device, setDevice] = useState('');
  const [positionMS, setPositionMS] = useState(startMS);
  const [durationMS, setDurationMS] = useState(0);
  const [paused, setPaused] = useState(false);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [fellBack, setFellBack] = useState(false);
  // Which quality this cast is using, and whether playback got going at it.
  const quality = useRef<CastQuality>('original');
  const startedAt = useRef<number | null>(null);
  const played = useRef(false);
  // Converted streams start at the cast point: the device's clock plus this is the file's time.
  const offset = useRef(0);
  const direct = useRef(true);
  const session = useRef<string | undefined>(undefined);
  const player = useRef<any>(null);
  const controller = useRef<any>(null);
  const pos = useRef(startMS);
  const duration = useRef(0);
  const ended = useRef(false);
  const generation = useRef(0);

  // Start (or restart, to seek in a converted stream) the title on the device.
  const start = useCallback(
    async (atMS: number, fallback = false): Promise<void> => {
      const run = ++generation.current;
      setBusy(true);
      setError(null);
      let ownedSession: string | undefined;
      try {
        if (session.current) {
          const old = session.current;
          session.current = undefined;
          await api.stopTranscode(old).catch(() => {});
        }
        if (run !== generation.current) return;
        const castSession = window.cast.framework.CastContext.getInstance().getCurrentSession();
        const name = castSession?.getCastDevice()?.friendlyName ?? 'Chromecast';
        quality.current = fallback ? 'basic' : rememberedQuality(name) ?? 'original';
        startedAt.current = null;
        played.current = false;
        const plan = await api.startPlayback(itemId, { caps: castCaps[quality.current], start_ms: Math.round(atMS), file_id: fileId, audio_stream: audioStream, burn_subtitle: burnSubtitle });
        ownedSession = plan.session_id;
        if (run !== generation.current) {
          if (ownedSession) await api.stopTranscode(ownedSession).catch(() => {});
          return;
        }
        const { url, local_only } = await api.castLink(plan.url);
        // The device shows text subtitles itself, timed to the stream it plays.
        const vtt = text
          ? (await api.castLink(`/api/items/${itemId}/subtitles/${text.id}/vtt?file=${plan.file_id}&shift_ms=${Math.round(text.shiftMS - (plan.hls ? plan.offset_ms : 0))}`)).url
          : undefined;
        if (run !== generation.current) {
          if (ownedSession) await api.stopTranscode(ownedSession).catch(() => {});
          return;
        }
        if (local_only) throw new Error('Only this computer can reach Arc. Turn on Allow other devices in Settings > Network so the Chromecast can too.');
        offset.current = plan.hls ? plan.offset_ms : 0;
        direct.current = !plan.hls;
        session.current = plan.session_id;
        setDurationMS(plan.duration_ms);
        duration.current = plan.duration_ms;
        const { chrome, cast } = window;
        const s = cast.framework.CastContext.getInstance().getCurrentSession();
        if (!s) throw new Error('The Chromecast disconnected');
        setDevice(s.getCastDevice()?.friendlyName ?? 'Chromecast');
        const info = new chrome.cast.media.MediaInfo(url, plan.hls ? 'application/x-mpegurl' : 'video/mp4');
        if (plan.hls) {
          info.hlsSegmentFormat = chrome.cast.media.HlsSegmentFormat.FMP4;
          info.hlsVideoSegmentFormat = chrome.cast.media.HlsVideoSegmentFormat.FMP4;
        }
        info.streamType = chrome.cast.media.StreamType.BUFFERED;
        info.metadata = new chrome.cast.media.GenericMediaMetadata();
        info.metadata.title = title;
        info.metadata.subtitle = subtitle;
        if (vtt && text) {
          const track = new chrome.cast.media.Track(1, chrome.cast.media.TrackType.TEXT);
          track.trackContentId = vtt;
          track.trackContentType = 'text/vtt';
          track.subtype = chrome.cast.media.TextTrackType.SUBTITLES;
          track.name = text.name;
          if (text.language) track.language = text.language;
          info.tracks = [track];
          const style = new chrome.cast.media.TextTrackStyle();
          style.foregroundColor = '#FFFFFFFF';
          style.backgroundColor = '#00000000';
          style.edgeType = chrome.cast.media.TextTrackEdgeType.DROP_SHADOW;
          style.edgeColor = '#000000CC';
          info.textTrackStyle = style;
        }
        const req = new chrome.cast.media.LoadRequest(info);
        if (vtt) req.activeTrackIds = [1];
        req.autoplay = true;
        // A copied stream may begin at an earlier keyframe, just as in the browser.
        req.currentTime = plan.hls ? Math.max(0, (atMS - plan.offset_ms) / 1000) : atMS / 1000;
        try {
          await s.loadMedia(req);
        } catch (e) {
          const code = (e as { code?: string })?.code;
          throw Object.assign(new Error(code ? `The Chromecast couldn't play it (${code})` : e instanceof Error ? e.message : String(e)), { loadFailed: true });
        }
        if (run !== generation.current) return;
        setBusy(false);
        // If the device stops with an error before playing at the original quality, it can't
        // decode it: fall back as for a failed load.
        const media = s.getMediaSession?.();
        media?.addUpdateListener?.(() => {
          if (run !== generation.current || quality.current !== 'original' || played.current) return;
          if (media.playerState === 'IDLE' && media.idleReason === 'ERROR') {
            rememberQuality(name, 'basic');
            setFellBack(true);
            void start(pos.current, true);
          }
        });
      } catch (e) {
        if (ownedSession) await api.stopTranscode(ownedSession).catch(() => {});
        if (session.current === ownedSession) session.current = undefined;
        if (run !== generation.current) return;
        // The device refused the original quality: try what every Chromecast plays.
        if ((e as { loadFailed?: boolean }).loadFailed && quality.current === 'original') {
          const name = window.cast.framework.CastContext.getInstance().getCurrentSession()?.getCastDevice()?.friendlyName ?? 'Chromecast';
          rememberQuality(name, 'basic');
          setFellBack(true);
          return start(atMS, true);
        }
        setBusy(false);
        setError(e instanceof Error ? e.message : typeof e === 'object' && e && 'code' in e ? `The Chromecast couldn't play it (${String((e as { code: string }).code)})` : String(e));
      }
    },
    [itemId, title, subtitle, fileId, audioStream, burnSubtitle, text],
  );

  // Follow the device: where it is, paused or not, and whether it's still connected.
  useEffect(() => {
    const { cast } = window;
    const p = new cast.framework.RemotePlayer();
    const c = new cast.framework.RemotePlayerController(p);
    player.current = p;
    controller.current = c;
    const T = cast.framework.RemotePlayerEventType;
    const onTime = () => {
      if (!p.isMediaLoaded) return;
      pos.current = offset.current + p.currentTime * 1000;
      setPositionMS(pos.current);
      // A few seconds played at the original quality: the device handles it.
      const began = (startedAt.current ??= p.currentTime);
      if (p.currentTime - began >= 1) played.current = true;
      if (quality.current === 'original' && p.currentTime - began >= 5) {
        const name = window.cast.framework.CastContext.getInstance().getCurrentSession()?.getCastDevice()?.friendlyName;
        if (name && rememberedQuality(name) !== 'original') rememberQuality(name, 'original');
      }
    };
    const onPaused = () => setPaused(p.isPaused);
    const onConnected = () => {
      if (!p.isConnected && !ended.current) {
        ended.current = true;
        ++generation.current;
        const owned = session.current;
        session.current = undefined;
        void (async () => {
          if (owned) await api.stopTranscode(owned).catch(() => {});
          onEnd(pos.current);
        })();
      }
    };
    c.addEventListener(T.CURRENT_TIME_CHANGED, onTime);
    c.addEventListener(T.IS_PAUSED_CHANGED, onPaused);
    c.addEventListener(T.IS_CONNECTED_CHANGED, onConnected);
    void start(startMS);
    return () => {
      c.removeEventListener(T.CURRENT_TIME_CHANGED, onTime);
      c.removeEventListener(T.IS_PAUSED_CHANGED, onPaused);
      c.removeEventListener(T.IS_CONNECTED_CHANGED, onConnected);
    };
    // Runs once per cast.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Closing the player stops casting too.
  useEffect(
    () => () => {
      ++generation.current;
      if (session.current) void api.stopTranscode(session.current).catch(() => {});
      if (ended.current) return;
      ended.current = true;
      void api.reportProgress(itemId, pos.current, duration.current, 'stopped').catch(() => {});
      window.cast?.framework.CastContext.getInstance().endCurrentSession(true);
    },
    [],
  );

  // Save progress, and keep a converting stream going.
  useEffect(() => {
    const t = window.setInterval(() => {
      if (durationMS && !busy) void api.reportProgress(itemId, pos.current, durationMS, paused ? 'paused' : 'playing').catch(() => {});
      if (session.current) void api.pingTranscode(session.current).catch(() => {});
    }, 10_000);
    return () => window.clearInterval(t);
  }, [itemId, durationMS, paused, busy]);

  const seekTo = (ms: number) => {
    const t = Math.max(0, durationMS ? Math.min(ms, durationMS - 1000) : ms);
    setPositionMS(t);
    pos.current = t;
    if (direct.current && player.current?.isMediaLoaded) {
      player.current.currentTime = t / 1000;
      controller.current.seek();
    } else {
      void start(t); // a converted stream starts again from there
    }
  };

  const stop = async () => {
    if (ended.current) return;
    ++generation.current;
    ended.current = true;
    setBusy(true);
    const owned = session.current;
    session.current = undefined;
    window.cast?.framework.CastContext.getInstance().endCurrentSession(true);
    // Release the receiver's slot before the browser starts a replacement conversion.
    if (owned) await api.stopTranscode(owned).catch(() => {});
    await api.reportProgress(itemId, pos.current, durationMS, 'paused').catch(() => {});
    onEnd(pos.current);
  };

  return (
    <div className="absolute inset-0 flex flex-col items-center justify-center gap-6 bg-black px-6 text-white">
      <CastIcon className="size-14 text-white/60" strokeWidth={1.5} />
      <div className="text-center">
        <p className="text-sm text-white/60">{device ? `Playing on ${device}` : 'Connecting…'}</p>
        {fellBack && <p role="status" className="text-xs text-white/50">{device || 'This Chromecast'} couldn&apos;t play the original, so Arc is converting it.</p>}
        <p className="mt-1 text-2xl font-semibold">{title}</p>
        {subtitle && <p className="text-white/60">{subtitle}</p>}
      </div>
      {error && <div className="max-w-md space-y-2 text-center"><p role="alert" className="text-sm text-warning">{error}</p><Button variant="secondary" onClick={() => void start(pos.current)}>Retry casting</Button></div>}
      <div className="w-full max-w-xl">
        <input
          type="range"
          aria-label="Position"
          min={0}
          max={durationMS || 1}
          value={positionMS}
          disabled={busy}
          onChange={(e) => seekTo(Number(e.target.value))}
          className="w-full accent-[var(--color-accent)]"
        />
        <div className="flex justify-between text-xs tabular-nums text-white/60">
          <span>{fmt.clock(positionMS)}</span>
          <span>{durationMS ? fmt.clock(durationMS) : ''}</span>
        </div>
      </div>
      <div className="flex items-center gap-4">
        <Button variant="glass" size="icon" aria-label="Back 10 seconds" disabled={busy} onClick={() => seekTo(pos.current - 10_000)}>
          <RotateCcw />
        </Button>
        <Button variant="primary" size="lg" className="size-14 rounded-full p-0" aria-label={paused ? 'Play' : 'Pause'} disabled={busy} onClick={() => controller.current?.playOrPause()}>
          {busy ? <Loader2 className="animate-spin" /> : paused ? <Play /> : <Pause />}
        </Button>
        <Button variant="glass" size="icon" aria-label="Forward 10 seconds" disabled={busy} onClick={() => seekTo(pos.current + 10_000)}>
          <RotateCw />
        </Button>
      </div>
      <Button variant="secondary" onClick={stop}>
        Stop Casting
      </Button>
    </div>
  );
}
