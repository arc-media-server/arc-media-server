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
  /** Episodes, where the API includes it: their series' title. */
  series_title?: string;
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

export type LibraryKind = 'movies' | 'shows';
export type SourceOrdering = 'tvdb_aired' | 'tvdb_dvd' | 'tvdb_absolute' | 'tmdb_aired';

export interface Library {
  id: number;
  name: string;
  kind: LibraryKind;
  source_ordering: SourceOrdering;
  paths: string[];
  /** How many movies or shows it lists. */
  count: number;
  scan: ScanStatus;
}

export interface LibraryInput {
  name: string;
  kind: LibraryKind;
  source_ordering: SourceOrdering;
  paths: string[];
}

export interface PreviewItem {
  kind: 'movie' | 'series';
  title: string;
  year?: number;
  files: number;
  episodes?: number;
  example: ParsedName;
  status: 'pending' | 'identified' | 'unidentified';
  match?: MatchCandidate;
  steps: MatchStep[];
}

export interface LibraryPreview {
  stage: 'scanning' | 'identifying' | 'done';
  error?: string;
  unavailable: string[];
  files: number;
  extras: number;
  unrecognised: string[];
  items: PreviewItem[];
  checked: number;
}

export interface FolderListing {
  path: string;
  /** null at the top level; "" at the top of a drive (go back to the drive list). */
  parent: string | null;
  folders: { name: string; path: string }[];
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

export interface FileProblem {
  file_id: number;
  path: string;
  size: number;
  error?: string;
  missing_since?: number;
  item_id?: string;
  item_title?: string;
  item_kind?: ItemKind;
  series_id?: string;
}

export interface MissingEpisode {
  id: string;
  title: string;
  season_number: number;
  episode_number: number;
  air_date: string;
}

export interface LibraryHealth {
  library: Library;
  unidentified: Item[];
  unreadable: FileProblem[];
  unrecognised: FileProblem[];
  missing_files: FileProblem[];
  duplicates: { kind: 'copies' | 'same_title'; items: Item[]; files?: FileProblem[] }[];
  missing_episodes: { series: Item; ordering: string; have: number; unplaced: number; missing: MissingEpisode[] }[];
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

export type ProfileColor = 'violet' | 'blue' | 'teal' | 'green' | 'amber' | 'orange' | 'rose' | 'slate';

export interface Profile {
  id: string;
  name: string;
  color: ProfileColor;
  has_pin: boolean;
  is_admin: boolean;
}

export interface AuthStatus {
  setup_needed: boolean;
  signed_in: boolean;
  /** Signed in with the password (rather than on the home network without it). */
  password_session: boolean;
  /** This request comes from the home network, and profiles can be picked without the password. */
  bypass: boolean;
  local: boolean;
  profile: Profile | null;
  profiles: Profile[];
}

export interface ServerSettings {
  tmdb_api_key: string;
  tvdb_api_key: string;
  ffprobe_path: string;
  ffprobe: { found: boolean; path?: string; version?: string; error?: string };
  local_bypass: boolean;
  port: number;
  allow_network: boolean;
  listen: string;
  from_env: Record<string, boolean>;
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    /** What the server needs to go on, e.g. "pin" or "password" when choosing a profile. */
    readonly needs?: string,
  ) {
    super(message);
  }
}

/** Fired when the server says the session is gone, so the app can show the sign-in screen. */
export const UNAUTHORIZED_EVENT = 'ams:unauthorized';

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    let message = res.statusText;
    let needs: string | undefined;
    try {
      const body = await res.json();
      message = body.error ?? message;
      needs = body.needs;
    } catch {
      // not JSON; keep the status text
    }
    if (res.status === 401 && !path.startsWith('/api/auth/')) window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
    throw new ApiError(message, res.status, needs);
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
  authStatus: () => request<AuthStatus>('GET', '/api/auth/status'),
  setup: (body: { name: string; password: string; local_bypass: boolean; tmdb_api_key?: string; tvdb_api_key?: string }) =>
    request<Profile>('POST', '/api/auth/setup', body),
  login: (password: string) => request<void>('POST', '/api/auth/login', { password }),
  chooseProfile: (id: string, secret: { pin?: string; password?: string } = {}) =>
    request<Profile>('POST', '/api/auth/profile', { id, ...secret }),
  leaveProfile: () => request<void>('DELETE', '/api/auth/profile'),
  logout: () => request<void>('POST', '/api/auth/logout'),
  changePassword: (current: string, next: string) => request<void>('POST', '/api/auth/password', { current, new: next }),
  profiles: () => request<Profile[]>('GET', '/api/profiles'),
  createProfile: (p: { name: string; color: ProfileColor; pin?: string }) => request<Profile>('POST', '/api/profiles', p),
  updateProfile: (id: string, p: { name?: string; color?: ProfileColor; pin?: string }) => request<Profile>('PATCH', `/api/profiles/${id}`, p),
  deleteProfile: (id: string) => request<void>('DELETE', `/api/profiles/${id}`),
  settings: () => request<ServerSettings>('GET', '/api/settings'),
  updateSettings: (s: Partial<Pick<ServerSettings, 'tmdb_api_key' | 'tvdb_api_key' | 'ffprobe_path' | 'local_bypass' | 'port' | 'allow_network'>>) =>
    request<ServerSettings>('PATCH', '/api/settings', s),

  libraries: () => request<Library[]>('GET', '/api/libraries'),
  createLibrary: (lib: LibraryInput) => request<Library>('POST', '/api/libraries', lib),
  updateLibrary: (id: number, lib: Partial<Omit<LibraryInput, 'kind'>>) => request<Library>('PATCH', `/api/libraries/${id}`, lib),
  deleteLibrary: (id: number) => request<void>('DELETE', `/api/libraries/${id}`),
  startPreview: (lib: Omit<LibraryInput, 'name'>, editingId?: number) =>
    request<{ id: string; preview: LibraryPreview }>('POST', `/api/library-previews${query({ library: editingId })}`, lib),
  preview: (id: string) => request<LibraryPreview>('GET', `/api/library-previews/${id}`),
  folders: (path?: string) => request<FolderListing>('GET', `/api/folders${query({ path })}`),
  scanLibrary: (id: number) => request<ScanStatus>('POST', `/api/libraries/${id}/scan`),
  unmatched: (libraryId: number) => request<Item[]>('GET', `/api/libraries/${libraryId}/unmatched`),
  libraryHealth: (libraryId: number) => request<LibraryHealth>('GET', `/api/libraries/${libraryId}/health`),
  /** Problems needing the user, per library id. */
  healthSummary: () => request<Record<string, number>>('GET', '/api/health/libraries'),

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
