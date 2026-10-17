import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { AnimatePresence, motion } from 'motion/react';
import { AlertCircle, OctagonX, X } from 'lucide-react';
import { api, ApiError } from '@/lib/api';
import { Button } from './ui/button';

interface NowPlaying {
  title: string;
  subtitle?: string;
  src: string;
  /** The movie or episode, so progress is saved for the current profile. */
  itemId?: string;
  /** Where to start, in milliseconds (a resume point). */
  startMS?: number;
}

const PlayerContext = createContext<(item: NowPlaying) => void>(() => {});

/** Opens the full-screen player from anywhere in the app. */
export function usePlayer() {
  return useContext(PlayerContext);
}

// How often progress is saved while playing.
const reportEveryMS = 10_000;

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

function PlayerOverlay({ item, onClose }: { item: NowPlaying; onClose: () => void }) {
  const video = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState(false);
  // Why the server's admin stopped playback, if they did.
  const [stopped, setStopped] = useState<string | null>(null);
  const [chromeVisible, setChromeVisible] = useState(true);
  const hideTimer = useRef<number>(undefined);
  const lastReport = useRef(0);

  const report = useCallback(
    (state: 'playing' | 'paused' | 'stopped' = 'playing') => {
      const v = video.current;
      if (!item.itemId || !v || !v.duration || Number.isNaN(v.duration)) return;
      lastReport.current = Date.now();
      api.reportProgress(item.itemId, v.currentTime * 1000, v.duration * 1000, state).catch((e) => {
        if (e instanceof ApiError && e.needs === 'stopped') setStopped(e.message);
      });
    },
    [item.itemId],
  );

  // Save progress when the player closes (including the window closing).
  useEffect(() => {
    const onHide = () => report('stopped');
    window.addEventListener('pagehide', onHide);
    return () => {
      window.removeEventListener('pagehide', onHide);
      onHide();
    };
  }, [report]);

  // A file that stops loading may have been stopped by the admin rather than be unplayable.
  const onVideoError = useCallback(async () => {
    try {
      const res = await fetch(item.src, { headers: { Range: 'bytes=0-0' } });
      if (res.status === 403) {
        const body = await res.json().catch(() => ({}));
        setStopped(body.error ?? "The server's admin stopped this.");
        return;
      }
    } catch {
      // fall through to the generic message
    }
    setError(true);
  }, [item.src]);

  const poke = useCallback(() => {
    setChromeVisible(true);
    window.clearTimeout(hideTimer.current);
    hideTimer.current = window.setTimeout(() => setChromeVisible(false), 2500);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.fullscreenElement) onClose();
    };
    window.addEventListener('keydown', onKey);
    document.body.style.overflow = 'hidden';
    poke();
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
      window.clearTimeout(hideTimer.current);
    };
  }, [onClose, poke]);

  return (
    <motion.div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.25 }}
      onMouseMove={poke}
    >
      {stopped !== null ? (
        <div className="max-w-md px-6 text-center text-white">
          <OctagonX className="mx-auto mb-4 size-10 text-white/70" strokeWidth={1.5} />
          <h2 className="text-xl font-semibold">Playback was stopped</h2>
          <p className="mt-2 text-sm text-white/60">{stopped}</p>
          <Button variant="glass" className="mt-6" onClick={onClose}>
            Close
          </Button>
        </div>
      ) : error ? (
        <div className="max-w-md px-6 text-center text-white">
          <AlertCircle className="mx-auto mb-4 size-10 text-warning" strokeWidth={1.5} />
          <h2 className="text-xl font-semibold">Can't play this in the browser yet</h2>
          <p className="mt-2 text-sm text-white/60">
            This file's format isn't supported by your browser. AMS will convert it automatically once
            transcoding arrives.
          </p>
          <Button variant="glass" className="mt-6" onClick={onClose}>
            Close
          </Button>
        </div>
      ) : (
        <video
          ref={video}
          src={item.src}
          autoPlay
          controls
          playsInline
          className="max-h-full max-w-full"
          onError={onVideoError}
          onLoadedMetadata={(e) => {
            if (item.startMS) e.currentTarget.currentTime = item.startMS / 1000;
          }}
          onTimeUpdate={() => {
            if (Date.now() - lastReport.current >= reportEveryMS) report();
          }}
          onPlay={() => report('playing')}
          onPause={() => report('paused')}
          onEnded={() => report('paused')}
        />
      )}

      <div
        className={`pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between bg-gradient-to-b from-black/70 to-transparent p-6 transition-opacity duration-500 ${chromeVisible || error || stopped !== null ? 'opacity-100' : 'opacity-0'}`}
      >
        <div className="text-white">
          <p className="text-lg font-semibold">{item.title}</p>
          {item.subtitle && <p className="text-sm text-white/60">{item.subtitle}</p>}
        </div>
        <Button variant="glass" size="icon" className="pointer-events-auto" aria-label="Close player" onClick={onClose}>
          <X />
        </Button>
      </div>
    </motion.div>
  );
}
