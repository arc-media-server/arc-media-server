import { useEffect } from 'react';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';

/**
 * Live updates (docs/EVENTS-API.md): Arc sends "something changed" hints on /api/v1/events, and
 * the app refetches what's on screen. Hints name a topic only, never the record, so a hint marks
 * the matching queries stale: visible ones refetch, the rest refetch when next shown.
 *
 * Which queries each topic touches. Unknown topics are ignored (the stream promises new ones may
 * appear). Our own edits already invalidate what they change; their echo costs one cheap refetch.
 */
export const topicQueries: Record<string, string[]> = {
  catalog: ['items', 'item', 'seasons', 'featured', 'continue', 'next-up', 'watchlist', 'recommended', 'collections',
    'collection', 'playlist', 'similar', 'person', 'search', 'facets', 'labels', 'libraries', 'orderings'],
  library: ['libraries', 'items', 'featured', 'recommended', 'facets', 'folders', 'recommended-layout', 'collections'],
  profile: ['prefs', 'continue', 'next-up', 'watchlist', 'items', 'item', 'seasons', 'playlists', 'playlist', 'featured',
    'recommended', 'my-year'],
  playback: ['playing', 'continue', 'next-up', 'item', 'seasons'],
  sessions: ['devices', 'playing'],
  activity: ['activity', 'scheduled-work'],
  artwork: ['artwork', 'item', 'featured'],
  optimized: ['optimized', 'playback-options', 'item'],
  previews: ['item'],
  markers: ['markers', 'marker-coverage'],
  watch_together: ['watch-together-rooms'],
  maintenance: ['scheduled-work', 'maintenance-window', 'health', 'health-summary', 'stats-storage', 'backups'],
};

/** The query-key prefixes a set of topics should refresh; null means everything (a resync). */
export function prefixesFor(topics: Iterable<string>): Set<string> | null {
  const out = new Set<string>();
  for (const topic of topics) {
    if (topic === 'resync') return null;
    for (const key of topicQueries[topic] ?? []) out.add(key);
  }
  return out;
}

/**
 * Collects hints and refreshes at most once every `spacing` ms, the first refresh `first` ms after
 * a quiet spell. A scan sends hints twice a second for minutes; refreshing on each would reload
 * Home constantly, and waiting for the hints to stop would show nothing until the scan ends.
 */
export class HintBatcher {
  private pending = new Set<string>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private last = -Infinity;

  constructor(
    private readonly flush: (topics: Set<string>) => void,
    private readonly first = 1_500,
    private readonly spacing = 5_000,
    private readonly now: () => number = () => Date.now(),
  ) {}

  add(topic: string) {
    this.pending.add(topic);
    if (this.timer !== undefined) return;
    const wait = Math.max(this.first, this.last + this.spacing - this.now());
    this.timer = setTimeout(() => this.run(), wait);
  }

  /** Sends what's waiting now (e.g. when the stream reconnects after a gap). */
  run() {
    if (this.timer !== undefined) clearTimeout(this.timer);
    this.timer = undefined;
    if (this.pending.size === 0) return;
    const topics = this.pending;
    this.pending = new Set();
    this.last = this.now();
    this.flush(topics);
  }

  stop() {
    if (this.timer !== undefined) clearTimeout(this.timer);
    this.timer = undefined;
    this.pending.clear();
  }
}

function refresh(qc: QueryClient, topics: Set<string>) {
  const prefixes = prefixesFor(topics);
  void qc.invalidateQueries({
    // ['auth'] is the sign-in state: the stream itself closes if that changes.
    predicate: (q) => {
      const head = String(q.queryKey[0]);
      return head !== 'auth' && (prefixes === null || prefixes.has(head));
    },
  });
}

/** How long a hidden tab keeps its stream (the server allows 32 at once). */
const HIDDEN_GRACE_MS = 60_000;

/**
 * Keeps one events stream open while the tab is visible and refreshes stale screens from it. The
 * browser's EventSource reconnects by itself and resumes with Last-Event-ID; when the server
 * refuses (429, full) or the tab was away, it's reopened later and everything visible refreshed.
 */
export function useLiveUpdates(profileId: string | undefined) {
  const qc = useQueryClient();
  useEffect(() => {
    if (!profileId || typeof EventSource === 'undefined') return;
    const batcher = new HintBatcher((topics) => refresh(qc, topics));
    let source: EventSource | undefined;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let hiddenTimer: ReturnType<typeof setTimeout> | undefined;
    let backoff = 5_000;
    let missed = false;

    const open = () => {
      if (source) return;
      const es = new EventSource('/api/v1/events', { withCredentials: true });
      source = es;
      es.addEventListener('open', () => {
        backoff = 5_000;
        // Anything could have changed while there was no stream.
        if (missed) {
          missed = false;
          batcher.add('resync');
          batcher.run();
        }
      });
      es.addEventListener('invalidate', (e) => {
        try {
          const topic = (JSON.parse((e as MessageEvent).data) as { topic?: unknown }).topic;
          if (typeof topic === 'string') batcher.add(topic);
        } catch {
          // A malformed hint is skipped; the next one (or a resync) catches up.
        }
      });
      es.addEventListener('resync', () => batcher.add('resync'));
      es.addEventListener('error', () => {
        // CONNECTING: the browser is retrying by itself. CLOSED: it gave up (a refusal such as
        // 429 when the server's streams are full, or a sign-out); try again later.
        if (es.readyState !== EventSource.CLOSED) return;
        close();
        missed = true;
        retry = setTimeout(() => {
          retry = undefined;
          // A hidden tab waits: becoming visible opens it.
          if (document.visibilityState !== 'hidden') open();
        }, backoff);
        backoff = Math.min(backoff * 2, 60_000);
      });
    };
    const close = () => {
      source?.close();
      source = undefined;
    };
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        hiddenTimer = setTimeout(() => {
          close();
          missed = true;
        }, HIDDEN_GRACE_MS);
      } else {
        clearTimeout(hiddenTimer);
        if (!source && retry === undefined) open();
      }
    };

    open();
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      clearTimeout(retry);
      clearTimeout(hiddenTimer);
      batcher.stop();
      close();
    };
  }, [qc, profileId]);
}
