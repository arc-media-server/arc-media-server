import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { AlertCircle, X } from 'lucide-react';
import { Button } from './ui/button';

interface NowPlaying {
  title: string;
  subtitle?: string;
  src: string;
}

const PlayerContext = createContext<(item: NowPlaying) => void>(() => {});

/** Opens the full-screen player from anywhere in the app. */
export function usePlayer() {
  return useContext(PlayerContext);
}

export function PlayerProvider({ children }: { children: ReactNode }) {
  const [current, setCurrent] = useState<NowPlaying | null>(null);
  const play = useCallback((item: NowPlaying) => setCurrent(item), []);
  return (
    <PlayerContext.Provider value={play}>
      {children}
      <AnimatePresence>{current && <PlayerOverlay item={current} onClose={() => setCurrent(null)} />}</AnimatePresence>
    </PlayerContext.Provider>
  );
}

function PlayerOverlay({ item, onClose }: { item: NowPlaying; onClose: () => void }) {
  const video = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState(false);
  const [chromeVisible, setChromeVisible] = useState(true);
  const hideTimer = useRef<number>(undefined);

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
      {error ? (
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
          onError={() => setError(true)}
        />
      )}

      <div
        className={`pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between bg-gradient-to-b from-black/70 to-transparent p-6 transition-opacity duration-500 ${chromeVisible || error ? 'opacity-100' : 'opacity-0'}`}
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
