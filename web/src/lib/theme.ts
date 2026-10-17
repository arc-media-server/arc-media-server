import { useEffect } from 'react';
import type { Theme } from '@/lib/api';
import { usePrefs } from '@/lib/home';

const storageKey = 'arc-theme';

/** Shows the app in a theme ("" follows the device), and remembers it for the next page load. */
export function applyTheme(theme: Theme) {
  if (theme) document.documentElement.dataset.theme = theme;
  else delete document.documentElement.dataset.theme;
  try {
    if (theme) localStorage.setItem(storageKey, theme);
    else localStorage.removeItem(storageKey);
  } catch {
    // storage may be unavailable (private mode); the theme still applies for now
  }
}

/** The theme last used in this browser, applied before the app draws so it doesn't flash. */
export function restoreTheme() {
  try {
    const t = localStorage.getItem(storageKey);
    if (t === 'dark' || t === 'light') document.documentElement.dataset.theme = t;
  } catch {
    // ignore
  }
}

/** Keeps the page in the signed-in profile's theme. */
export function useProfileTheme() {
  const { data } = usePrefs();
  const theme = data?.theme;
  useEffect(() => {
    if (theme !== undefined) applyTheme(theme);
  }, [theme]);
}
