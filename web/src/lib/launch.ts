// Opening Arc afresh starts at "Who's watching?", unless this browser signs in automatically as
// one profile. A reload, or another Arc tab opened while one is still open, isn't a fresh start.
const autoKey = 'arc-auto-profile';
const aliveKey = 'arc-alive';
const aliveFor = 3 * 60_000; // background tabs may only tick about once a minute

function read(key: string) {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // storage may be unavailable (private mode): every start is then fresh, which is safe
  }
}

/** The profile this browser signs in as automatically, or "" for "Who's watching?". */
export function autoProfile() {
  return read(autoKey) ?? '';
}

export function setAutoProfile(id: string) {
  write(autoKey, id || null);
}

let fresh: boolean | null = null;

/** Whether this page load opened Arc afresh (decided once per load), and keeps the mark up to date. */
export function freshStart() {
  if (fresh === null) {
    const last = Number(read(aliveKey) ?? 0);
    fresh = !last || Date.now() - last > aliveFor;
    const touch = () => write(aliveKey, String(Date.now()));
    touch();
    window.setInterval(touch, 30_000);
    document.addEventListener('visibilitychange', touch);
    window.addEventListener('pagehide', touch);
  }
  return fresh;
}
