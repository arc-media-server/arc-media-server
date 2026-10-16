// Typed client for the AMS REST API. Field names match the Go JSON exactly.

export type ItemKind = 'movie' | 'series' | 'episode';
export type MetadataState = 'pending' | 'matched' | 'unmatched' | 'locked';
export type LockableField =
  | 'title'
  | 'sort_title'
  | 'original_title'
  | 'year'
  | 'overview'
  | 'release_date'
  | 'runtime_min'
  | 'poster'
  | 'backdrop';

/** A manual metadata edit: fields sent are set and locked; locked_fields replaces the locked set. */
export interface ItemEdit {
  title?: string;
  sort_title?: string;
  original_title?: string;
  year?: number;
  overview?: string;
  release_date?: string;
  runtime_min?: number;
  locked_fields?: LockableField[];
}

export interface Stream {
  index: number;
  kind: 'video' | 'audio' | 'subtitle' | 'attachment' | 'data' | string;
  codec: string;
  profile?: string;
  language?: string;
  title?: string;
  width?: number;
  height?: number;
  channels?: number;
  is_default: boolean;
  is_forced: boolean;
}

export interface MediaFile {
  id: number;
  path: string;
  size: number;
  container?: string;
  duration_ms?: number;
  bitrate?: number;
  probe_error?: string;
  missing?: boolean;
  part: number;
  streams: Stream[];
}

export interface Item {
  id: string;
  library_id: number;
  kind: ItemKind;
  parent_id?: string;
  title: string;
  sort_title: string;
  original_title?: string;
  year?: number;
  overview?: string;
  runtime_min?: number;
  release_date?: string;
  tmdb_id?: number;
  tvdb_id?: number;
  imdb_id?: string;
  metadata_state: MetadataState;
  added_at: number;
  locked_fields: LockableField[];
  images?: { primary?: string; backdrop?: string };
  stream_url?: string;
  files?: MediaFile[];
}

export interface ScanStatus {
  running: boolean;
  started_at?: string;
  finished_at?: string;
  files_seen: number;
  files_probed: number;
  new_items: number;
  files_missing: number;
  matched: number;
  unmatched: number;
  not_recognised: number;
  error?: string;
}

export interface Library {
  id: number;
  name: string;
  kind: 'movies' | 'shows';
  source_ordering: string;
  paths: string[];
  scan: ScanStatus;
}

export interface Ordering {
  id: number;
  series_id: string;
  source: string;
  source_ref?: string;
  name: string;
  description?: string;
  episode_count: number;
}

export interface OrderingsResponse {
  display_ordering_id: number;
  source_ordering: string;
  orderings: Ordering[];
}

export interface SeasonEpisode extends Item {
  season_number: number;
  episode_number: number;
  available: boolean;
  locked?: boolean;
}

export interface Season {
  season_number: number;
  name: string;
  episodes: SeasonEpisode[];
}

export interface SeasonsResponse {
  series: Item;
  ordering: Ordering | null;
  seasons: Season[];
  unplaced: SeasonEpisode[];
}

export type Provider = 'tmdb' | 'tvdb';

export interface MatchCandidate {
  provider: Provider;
  id: number;
  title: string;
  original_title?: string;
  year?: number;
  overview?: string;
  poster?: string;
}

export interface MatchStep {
  provider: Provider;
  kind: 'search' | 'find' | 'lookup' | 'chosen';
  query: string;
  year?: number;
  results: MatchCandidate[];
  picked?: number;
  note?: string;
  error?: string;
}

export interface ParsedName {
  path: string;
  source: string;
  source_kind: 'file' | 'folder';
  title: string;
  year?: number;
  tmdb_id?: number;
  tvdb_id?: number;
  imdb_id?: string;
  season?: number;
  episodes?: number[];
  absolute?: boolean;
  air_date?: string;
  recognised: boolean;
}

export interface MatchInfo {
  item: Item;
  parsed: ParsedName[];
  checked_at: number;
  steps: MatchStep[];
  providers: Record<Provider, boolean>;
  source_ordering: string;
}

export type BackupReason = 'auto' | 'manual' | 'before-restore' | 'before-upgrade';

export interface Backup {
  name: string;
  created_at: string;
  reason: BackupReason;
  size: number;
}

export interface BackupsResponse {
  folder: string;
  backups: Backup[];
}

export interface ItemQuery {
  library_id?: number;
  kind?: ItemKind;
  q?: string;
  sort?: 'title' | 'added' | 'year';
  limit?: number;
  offset?: number;
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    let message = res.statusText;
    try {
      message = (await res.json()).error ?? message;
    } catch {
      // not JSON; keep the status text
    }
    throw new ApiError(message, res.status);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

function query(params: object): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `?${s}` : '';
}

export const api = {
  libraries: () => request<Library[]>('GET', '/api/libraries'),
  scanLibrary: (id: number) => request<ScanStatus>('POST', `/api/libraries/${id}/scan`),
  unmatched: (libraryId: number) => request<Item[]>('GET', `/api/libraries/${libraryId}/unmatched`),

  items: (q: ItemQuery) => request<{ total: number; items: Item[] }>('GET', `/api/items${query(q)}`),
  item: (id: string) => request<Item>('GET', `/api/items/${id}`),
  editItem: (id: string, edit: ItemEdit) => request<Item>('PATCH', `/api/items/${id}`, edit),
  refreshItem: (id: string) => request<Item>('POST', `/api/items/${id}/refresh`),
  matchInfo: (id: string) => request<MatchInfo>('GET', `/api/items/${id}/match`),
  searchMatches: (id: string, provider: Provider, q: string, year?: number) =>
    request<MatchCandidate[]>('GET', `/api/items/${id}/match/search${query({ provider, q, year })}`),
  setMatch: (id: string, provider: Provider, matchId: number) =>
    request<Item>('PUT', `/api/items/${id}/match`, { provider, id: matchId }),

  seasons: (seriesId: string, opts: { ordering?: number; missing?: boolean } = {}) =>
    request<SeasonsResponse>(
      'GET',
      `/api/series/${seriesId}/seasons${query({ ordering: opts.ordering, missing: opts.missing ? 1 : undefined })}`,
    ),
  orderings: (seriesId: string) => request<OrderingsResponse>('GET', `/api/series/${seriesId}/orderings`),
  setDisplayOrdering: (seriesId: string, orderingId: number | null) =>
    request<OrderingsResponse>('PUT', `/api/series/${seriesId}/display-ordering`, { ordering_id: orderingId }),
  refreshSeries: (seriesId: string) => request<OrderingsResponse>('POST', `/api/series/${seriesId}/refresh`),

  backups: () => request<BackupsResponse>('GET', '/api/backups'),
  createBackup: () => request<Backup>('POST', '/api/backups'),
  restoreBackup: (name: string) => request<void>('POST', `/api/backups/${encodeURIComponent(name)}/restore`),
  deleteBackup: (name: string) => request<void>('DELETE', `/api/backups/${encodeURIComponent(name)}`),
};

export function posterUrl(item: Item): string | undefined {
  return item.images?.primary;
}

export function backdropUrl(item: Item): string | undefined {
  return item.images?.backdrop;
}

export function itemHref(item: Item): string {
  switch (item.kind) {
    case 'series':
      return `/series/${item.id}`;
    case 'episode':
      return `/series/${item.parent_id}`;
    default:
      return `/movie/${item.id}`;
  }
}
