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
  | 'backdrop'
  | 'logo'
  | 'genres'
  | 'tagline'
  | 'content_rating'
  | 'studio';

/** A manual metadata edit: fields sent are set and locked; locked_fields replaces the locked set. */
/** A change to many movies and shows at once; anything left out stays as it is. */
export interface BulkEdit {
  item_ids: string[];
  add_genres?: string[];
  remove_genres?: string[];
  add_labels?: string[];
  remove_labels?: string[];
  content_rating?: string;
  /** Lock every field against refreshes, or unlock them all. */
  locks?: 'all' | 'none';
  /** Watched or not, for the current profile. */
  played?: boolean;
  /** Fetch their metadata again in the background. */
  refresh?: boolean;
}

export interface ItemEdit {
  title?: string;
  sort_title?: string;
  original_title?: string;
  year?: number;
  overview?: string;
  release_date?: string;
  runtime_min?: number;
  tagline?: string;
  content_rating?: string;
  studio?: string;
  genres?: string[];
  labels?: string[];
  /** A show's episode numbering; "" follows its library. */
  source_ordering?: SourceOrdering | '';
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
  /** The edition, from the file name, e.g. "Director's Cut". */
  edition?: string;
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
  /** Episodes, where the API includes it: their series' title and place in the profile's order. */
  series_title?: string;
  season_number?: number;
  episode_number?: number;
  /** The current profile's watch state (movies and episodes). */
  user_data?: UserData;
  /** Series: episodes on disk the current profile hasn't watched. */
  unwatched_count?: number;
  tagline?: string;
  /** Age rating for the server's region, e.g. "15" or "PG-13". */
  content_rating?: string;
  /** Studios (movies) or networks (shows), comma-separated. */
  studio?: string;
  /** The language it was made in, 2-letter code (e.g. "ja"). */
  original_language?: string;
  /** Shows: whether new episodes are still coming, and the last and next air dates (YYYY-MM-DD). */
  status?: 'continuing' | 'ended' | 'cancelled' | 'upcoming';
  last_aired?: string;
  next_aired?: string;
  /** Audience score out of 10. */
  rating?: number;
  /** On a movie or show's own page. */
  genres?: string[];
  /** Your own labels (on a movie or show's own page). */
  labels?: string[];
  /** A show's own episode numbering, when it overrides its library's. */
  source_ordering?: SourceOrdering;
  credits?: Credit[];
  extras?: Extra[];
  /** On a movie or show's own page: the collections it's in. */
  collections?: { id: string; title: string; kind: Collection['kind'] }[];
  images?: { primary?: string; backdrop?: string; logo?: string; series_primary?: string; series_backdrop?: string; season_primary?: string };
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
  /** Thumbnails for scrubbing along the player's timeline (off by default). */
  trickplay: boolean;
  /** The fewest films a TMDB collection needs to be shown; 0 for no automatic collections. */
  collection_min: number;
  /** The language titles and descriptions are fetched in (e.g. "fr-FR"); "" for English. */
  metadata_language: string;
  /** How many movies or shows it lists. */
  count: number;
  scan: ScanStatus;
}

export interface LibraryInput {
  name: string;
  kind: LibraryKind;
  source_ordering: SourceOrdering;
  paths: string[];
  trickplay?: boolean;
  collection_min?: number;
  metadata_language?: string;
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
  overview?: string;
  /** Season fields edited by hand. */
  locked_fields: ('name' | 'overview')[];
  /** The season's poster, in orders that season posters belong to. */
  poster?: string;
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

export type PlaybackState = 'playing' | 'paused' | 'buffering';

/** Something being watched right now (Dashboard). */
/** A piece of background work on the server (a scan, scrubbing previews, a backup). */
export interface ActivityTask {
  id: number;
  kind: 'scan' | 'refresh' | 'previews' | 'backup';
  title: string;
  /** What it's doing now, e.g. "Matching movies · 3 of 40". */
  detail?: string;
  done?: number;
  total?: number;
  started_at: string;
  finished_at?: string;
  error?: string;
}

export interface Playback {
  id: string;
  profile: Profile;
  item: Item;
  /** The trailer or other extra being played, by title. */
  extra?: string;
  device: string;
  address: string;
  method: 'direct' | 'transcode';
  position_ms: number;
  duration_ms: number;
  state: PlaybackState;
  started_at: number;
}

export type LogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';

/** What this browser can play, reported when starting playback. */
export interface PlayerCaps {
  containers: string[];
  video: string[];
  audio: string[];
  max_height: number;
}

export type PlaybackMethod = 'direct' | 'remux' | 'transcode';

export interface AudioTrack {
  index: number;
  language?: string;
  title?: string;
  codec: string;
  channels?: number;
  default: boolean;
}

/** How the server will send a movie or episode to this player. */
export interface PlaybackPlan {
  method: PlaybackMethod;
  /** Why it can't be played as it is, e.g. "MKV file", "Dolby Digital audio". */
  reasons: string[];
  url: string;
  /** An HLS playlist (hls.js, or Safari natively). */
  hls: boolean;
  session_id?: string;
  /** Where the stream starts in the file: add it to the video's clock. */
  offset_ms: number;
  duration_ms: number;
  file_id: number;
  audio_stream: number;
  audio_tracks: AudioTrack[];
  subtitle_tracks: SubtitleTrack[];
  /** The title's files: editions and versions (e.g. 4K and 1080p). */
  versions: Version[];
  /** The file's overall bitrate (kbit/s) and picture height, for offering lower qualities. */
  source_kbps: number;
  source_height: number;
  /** The file's chapter marks (times in the file). */
  chapters: Chapter[];
  /** The picture subtitle track being burned into the video, if any. */
  burn_subtitle?: string;
  /** When converting: the graphics card doing it (e.g. "NVIDIA NVENC"), and whether HDR is mapped to SDR. */
  encoder?: string;
  tone_mapped?: boolean;
}

export interface Version {
  file_id: number;
  edition?: string;
  width?: number;
  height?: number;
  video_codec?: string;
  /** HDR10 or HLG. */
  hdr?: string;
  size: number;
  duration_ms?: number;
}

export interface SubtitleTrack {
  id: string;
  language?: string;
  title?: string;
  codec: string;
  forced: boolean;
  default: boolean;
  hearing_impaired: boolean;
  /** A file beside the video. */
  external: boolean;
  /** Found online and kept in Arc's data folder. */
  downloaded?: boolean;
  /** A picture format (Blu-ray, DVD): shown by burning it into the video. */
  image: boolean;
}

/** Timeline thumbnails: one every interval_ms, in sheets of columns × rows. */
export interface TrickplayInfo {
  interval_ms: number;
  width: number;
  height: number;
  columns: number;
  rows: number;
  count: number;
  sheets: number;
}

/** A subtitle file found on OpenSubtitles. */
export interface OnlineSubtitle {
  file_id: number;
  language: string;
  release: string;
  file_name: string;
  downloads: number;
  hearing_impaired: boolean;
  forced: boolean;
  /** Made for exactly this file. */
  hash_match: boolean;
  machine_translated: boolean;
}

export interface SubtitleCue {
  start_ms: number;
  end_ms: number;
  /** Text with line breaks; only <i>, <b> and <u> tags. */
  text: string;
}

export interface LogLine {
  seq: number;
  time: string;
  level: LogLevel;
  message: string;
  attrs?: string;
}

export interface BackupsResponse {
  folder: string;
  backups: Backup[];
}

export type ItemSort = 'title' | 'added' | 'year' | 'release' | 'rating' | 'runtime' | 'watched' | 'shuffle';
export type Resolution = '4k' | '1080' | '720' | 'sd';

export interface Facet {
  value: string;
  count: number;
}

export interface LibraryFacets {
  genres: Facet[];
  decades: Facet[];
  resolutions: Facet[];
  /** Original languages, most titles first. */
  languages: Facet[];
  labels: Facet[];
}

export interface ItemQuery {
  library_id?: number;
  kind?: ItemKind;
  q?: string;
  sort?: ItemSort;
  genre?: string;
  /** Original language, 2-letter code. */
  language?: string;
  label?: string;
  decade?: number;
  resolution?: Resolution;
  unwatched?: boolean;
  /** Added in the last this many days. */
  added_within?: number;
  min_runtime?: number;
  max_runtime?: number;
  min_rating?: number;
  /** Any of these words in the title, tagline or description, comma-separated. */
  about?: string;
  limit?: number;
  offset?: number;
}

/** The browse filters a custom home row is made of. */
export interface RowFilter {
  library_id: number;
  genre?: string;
  language?: string;
  label?: string;
  decade?: number;
  resolution?: Resolution;
  unwatched?: boolean;
  added_within?: number;
  min_runtime?: number;
  max_runtime?: number;
  min_rating?: number;
  sort?: ItemSort;
}

/** A row on the home page: built in ("continue", "next-up", "recent:<library>", "seasonal") or "custom:…". */
export interface HomeRow {
  id: string;
  hidden?: boolean;
  title?: string;
  filter?: RowFilter;
  /** Posters or landscape thumbnails; absent for the row's usual look. */
  style?: 'poster' | 'thumb';
  /** A TV library's Recently Added: one card per show instead of grouping new episodes by season. */
  shows_only?: boolean;
  /** For episodes and seasons on a poster row: the show's poster or the season's. */
  posters?: 'series' | 'season';
}

/** A card in a TV library's Recently Added: a new show, new episodes of a season, or one episode. */
export interface RecentCard {
  type: 'series' | 'season' | 'episode';
  item: Item;
  season_number?: number;
  episodes: number;
  season_poster?: string;
  added_at: number;
}

export type Theme = '' | 'dark' | 'light';

/** The current profile's own preferences. */
export interface Prefs {
  hide_spoilers: boolean;
  /** "" follows the device. */
  theme: Theme;
  /** The subtitle language last chosen, e.g. "eng"; "" for off. */
  subtitle_language: string;
  /** The audio language last chosen, e.g. "jpn"; played by default when a file has it. */
  audio_language?: string;
  /** Up Next as its own row; otherwise next episodes are part of Continue Watching, as in Plex. */
  split_up_next: boolean;
  /** Play the next episode after a countdown; absent means on, as in Plex. */
  auto_play?: boolean;
  /** Empty means the default layout. */
  home: HomeRow[];
  /** The rows are the admin's default home: the profile hasn't arranged its own. */
  home_is_default?: boolean;
}

export type ArtworkType = 'poster' | 'backdrop' | 'logo';

export interface ArtworkImage {
  path: string;
  thumb: string;
  width: number;
  height: number;
  language?: string;
  votes: number;
}

export interface ArtworkChoices {
  posters: ArtworkImage[];
  backdrops: ArtworkImage[];
  logos: ArtworkImage[];
  current: Record<ArtworkType, string>;
  chosen: Record<ArtworkType, boolean>;
  local: Record<ArtworkType, boolean>;
}

export interface Extra {
  id: string;
  item_id: string;
  kind: 'trailer' | 'featurette' | 'behind_the_scenes' | 'deleted_scene' | 'interview' | 'scene' | 'short' | 'other';
  title: string;
  duration_ms?: number;
  stream_url: string;
}

export interface Credit {
  person_id: number;
  name: string;
  role: 'actor' | 'director' | 'writer' | 'creator';
  character?: string;
  image?: string;
}

export interface UserData {
  position_ms?: number;
  played: boolean;
  play_count?: number;
  last_played_at?: number;
  /** The profile's own rating, 1-10 (half stars out of five). */
  rating?: number;
  watchlisted?: boolean;
}

export type ProfileColor = 'violet' | 'blue' | 'teal' | 'green' | 'amber' | 'orange' | 'rose' | 'slate';

export interface Profile {
  id: string;
  name: string;
  color: ProfileColor;
  has_pin: boolean;
  is_admin: boolean;
  /** Age limit: the oldest age rating this profile may watch, 0 for none. */
  max_age: number;
  /** The libraries this profile may see; null for all of them. */
  library_ids: number[] | null;
  /** Titles with these labels are hidden from this profile. */
  exclude_labels: string[];
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
  ffprobe: { found: boolean; path?: string; version?: string; error?: string; /** FFmpeg beside it, for converting. */ ffmpeg?: string };
  local_bypass: boolean;
  /** Convert with the graphics card when one can. */
  hw_accel: boolean;
  /** Map HDR to SDR when converting. */
  tone_mapping: boolean;
  /** OpenSubtitles.com, for finding subtitles online (the key masked; the password is never sent). */
  opensubtitles_api_key: string;
  opensubtitles_username: string;
  /** What this server's FFmpeg can do. */
  transcoder: { hw_encoder: string; hw_label: string; tone_mapping: boolean; checked: boolean };
  port: number;
  allow_network: boolean;
  /** Country whose age ratings are used, e.g. "GB". */
  region: string;
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

async function request<T>(method: string, path: string, body?: unknown, opts: { keepalive?: boolean } = {}): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    // Lets a last progress report finish even if the page is closing.
    keepalive: opts.keepalive,
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
  setup: (body: { name: string; password: string; local_bypass: boolean; tmdb_api_key?: string; tvdb_api_key?: string; region?: string }) =>
    request<Profile>('POST', '/api/auth/setup', body),
  login: (password: string) => request<void>('POST', '/api/auth/login', { password }),
  chooseProfile: (id: string, secret: { pin?: string; password?: string } = {}) =>
    request<Profile>('POST', '/api/auth/profile', { id, ...secret }),
  leaveProfile: () => request<void>('DELETE', '/api/auth/profile'),
  logout: () => request<void>('POST', '/api/auth/logout'),
  changePassword: (current: string, next: string) => request<void>('POST', '/api/auth/password', { current, new: next }),
  profiles: () => request<Profile[]>('GET', '/api/profiles'),
  createProfile: (p: { name: string; color: ProfileColor; pin?: string; max_age?: number; library_ids?: number[] | null; exclude_labels?: string[] }) => request<Profile>('POST', '/api/profiles', p),
  updateProfile: (id: string, p: { name?: string; color?: ProfileColor; pin?: string; max_age?: number; library_ids?: number[] | null; exclude_labels?: string[] }) => request<Profile>('PATCH', `/api/profiles/${id}`, p),
  profilePrefs: (id: string) => request<Prefs>('GET', `/api/profiles/${id}/prefs`),
  updateProfilePrefs: (id: string, patch: Partial<Prefs>) => request<Prefs>('PATCH', `/api/profiles/${id}/prefs`, patch),
  defaultHome: () => request<{ home: HomeRow[] }>('GET', '/api/home/default'),
  setDefaultHome: (home: HomeRow[]) => request<{ home: HomeRow[] }>('PUT', '/api/home/default', { home }),
  deleteProfile: (id: string) => request<void>('DELETE', `/api/profiles/${id}`),
  settings: () => request<ServerSettings>('GET', '/api/settings'),
  updateSettings: (s: Partial<Pick<ServerSettings, 'tmdb_api_key' | 'tvdb_api_key' | 'ffprobe_path' | 'local_bypass' | 'hw_accel' | 'tone_mapping' | 'port' | 'allow_network' | 'region' | 'opensubtitles_api_key' | 'opensubtitles_username'>> & { opensubtitles_password?: string }) =>
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
  compatibility: (libraryId: number, caps: PlayerCaps) => request<Compatibility>('POST', `/api/libraries/${libraryId}/compatibility`, { caps }),
  collections: (libraryId: number) => request<Collection[]>('GET', `/api/libraries/${libraryId}/collections`),
  collection: (id: string) => request<{ collection: Collection; items: Item[] }>('GET', `/api/collections/${id}`),
  createCollection: (c: { library_id: number; kind: 'manual' | 'smart'; title: string; overview?: string; filter?: RowFilter; item_ids?: string[] }) =>
    request<Collection>('POST', '/api/collections', c),
  updateCollection: (id: string, c: { title?: string; sort_title?: string; overview?: string; filter?: RowFilter; locked_fields?: string[] }) =>
    request<Collection>('PATCH', `/api/collections/${id}`, c),
  collectionArtwork: (id: string) => request<ArtworkChoices>('GET', `/api/collections/${id}/artwork`),
  chooseCollectionArtwork: (id: string, type: 'poster' | 'backdrop', path: string | null) =>
    request<unknown>('PUT', `/api/collections/${id}/artwork`, { type, path }),
  uploadCollectionArtwork: async (id: string, type: 'poster' | 'backdrop', file: File) => {
    const res = await fetch(`/api/collections/${id}/artwork/${type}`, { method: 'POST', body: file });
    if (!res.ok) throw new ApiError((await res.json().catch(() => ({}))).error ?? res.statusText, res.status);
  },
  deleteCollection: (id: string) => request<void>('DELETE', `/api/collections/${id}`),
  addToCollection: (id: string, itemIds: string[]) => request<void>('POST', `/api/collections/${id}/items`, { item_ids: itemIds }),
  removeFromCollection: (id: string, itemId: string) => request<void>('DELETE', `/api/collections/${id}/items/${itemId}`),
  orderCollection: (id: string, itemIds: string[]) => request<void>('PUT', `/api/collections/${id}/order`, { item_ids: itemIds }),
  labels: () => request<string[]>('GET', '/api/labels'),
  facets: (libraryId: number) => request<LibraryFacets>('GET', `/api/libraries/${libraryId}/facets`),
  /** Problems needing the user, per library id. */
  healthSummary: () => request<Record<string, number>>('GET', '/api/health/libraries'),

  items: (q: ItemQuery) => request<{ total: number; items: Item[] }>('GET', `/api/items${query(q)}`),
  item: (id: string) => request<Item>('GET', `/api/items/${id}`),
  mergeItems: (targetId: string, itemIds: string[]) => request<void>('POST', '/api/items/merge', { target_id: targetId, item_ids: itemIds }),
  splitItem: (id: string) => request<{ item_ids: string[] }>('POST', `/api/items/${id}/split`),
  editItems: (edit: BulkEdit) => request<void>('POST', '/api/items/bulk', edit),
  editItem: (id: string, edit: ItemEdit) => request<Item>('PATCH', `/api/items/${id}`, edit),
  refreshItem: (id: string) => request<Item>('POST', `/api/items/${id}/refresh`),
  reportProgress: (id: string, positionMS: number, durationMS: number, state: 'playing' | 'paused' | 'stopped' = 'playing') =>
    request<UserData>(
      'PUT',
      `/api/items/${id}/progress`,
      { position_ms: Math.round(positionMS), duration_ms: Math.round(durationMS), state },
      { keepalive: true },
    ),
  playing: () => request<Playback[]>('GET', '/api/playing'),
  activity: () => request<{ running: ActivityTask[]; recent: ActivityTask[] }>('GET', '/api/activity'),
  stopPlaying: (id: string, message: string) => request<void>('POST', `/api/playing/${id}/stop`, { message }),
  startPlayback: (
    itemId: string,
    body: { caps: PlayerCaps; file_id?: number; audio_stream?: number; start_ms?: number; burn_subtitle?: string; edition?: string; max_kbps?: number },
  ) =>
    request<PlaybackPlan>('POST', `/api/items/${itemId}/playback`, body),
  trickplay: (itemId: string, fileId: number) =>
    request<{ info: TrickplayInfo; sheets: string[] }>('GET', `/api/items/${itemId}/trickplay${query({ file: fileId })}`),
  subtitleTracks: (itemId: string, fileId: number) => request<SubtitleTrack[]>('GET', `/api/items/${itemId}/subtitles${query({ file: fileId })}`),
  searchSubtitles: (itemId: string, fileId: number, languages: string) =>
    request<OnlineSubtitle[]>('GET', `/api/items/${itemId}/subtitles/search${query({ file: fileId, languages })}`),
  downloadSubtitle: (itemId: string, fileId: number, s: OnlineSubtitle) =>
    request<{ track: SubtitleTrack; remaining: number }>('POST', `/api/items/${itemId}/subtitles/download`, {
      file: fileId,
      result: s.file_id,
      language: s.language,
      forced: s.forced,
      hearing_impaired: s.hearing_impaired,
    }),
  subtitleCues: (itemId: string, track: string, fileId: number) =>
    request<{ track: SubtitleTrack; cues: SubtitleCue[] }>('GET', `/api/items/${itemId}/subtitles/${track}${query({ file: fileId })}`),
  pingTranscode: (sessionId: string) => request<void>('POST', `/api/transcode/${sessionId}/ping`),
  stopTranscode: (sessionId: string) => request<void>('DELETE', `/api/transcode/${sessionId}`, undefined, { keepalive: true }),
  logs: (level: string, after = 0) => request<LogLine[]>('GET', `/api/logs${query({ level, after })}`),
  setRating: (id: string, rating: number) => request<void>('PUT', `/api/items/${id}/rating`, { rating }),
  setWatchlist: (id: string, on: boolean) => request<void>('PUT', `/api/items/${id}/watchlist`, { on }),
  watchlist: () => request<Item[]>('GET', '/api/watchlist'),
  setPlayed: (id: string, played: boolean, season?: number) =>
    request<void>('PUT', `/api/items/${id}/played`, { played, season_number: season }),
  playlists: () => request<Playlist[]>('GET', '/api/playlists'),
  playlist: (id: string) => request<{ playlist: Playlist; entries: PlaylistEntry[] }>('GET', `/api/playlists/${id}`),
  createPlaylist: (title: string, itemIds: string[] = []) => request<Playlist>('POST', '/api/playlists', { title, item_ids: itemIds }),
  renamePlaylist: (id: string, title: string) => request<void>('PATCH', `/api/playlists/${id}`, { title }),
  deletePlaylist: (id: string) => request<void>('DELETE', `/api/playlists/${id}`),
  addToPlaylist: (id: string, itemIds: string[]) => request<{ added: number }>('POST', `/api/playlists/${id}/items`, { item_ids: itemIds }),
  removeFromPlaylist: (id: string, entryId: number) => request<void>('DELETE', `/api/playlists/${id}/entries/${entryId}`),
  orderPlaylist: (id: string, entryIds: number[]) => request<void>('PUT', `/api/playlists/${id}/order`, { entry_ids: entryIds }),
  search: (q: string) => request<SearchResults>('GET', `/api/search${query({ q })}`),
  person: (id: number) => request<{ person: Person; credits: PersonCredit[] }>('GET', `/api/people/${id}`),
  stats: (p: StatsParams & { days?: number }) => request<StatsOverview>('GET', `/api/stats${query(p)}`),
  statsHistory: (p: StatsParams & { before?: number }) => request<StatsHistory>('GET', `/api/stats/history${query(p)}`),
  statsStorage: (library?: number) => request<StatsStorage>('GET', `/api/stats/storage${query({ library })}`),
  myYear: (year?: number) => request<YearReview>('GET', `/api/me/year${query({ year })}`),
  recentShows: (libraryId: number) => request<RecentCard[]>('GET', `/api/libraries/${libraryId}/recent-shows`),
  /** The episode that follows one in the profile's episode order, if any. */
  similar: (id: string) => request<Item[]>('GET', `/api/items/${id}/similar`),
  recommended: (libraryId: number) =>
    request<{ rows: { title: string; items: Item[]; genre?: string }[] }>('GET', `/api/libraries/${libraryId}/recommended`),
  removeFromContinue: (id: string) => request<void>('DELETE', `/api/home/continue/${id}`),
  nextEpisode: (id: string) => request<Item | undefined>('GET', `/api/items/${id}/next`),
  continueWatching: (withUpNext = false) => request<Item[]>('GET', `/api/home/continue${withUpNext ? '?with_up_next=1' : ''}`),
  nextUp: () => request<Item[]>('GET', '/api/home/next-up'),
  prefs: () => request<Prefs>('GET', '/api/me/prefs'),
  updatePrefs: (patch: Partial<Prefs>) => request<Prefs>('PATCH', '/api/me/prefs', patch),
  matchInfo: (id: string) => request<MatchInfo>('GET', `/api/items/${id}/match`),
  searchMatches: (id: string, provider: Provider, q: string, year?: number) =>
    request<MatchCandidate[]>('GET', `/api/items/${id}/match/search${query({ provider, q, year })}`),
  artwork: (id: string) => request<ArtworkChoices>('GET', `/api/items/${id}/artwork`),
  editSeason: (seriesId: string, season: number, orderingId: number | undefined, edit: { name?: string; overview?: string; locked_fields?: string[] }) =>
    request<void>('PATCH', `/api/series/${seriesId}/seasons/${season}${orderingId ? `?ordering=${orderingId}` : ''}`, edit),
  seasonArt: (seriesId: string) => request<{ seasons: SeasonArt[] }>('GET', `/api/series/${seriesId}/season-art`),
  seasonArtwork: (seriesId: string, season: number) =>
    request<{ posters: ArtworkImage[] }>('GET', `/api/series/${seriesId}/seasons/${season}/artwork`),
  chooseSeasonArtwork: (seriesId: string, season: number, path: string | null) =>
    request<{ seasons: SeasonArt[] }>('PUT', `/api/series/${seriesId}/seasons/${season}/artwork`, { path }),
  uploadSeasonArtwork: async (seriesId: string, season: number, file: File) => {
    const res = await fetch(`/api/series/${seriesId}/seasons/${season}/artwork`, { method: 'POST', body: file });
    if (!res.ok) throw new ApiError((await res.json().catch(() => ({}))).error ?? res.statusText, res.status);
    return (await res.json()) as { seasons: SeasonArt[] };
  },
  chooseArtwork: (id: string, type: ArtworkType, path: string | null) => request<Item>('PUT', `/api/items/${id}/artwork`, { type, path }),
  uploadArtwork: async (id: string, type: ArtworkType, file: File) => {
    const res = await fetch(`/api/items/${id}/artwork/${type}`, { method: 'POST', body: file });
    if (!res.ok) throw new ApiError((await res.json().catch(() => ({}))).error ?? res.statusText, res.status);
    return (await res.json()) as Item;
  },
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
  emptyTrash: (libraryId: number) => request<{ removed: number }>('POST', `/api/libraries/${libraryId}/empty-trash`),
  removeItem: (id: string) => request<void>('DELETE', `/api/items/${id}`),
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

/** How a library's titles would play on a device. */
export interface Compatibility {
  total: number;
  counts: Record<PlaybackMethod, number>;
  /** Why titles can't play as they are, with how many. */
  reasons: Record<string, number>;
  /** Titles that would be converted, and why. */
  converted: { item: Item; reasons: string[] }[];
}

/** A group of a library's titles: automatic (a TMDB collection), manual, or smart (a filter). */
export interface Collection {
  id: string;
  library_id: number;
  kind: 'auto' | 'manual' | 'smart';
  tmdb_id?: number;
  title: string;
  sort_title: string;
  overview?: string;
  /** Fields edited by hand, which TMDB refreshes leave alone. */
  locked_fields: string[];
  count: number;
  images: { primary?: string; backdrop?: string };
  smart_filter?: RowFilter;
}

/** A profile's playlist of movies and episodes. */
export interface Playlist {
  id: string;
  title: string;
  count: number;
  duration_ms: number;
  updated_at: number;
  /** The first few titles' posters. */
  images?: string[];
}

export interface PlaylistEntry {
  entry_id: number;
  item: Item;
}

/** Someone in the cast or crew. */
export interface Person {
  id: number;
  name: string;
  image?: string;
  /** How many titles in the libraries they're in. */
  count: number;
  known_for?: 'actor' | 'director' | 'writer' | 'creator' | '';
}

export interface PersonCredit {
  item: Item;
  roles: Credit['role'][];
  character?: string;
}

export interface SearchResults {
  movies: Item[];
  shows: Item[];
  episodes: Item[];
  people: Person[];
  collections: Collection[];
  genres: { library_id: number; library_name: string; name: string; count: number }[];
}

/** Filters for the Insights pages; empty means everyone or every library. */
export interface StatsParams {
  profile?: string;
  library?: number;
}

/** How much was watched: viewings and time spent playing. */
export interface Bucket {
  plays: number;
  watched_ms: number;
}

/** A movie or show and how much it was watched. */
export interface RankedTitle extends Bucket {
  id: string;
  title: string;
  profiles: number;
  episodes?: number;
}

export interface NamedBucket extends Bucket {
  name: string;
}

/** The titles, profiles and libraries that stats mention by id. */
export interface StatsContext {
  items: Record<string, Item>;
  profiles: Profile[];
  libraries: { id: number; name: string; kind: Library['kind'] }[];
}

export interface StatsOverview extends StatsContext {
  total: Bucket;
  titles: number;
  profiles_watching: number;
  step: 'day' | 'week' | 'month' | '';
  timeline: (Bucket & { date: string })[];
  by_profile: (Bucket & { id: string })[];
  by_library: (Bucket & { id: string })[];
  movies: RankedTitle[];
  shows: RankedTitle[];
  genres: NamedBucket[];
  devices: NamedBucket[];
  methods: NamedBucket[];
  /** Watch time by weekday (0 = Sunday) and hour. */
  hours: number[][];
}

/** One viewing of a movie or episode. */
export interface Play {
  id: number;
  user_id: string;
  item_id: string;
  kind: 'movie' | 'episode';
  library_id: number;
  series_id?: string;
  title: string;
  series_title?: string;
  started_at: number;
  ended_at: number;
  watched_ms: number;
  position_ms: number;
  duration_ms: number;
  completed: boolean;
  device?: string;
  method?: 'direct' | 'remux' | 'transcode';
}

export interface StatsHistory extends StatsContext {
  plays: Play[];
  more: boolean;
}

export interface TitleSize {
  item_id: string;
  size: number;
  files: number;
  added_at: number;
  watched: boolean;
  last_played?: number;
}

export interface StatsStorage {
  libraries: { id: number; name: string; kind: Library['kind']; size: number; titles: number; files: number; unwatched_titles: number; unwatched_size: number }[];
  biggest: TitleSize[];
  never_watched: TitleSize[];
  items: Record<string, Item>;
}

/** A profile's year in review. */
export interface YearReview extends Bucket {
  year: number;
  years: number[];
  movies: number;
  shows: number;
  episodes: number;
  days: number;
  streak: number;
  top_movies: RankedTitle[];
  top_shows: RankedTitle[];
  genres: NamedBucket[];
  people: (Person & { titles: number; watched_ms: number })[];
  months: number[];
  weekdays: number[];
  hours: number[];
  binge?: { series_id: string; title: string; date: string; episodes: number };
  first?: Play;
  items: Record<string, Item>;
}

/** A season of a show and the poster it uses. */
export interface SeasonArt {
  season_number: number;
  name: string;
  image?: string;
  /** Chosen by hand, rather than automatic. */
  chosen: boolean;
  /** There's a poster file for it in the show's folder. */
  local: boolean;
}

/** A chapter mark in a media file. */
export interface Chapter {
  start_ms: number;
  end_ms: number;
  title?: string;
}
