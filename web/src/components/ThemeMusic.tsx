import { useEffect } from 'react';
import { usePrefs } from '@/lib/home';

const volume = 0.25;

/**
 * Plays a title's theme tune quietly while its page is open, as Plex does: once, fading in, and
 * fading out when the page closes. It stops when a video starts, and when the profile has turned
 * Theme Music off.
 */
export function useThemeMusic(url?: string) {
  const prefs = usePrefs();
  const on = !!url && prefs.data !== undefined && prefs.data.theme_music !== false;
  useEffect(() => {
    if (!on || !url) return;
    const audio = new Audio(url);
    audio.volume = 0;
    let fade = 0;
    const fadeTo = (target: number, done?: () => void) => {
      window.clearInterval(fade);
      fade = window.setInterval(() => {
        const next = audio.volume + (target > audio.volume ? 0.02 : -0.02);
        if (Math.abs(target - audio.volume) <= 0.02) {
          audio.volume = target;
          window.clearInterval(fade);
          done?.();
        } else {
          audio.volume = Math.min(1, Math.max(0, next));
        }
      }, 60);
    };
    // A video starting (the player, a trailer) stops the tune.
    const onPlay = (e: Event) => {
      if (e.target instanceof HTMLVideoElement) fadeTo(0, () => audio.pause());
    };
    document.addEventListener('play', onPlay, true);
    audio
      .play()
      .then(() => fadeTo(volume))
      .catch(() => {}); // the browser may not allow sound before the first click; that's fine
    return () => {
      document.removeEventListener('play', onPlay, true);
      fadeTo(0, () => {
        audio.pause();
        audio.src = '';
      });
    };
  }, [on, url]);
}
