import i18n from '@/i18n';
// Typed client for the AMS REST API. Field names match the Go JSON exactly.

export type ItemKind = 'movie' | 'series' | 'episode';
/** "locked": never looked up online (home videos in an Other videos library). */
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
  details_provider?: '' | 'tvdb';
  locked_fields?: LockableField[];
}

export interface Stream {
  index: number;
  kind: 'video' | 'audio' | 'subtitle' | 'attachment' | 'data' | string;
  codec: string;
  profile?: string;
  channel_layout?: string;
  color_transfer?: string;
  color_primaries?: string;
  bit_depth?: number;
  dolby_vision?: boolean;
  dolby_vision_profile?: number;
  hdr10_plus?: boolean;
  bitrate?: number;
  frame_rate?: number;
  pix_fmt?: string;
  /** sampled means a bounded read, never proof of whole-file absence. */
  hdr_probe_status?: '' | 'stream' | 'sampled' | 'unavailable';
  language?: string;
  title?: string;
  width?: number;
  height?: number;
  /** Display aspect ratio, e.g. 1.78 or 2.39; missing until the file has been analysed. */
  aspect_ratio?: number;
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
  /** Other sites' scores: IMDb (out of 10, and its votes), Rotten Tomatoes' critics (percent), Metacritic (out of 100). */
  imdb_rating?: number;
  imdb_votes?: number;
  rotten_tomatoes?: number;
  metacritic?: number;
  /** Audience score out of 10. */
  rating?: number;
  /** On a movie or show's own page. */
  genres?: string[];
  /** Your own labels (on a movie or show's own page). */
  labels?: string[];
  /** A theme tune found beside the title, played quietly on its page. */
  theme_url?: string;
  /** A show's own episode numbering, when it overrides its library's. */
  source_ordering?: SourceOrdering;
  /** Where a show's details and artwork come from: TMDB (absent) or TheTVDB. */
  details_provider?: 'tvdb';
  credits?: Credit[];
  extras?: Extra[];
  /** On a movie or show's own page: the collections it's in. */
  collections?: { id: string; title: string; kind: Collection['kind'] }[];
  /** backdrop is sized for cards; backdrop_full is the full-resolution picture for banners. */
  images?: { primary?: string; backdrop?: string; backdrop_full?: string; logo?: string; series_primary?: string; series_backdrop?: string; series_logo?: string; season_primary?: string };
  stream_url?: string;
  files?: MediaFile[];
}

export interface ScanStatus {
  job_id?: number;
  operation?: 'scan' | 'maintenance' | 'refresh' | 'analyze' | 'analyze_item' | 'empty_trash';
  stopping?: boolean;
  stopped?: boolean;
  running: boolean;
  started_at?: string;
  finished_at?: string;
  files_seen: number;
  files_probed: number;
  files_reused?: number;
  probe_errors?: number;
  metadata_errors?: number;
  new_items: number;
  files_missing: number;
  matched: number;
  unmatched: number;
  not_recognised: number;
  error?: string;
  /** The stage after looking through the folders (e.g. "Matching movies"), and how far through it is. */
  phase?: string;
  done?: number;
  total?: number;
}

/** "videos" is Other videos: home videos and the like, with no online details. */
export type LibraryKind = 'movies' | 'shows' | 'videos';
export type SourceOrdering = 'tvdb_aired' | 'tvdb_dvd' | 'tvdb_absolute' | 'tmdb_aired';

export interface Library {
  details_provider: "" | "tvdb";
  id: number;
  name: string;
  kind: LibraryKind;
  source_ordering: SourceOrdering;
  paths: string[];
  /** Thumbnails for scrubbing along the player's timeline (off by default). */
  trickplay: boolean;
  /** The fewest films a TMDB collection needs to be shown; 0 for no automatic collections. */
  collection_min: number;
  /** How collections show in the grid unless they say: "" their titles, or the collection instead, or both. */
  collection_grid: '' | 'collection' | 'both';
  /** The language titles and descriptions are fetched in (e.g. "fr-FR"); "" for English. */
  metadata_language: string;
  /** The country whose age ratings the library uses (e.g. "GB"). */
  rating_region: string;
  /** Offer Skip Intro (TV) and Skip Credits (TV and movies). */
  skip_intros: boolean;
  skip_credits: boolean;
  /** How many movies or shows it lists. */
  count: number;
  scan: ScanStatus;
}

export interface LibraryInput {
  details_provider?: "" | "tvdb";
  name: string;
  kind: LibraryKind;
  source_ordering: SourceOrdering;
  paths: string[];
  trickplay?: boolean;
  collection_min?: number;
  metadata_language?: string;
  rating_region?: string;
  skip_intros?: boolean;
  skip_credits?: boolean;
  collection_grid?: Library['collection_grid'];
}

export interface PreviewItem {
  kind: 'movie' | 'series';
  title: string;
  year?: number;
  files: number;
  episodes?: number;
  example: ParsedName;
  status: 'pending' | 'identified' | 'unidentified' | 'local';
  match?: MatchCandidate;
  steps: MatchStep[];
}

export interface LibraryPreview {
  stage: 'scanning' | 'identifying' | 'done';
  /** Other videos: how many folders (collections) were found. */
  folders?: number;
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
  display_choice_id?: number;
	inline_specials: boolean;
  display_ordering_id: number;
  source_ordering: string;
  orderings: Ordering[];
}

export interface SeasonEpisode extends Item {
	inline_special?: boolean;
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
  watch_problem?: string;
  library: Library;
  unidentified: Item[];
  unreadable: FileProblem[];
  unrecognised: FileProblem[];
  missing_files: FileProblem[];
  /** same_title: where each title is (its file, or a show's folder), by item id. */
  duplicates: { kind: 'copies' | 'same_title'; items: Item[]; files?: FileProblem[]; folders?: Record<string, string> }[];
  missing_episodes: { series: Item; ordering: string; have: number; unplaced: number; missing: MissingEpisode[] }[];
}

export type BackupReason = 'auto' | 'manual' | 'before-restore' | 'before-upgrade' | 'before-delete';

export interface Backup {
  name: string;
  created_at: string;
  reason: BackupReason;
  size: number;
}

export type PlaybackState = 'playing' | 'paused' | 'buffering';

/** Something being watched right now (Dashboard). */
/** A piece of background work on the server (a scan, scrubbing previews, a backup). */
export type ProcessingMode = 'off' | 'scheduled' | 'added';
/** How many of a title's files have intro/credits markers, and how many were checked (Library Health). */
export interface MarkerCoverage { id: string; title: string; kind: 'series' | 'movie'; files: number; intros: number; credits: number; checked: number }

export interface ScheduledTask { kind: string; title: string; state: string; detail: string; pending: number; completed: number; failed: number; enabled: boolean; requested?: boolean; last_run?: { started_at: number; finished_at: number; last_success_at: number; detail: string; error?: string; paused?: boolean; stopped?: boolean } }

export interface ActivityTask {
  can_stop?: boolean;
  stopping?: boolean;
  stopped?: boolean;
  id: number;
  kind: 'scan' | 'refresh' | 'analysis' | 'previews' | 'backup' | 'maintenance' | 'ratings' | 'import' | 'markers' | 'optimize';
  title: string;
  /** What it's doing now, e.g. "Matching movies · 3 of 40". */
  detail?: string;
  current?: ActivityOperation[];
  done?: number;
  total?: number;
  started_at: string;
  finished_at?: string;
  error?: string;
}

export interface ActivityOperation {
  id: number;
  stage: string;
  subject: string;
  detail?: string;
  done?: number;
  total?: number;
}

/** An address Arc posts events to. */
export interface Webhook {
  id: string;
  name: string;
  url: string;
  /** The events it gets; empty for all. */
  events: string[];
  format: 'discord' | 'slack' | 'json';
  last?: { at: string; event: string; status: string; ok: boolean };
}

/** Something new for the profile: a movie, or a show with new episodes. */
export interface NewsEntry {
  item: Item;
  count: number;
  added_at: number;
  unread: boolean;
}

/** Importing watch history from Plex or Jellyfin. */
export interface ImportSession {
  id: string;
  source: 'plex' | 'jellyfin';
  stage: 'signing_in' | 'choose_server' | 'choose_accounts' | 'running' | 'done' | 'failed';
  auth_url?: string;
  servers?: { id: string; name: string; owned: boolean }[];
  server?: string;
  accounts?: { id: string; name: string; owner?: boolean; suggested_profile?: string }[];
  results?: {
    account: string; profile: string; matched: number; not_found: number; history: number; missing?: string[];
    /** Watchlist titles added, playlists made or added to and their new entries, list members not in Arc. */
    watchlist?: number; playlists?: number; playlist_items?: number; lists_not_found?: number;
    /** Why the watchlist or playlists couldn't be read (watch state still came over). */
    lists_error?: string;
  }[];
  /** The server's collections, brought into the libraries their titles are in. */
  collections?: { collections: number; collection_items: number; not_found: number; missing?: string[]; error?: string };
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
  method: PlaybackMethod;
  /** The conversion is far enough ahead and waits for the viewer. */
  throttled?: boolean;
  /** Part of a Watch Together room, with how many are in it (never who, or the code). */
  watch_together_room_id?: string;
  watch_together_participants?: number;
  /** How it's sent (Plex's stream details); absent for plays started before the server knew. */
  stream?: StreamDetail;
  /** For a conversion: how far into the file it has converted. */
  converted_ms?: number;
  position_ms: number;
  duration_ms: number;
  state: PlaybackState;
  started_at: number;
}

/** How a playback is sent: why, the file's video and audio and what the player gets, and the encoder. */
export interface StreamDetail {
  reasons?: string[];
  video?: string;
  audio?: string;
  /** Absent when sent as it is. */
  video_output?: string;
  audio_output?: string;
  /** The graphics card ("NVIDIA NVENC") or "CPU"; absent when no video is encoded. */
  encoder?: string;
  hardware?: boolean;
  tone_mapped?: boolean;
  subtitles_burned?: boolean;
}

export type LogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';

/** What this browser can play, reported when starting playback. */
export interface PlayerCaps {
  containers: string[];
  video: string[];
  audio: string[];
  max_height: number;
  /** On a retry: the method the browser was given and couldn't play. */
  failed_method?: 'direct' | 'remux';
}

export type PlaybackMethod = 'direct' | 'remux' | 'transcode';

export interface AudioTrack {
  index: number;
  profile?: string;
  channel_layout?: string;
  language?: string;
  title?: string;
  codec: string;
  channels?: number;
  default: boolean;
}

/** Source choices resolved with the same edition/version and audio defaults as playback. */
export type PlaybackOptions = Pick<PlaybackPlan, 'file_id' | 'audio_stream' | 'audio_tracks' | 'subtitle_tracks' | 'versions'>;

/** How the server will send a movie or episode to this player. */
export interface PlaybackPlan {
 rolling_hls?: boolean;
  /** Current server policy, independent of the player's requested cap; 0 is unlimited. */
  server_limit_kbps: number;
  /** A generated MP4; source track indices remain in audio_stream/audio_tracks for choices. */
  optimized?: { quality: number; source_audio_stream: number; audio?: AudioTrack; duration_ms?: number; bitrate?: number };
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
  markers: Marker[];
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
  source_formats?: string[];
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

/** A row of a library's Recommended view: what it is and what it's about, for the app to title. */
export type RecommendedLook = { type: RecommendedRow['type']; hidden?: boolean; style?: '' | 'poster' | 'thumb' | 'arc' };

export interface RecommendedRow {
  style?: 'poster' | 'thumb' | 'arc';
  type:
    | 'continue'
    | 'recently_released'
    | 'recently_added'
    | 'start_watching'
    | 'because_you_watched'
    | 'top_rated'
    | 'recently_watched'
    | 'rediscover'
    | 'more_in_genre'
    | 'top_in_genre'
    | 'top_by_person'
    | 'top_unwatched';
  items: Item[];
  /** The title, genre or person the row is about (or the library's name). */
  name?: string;
  genre?: string;
  person_id?: number;
  role?: 'actor' | 'director';
}

export type ItemSort = 'title' | 'added' | 'year' | 'release' | 'rating' | 'critics' | 'runtime' | 'watched' | 'shuffle';
export type Resolution = '4k' | '1080' | '720' | 'sd';

export interface Facet {
  value: string;
  count: number;
}

export interface LibraryFacets {
  /** How many titles the Unmatched filter finds. */
  unmatched: number;
  genres: Facet[];
  decades: Facet[];
  resolutions: Facet[];
  /** Original languages, most titles first. */
  languages: Facet[];
  labels: Facet[];
  /** Media details of present files, most titles first (absent from older servers). */
  video_codecs?: Facet[];
  audio_codecs?: Facet[];
  audio_channels?: Facet[];
  dynamic_ranges?: Facet[];
}

export type DynamicRange = 'dolby_vision' | 'hdr10_plus' | 'hlg' | 'hdr10' | 'sdr' | 'unknown';
export type WatchState = 'played' | 'unstarted' | 'unwatched';

export interface ItemQuery {
  library_id?: number;
  kind?: ItemKind;
  q?: string;
  sort?: ItemSort;
  reverse?: boolean;
  person_id?: number;
  person_role?: 'actor' | 'director';
  genre?: string;
  /** Original language, 2-letter code. */
  language?: string;
  label?: string;
  decade?: number;
  resolution?: Resolution;
  unwatched?: boolean;
  /** ffprobe codec name, e.g. hevc or truehd. */
  video_codec?: string;
  audio_codec?: string;
  /** Exact audio channel count: 6 is 5.1, 8 is 7.1. */
  audio_channels?: number;
  dynamic_range?: DynamicRange;
  /** Watched, or not started at all (unwatched, which includes in progress, is `unwatched`). */
  watch_state?: WatchState;
  /** Only titles Arc couldn't identify, and shows with episodes it couldn't place. */
  unmatched?: boolean;
  /** Added in the last this many days. */
  added_within?: number;
  min_runtime?: number;
  max_runtime?: number;
  min_rating?: number;
  /** Any of these words in the title, tagline or description, comma-separated. */
  about?: string;
  limit?: number;
  offset?: number;
  /** Start at this A–Z title boundary; only with title sort. Omit for later offset pages. */
  jump?: string;
  /** Also count the titles by first letter (`letters` in the response). */
  letters?: 1;
}

/** How many titles in a listing start with a letter ("a"–"z", "#" for anything else), in title order. */
export interface LetterCount {
  letter: string;
  count: number;
}

/** The browse filters a custom home row is made of. */
export interface RowFilter extends Pick<ItemQuery,
  'genre' | 'language' | 'label' | 'decade' | 'resolution' | 'unwatched' |
  'video_codec' | 'audio_codec' | 'audio_channels' | 'dynamic_range' | 'watch_state' |
  'added_within' | 'min_runtime' | 'max_runtime' | 'min_rating' | 'person_id' | 'person_role' | 'sort'> {
  library_id: number;
}

/** A row on the home page: built in ("continue", "next-up", "recent:<library>", "seasonal") or "custom:…". */
export interface HomeRow {
  id: string;
  hidden?: boolean;
  title?: string;
  filter?: RowFilter;
  /** Posters or landscape thumbnails; absent for the row's usual look. */
  style?: 'poster' | 'thumb' | 'arc';
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
export type SubtitleMode = '' | 'manual' | 'foreign' | 'always';
export interface SubtitleStyle {
  size?: 'small' | 'medium' | 'large' | 'huge';
  color?: 'white' | 'yellow';
  background?: 'box' | 'solid' | 'none';
  position?: 'bottom' | 'top';
}

/** The current profile's own preferences. */
export interface Prefs {
  /** Poster scale in percent, 75–150; absent/zero keeps the existing 100% default. */
  home_poster_size?: number;
  /** Shared across this profile's libraries, independently of Home. */
  library_poster_size?: number;
  /** Large featured Home banner; visible unless explicitly false. */
  show_home_hero?: boolean;
  /** Navigation order only; unavailable/unknown IDs never grant access. */
  library_order?: number[];
  /** How many libraries come before the Watchlist in navigation (and on Home); absent is 0. */
  watchlist_position?: number;
  /** Future progress threshold; absent/zero retains 90%. */
  watched_percent?: number;
  hide_spoilers: boolean;
  /** "" follows the device. */
  theme: Theme;
  /** The subtitle language last chosen, e.g. "eng"; "" for off. */
  subtitle_language: string;
  /** The audio language last chosen, e.g. "jpn"; played by default when a file has it. */
  audio_language?: string;
  /** Skip intros and recaps without asking. */
  auto_skip?: boolean;
  /** Internet streaming quality (kbit/s, one of `qualities`): the most a stream away from home may use; absent/0 for Maximum. Never applies at home. */
  remote_quality_kbps?: number;
  /** When subtitles turn on by themselves (Plex's subtitle mode); absent: in the language last chosen. */
  subtitle_mode?: SubtitleMode;
  /** How subtitles look; absent fields use the defaults. */
  subtitle_style: SubtitleStyle;
  /** Play a title's theme tune on its page; on unless false. */
  theme_music?: boolean;
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
	episode_count?: number;
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
  /** A friend outside the household, and whether they've used their invite yet. */
  is_friend?: boolean;
  joined?: boolean;
  /** A new friend's invite link token (only when they've just been added). */
  invite_token?: string;
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
  /** Signed in as a friend (with their own name and password): no profile picker. */
  friend: boolean;
}

/** A signed-in browser or app (Settings > Devices). */
export interface Device {
  is_tool: boolean;
  id: string;
  /** An app's name for itself; empty for a browser. */
  device: string;
  user_agent: string;
  via: 'password' | 'local' | 'friend';
  profile: string;
  created_at: number;
  last_seen_at: number;
  this_device: boolean;
}

/** How remote access stands. */
export interface RemoteStatus {
  enabled: boolean;
  /** The address to use away from home, once it's known. */
  url?: string;
  public_ip?: string;
  port: number;
  upnp: { state: 'off' | 'working' | 'mapped' | 'failed'; router?: string; external_ip?: string; error?: string };
  https: { mode: ServerSettings['https_mode']; names?: string[]; expires?: number; fetching?: boolean; error?: string };
  /** The router's own outside address isn't on the internet: forwarding a port won't be enough. */
  double_nat?: boolean;
}

/** One moment of the Dashboard's graphs; -1 where the system can't say. */
export interface ResourceSample {
  t: number;
  cpu_arc: number;
  cpu_system: number;
  mem_arc: number;
  mem_used: number;
  /** Bytes a second sent to devices at home and away. */
  net_home: number;
  net_away: number;
}

export interface ServerSettings {
  /** The server's friendly name, e.g. "Living Room" (apps show it; separate from its address). */
  server_name?: string;
  tmdb_api_key: string;
  tvdb_api_key: string;
  ffprobe_path: string;
  ffprobe: { found: boolean; path?: string; version?: string; error?: string; /** FFmpeg beside it, for converting. */ ffmpeg?: string };
  local_bypass: boolean;
  /** Convert with the graphics card when one can. */
  hw_accel: boolean;
  /** The graphics device that converts (one of transcoder.devices' ids), "" for Automatic. */
  hw_device?: string;
  /** Map HDR to SDR when converting. */
  tone_mapping: boolean;
  /** OpenSubtitles.com, for finding subtitles online (the key masked; the password is never sent). */
  opensubtitles_api_key: string;
  opensubtitles_username: string;
  /** What this server's FFmpeg can do. */
  transcoder: { hw_encoder: string; hw_label: string; hw_device?: string; hw_driver?: string; hw_low_power?: boolean; tone_mapping: boolean; checked: boolean; checking?: boolean; attempts?: { encoder: string; device?: string; driver?: string; low_power?: boolean; error?: string }[]; devices?: { id: string; label: string; name?: string; encoder: string; device?: string }[] };
  port: number;
  allow_network: boolean;
  /** Country whose age ratings are used, e.g. "GB". */
  region: string;
  /** The maintenance window, in hours (0–23): refreshing details and backups wait for it. */
  scan_automatic: boolean;
  scan_partial: boolean;
  scan_on_start: boolean;
  scan_interval_hours: number;
  scan_interval_minutes: number;
  preview_mode: ProcessingMode;
  detection_mode: ProcessingMode;
  analysis_enabled: boolean;
  metadata_refresh_enabled: boolean;
  auto_backups_enabled: boolean;
  empty_trash_enabled: boolean;
  database_upkeep_enabled: boolean;
  cache_cleanup_enabled: boolean;
  maintenance_start: number;
  maintenance_end: number;
  /** Make scrubbing previews only in the maintenance window. */
  previews_at_night: boolean;
  /** TheIntroDB, for intro and credits timings (masked), and whether to find them on this computer. */
  theintrodb_api_key: string;
  detect_intros: boolean;
  /** Streams to people away from home: the most each may use, and the home's upload speed (kbps, 0 for none). */
  remote_kbps: number;
  upload_kbps: number;
  /** Reachable from the internet, at an outside port, opened on the router with UPnP. */
  public_url: string;
  remote_access: boolean;
  remote_port: number;
  upnp: boolean;
  /** HTTPS on Arc's port: off (""), your own certificate files, or Let's Encrypt for a domain. */
  https_mode: '' | 'files' | 'letsencrypt';
  https_cert: string;
  https_key: string;
  https_domain: string;
  /** Networks that count as home besides detected interface subnets, e.g. "100.64.0.0/10". */
  home_networks: string;
  automatic_home_networks?: string[];
  home_network_bridge?: boolean;
  /** The /24 of this device when it's on a private network Arc doesn't count as home; "" otherwise. */
  suggested_home_network?: string;
  /** Reverse proxies whose forwarding headers are believed (none by default). */
  trusted_proxies: string;
  /** Where converted streams are written ("" for Arc's data folder), and videos converted at once (0 for no limit). */
  transcode_dir: string;
  max_transcodes: number;
  /** OMDb, for Rotten Tomatoes' and Metacritic's scores (masked). */
  omdb_api_key: string;
  /** Pre-roll videos before movies: files or folders, one per line; and whether one plays at random or all. */
  preroll_paths: string;
  preroll_mode: 'random' | 'all';
  /** How many pre-roll videos those paths hold. */
  preroll_found: number;
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

/** What this server supports (GET /api/capabilities); a missing key means it doesn't advertise it. */
export interface Capabilities {
  api_version: number;
  features: Record<string, { supported: boolean; available: boolean; allowed: boolean }>;
}

/** A play queue entry; one whose title is gone or no longer allowed carries only its id. */
export interface PlayQueueEntry { id: string; available: boolean; item_id?: string; item?: Item }

/** This device and profile's play queue, kept by the server so it survives a reload. */
export interface PlayQueue {
  id: string;
  revision: number;
  /** Empty once the queue has played through. */
  current_entry_id: string;
  shuffled: boolean;
  expires_at: number;
  entries: PlayQueueEntry[];
}

export type PlayQueueEdit =
  | { operation: 'append' | 'next'; item_ids: string[] }
  | { operation: 'remove' | 'reorder'; entry_ids: string[] }
  | { operation: 'select'; entry_id: string }
  | { operation: 'clear' | 'advance' | 'shuffle' | 'unshuffle' };

export const api = {
  diagnostics: () => request<{ detailed: boolean; until?: string; persistent: boolean; storage_error?: string }>('GET', '/api/diagnostics'),
  downloadDiagnostics: async () => { const r = await fetch('/api/diagnostics/download'); if (!r.ok) throw new Error(i18n.t('common:errors.downloadDiagnostics')); return r.blob(); },
  setDiagnostics: (detailed: boolean) => request<{ detailed: boolean; until?: string; persistent: boolean; storage_error?: string }>('PATCH', '/api/diagnostics', { detailed }),
  authStatus: () => request<AuthStatus>('GET', '/api/auth/status'),
  setup: (body: { name: string; password: string; local_bypass: boolean; tmdb_api_key?: string; tvdb_api_key?: string; region?: string }) =>
    request<Profile>('POST', '/api/auth/setup', body),
  login: (password: string) => request<void>('POST', '/api/auth/login', { password }),
  friendLogin: (name: string, password: string) => request<void>('POST', '/api/auth/friend', { name, password }),
  invite: (token: string) => request<{ name: string }>('GET', `/api/invites/${encodeURIComponent(token)}`),
  acceptInvite: (token: string, password: string) => request<void>('POST', `/api/invites/${encodeURIComponent(token)}`, { password }),
  reinvite: (id: string) => request<{ invite_token: string }>('POST', `/api/profiles/${id}/invite`),
  chooseProfile: (id: string, secret: { pin?: string; password?: string } = {}) =>
    request<Profile>('POST', '/api/auth/profile', { id, ...secret }),
  leaveProfile: () => request<void>('DELETE', '/api/auth/profile'),
  logout: () => request<void>('POST', '/api/auth/logout'),
  changePassword: (current: string, next: string) => request<void>('POST', '/api/auth/password', { current, new: next }),
  profiles: () => request<Profile[]>('GET', '/api/profiles'),
  createProfile: (p: { name: string; color: ProfileColor; friend?: boolean; pin?: string; max_age?: number; library_ids?: number[] | null; exclude_labels?: string[] }) => request<Profile>('POST', '/api/profiles', p),
  updateProfile: (id: string, p: { name?: string; color?: ProfileColor; pin?: string; max_age?: number; library_ids?: number[] | null; exclude_labels?: string[] }) => request<Profile>('PATCH', `/api/profiles/${id}`, p),
  profilePrefs: (id: string) => request<Prefs>('GET', `/api/profiles/${id}/prefs`),
  updateProfilePrefs: (id: string, patch: Partial<Prefs>) => request<Prefs>('PATCH', `/api/profiles/${id}/prefs`, patch),
  defaultHome: () => request<{ home: HomeRow[] }>('GET', '/api/home/default'),
  setDefaultHome: (home: HomeRow[]) => request<{ home: HomeRow[] }>('PUT', '/api/home/default', { home }),
  deleteProfile: (id: string) => request<void>('DELETE', `/api/profiles/${id}`),
  settings: () => request<ServerSettings>('GET', '/api/settings'),
  detectHardware: () => request<{started: boolean}>('POST', '/api/transcoder/detect'),
  resources: (day: boolean) => request<{ cores: number; mem_total: number; samples: ResourceSample[] }>('GET', `/api/dashboard/resources${day ? '?range=day' : ''}`),
  remote: () => request<RemoteStatus>('GET', '/api/remote'),
  checkRemote: () => request<{ reachable: boolean; error?: string }>('POST', '/api/remote/check'),
  updateSettings: (s: Partial<Pick<ServerSettings, 'server_name' | 'tmdb_api_key' | 'tvdb_api_key' | 'ffprobe_path' | 'local_bypass' | 'hw_accel' | 'hw_device' | 'tone_mapping' | 'port' | 'allow_network' | 'region' | 'opensubtitles_api_key' | 'opensubtitles_username' | 'scan_automatic' | 'scan_partial' | 'scan_on_start' | 'scan_interval_hours' | 'scan_interval_minutes' | 'preview_mode' | 'detection_mode' | 'analysis_enabled' | 'metadata_refresh_enabled' | 'auto_backups_enabled' | 'empty_trash_enabled' | 'database_upkeep_enabled' | 'cache_cleanup_enabled' | 'maintenance_start' | 'maintenance_end' | 'previews_at_night' | 'omdb_api_key' | 'preroll_paths' | 'preroll_mode' | 'theintrodb_api_key' | 'detect_intros' | 'remote_kbps' | 'upload_kbps' | 'public_url' | 'remote_access' | 'remote_port' | 'upnp' | 'https_mode' | 'https_cert' | 'https_key' | 'https_domain' | 'home_networks' | 'trusted_proxies' | 'transcode_dir' | 'max_transcodes'>> & { opensubtitles_password?: string }) =>
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
  markerCoverage: (libraryId: number) => request<{ skip_intros: boolean; skip_credits: boolean; titles: MarkerCoverage[] }>('GET', `/api/libraries/${libraryId}/markers`),
  collections: (libraryId: number) => request<Collection[]>('GET', `/api/libraries/${libraryId}/collections`),
  collection: (id: string) => request<{ collection: Collection; items: Item[] }>('GET', `/api/collections/${id}`),
  createCollection: (c: { library_id: number; kind: 'manual' | 'smart'; title: string; overview?: string; filter?: RowFilter; item_ids?: string[] }) =>
    request<Collection>('POST', '/api/collections', c),
  updateCollection: (id: string, c: { title?: string; sort_title?: string; overview?: string; filter?: RowFilter; locked_fields?: string[]; grid_mode?: Collection['grid_mode'] }) =>
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

  items: (q: ItemQuery) => request<{ total: number; items: Item[]; offset?: number; letters?: LetterCount[] }>('GET', `/api/items${query(q)}`),
  item: (id: string) => request<Item>('GET', `/api/items/${id}`),
  mergeItems: (targetId: string, itemIds: string[]) => request<void>('POST', '/api/items/merge', { target_id: targetId, item_ids: itemIds }),
  /** Unmatch a show: back to its file names, until Fix Match identifies it again (admin). */
  unmatchItem: (id: string) => request<Item>('POST', `/api/items/${id}/unmatch`),
  capabilities: () => request<Capabilities>('GET', '/api/capabilities'),
  createPlayQueue: (itemIds: string[], replace?: { id: string; revision: number }) =>
    request<PlayQueue>('POST', '/api/play-queues', { item_ids: itemIds, ...(replace && { replace_id: replace.id, revision: replace.revision }) }),
  currentPlayQueue: () => request<PlayQueue>('GET', '/api/play-queues/current'),
  playQueue: (id: string) => request<PlayQueue>('GET', `/api/play-queues/${encodeURIComponent(id)}`),
  editPlayQueue: (id: string, revision: number, edit: PlayQueueEdit) => request<PlayQueue>('PATCH', `/api/play-queues/${encodeURIComponent(id)}`, { revision, ...edit }),
  deletePlayQueue: (id: string, revision: number) => request<void>('DELETE', `/api/play-queues/${encodeURIComponent(id)}?revision=${revision}`),
  splitItem: (id: string) => request<{ item_ids: string[] }>('POST', `/api/items/${id}/split`),
  editItems: (edit: BulkEdit) => request<void>('POST', '/api/items/bulk', edit),
  editItem: (id: string, edit: ItemEdit) => request<Item>('PATCH', `/api/items/${id}`, edit),
  refreshItem: (id: string) => request<Item>('POST', `/api/items/${id}/refresh`),
  /** Rereads a title's files (every episode, for a show) in the background, like Plex's Analyze. */
  analyzeItem: (id: string) => request<ScanStatus>('POST', `/api/items/${id}/analyze`),
  reportProgress: (id: string, positionMS: number, durationMS: number, state: 'playing' | 'paused' | 'stopped' = 'playing') =>
    request<UserData>(
      'PUT',
      `/api/items/${id}/progress`,
      { position_ms: Math.round(positionMS), duration_ms: Math.round(durationMS), state },
      { keepalive: true },
    ),
  playing: () => request<Playback[]>('GET', '/api/playing'),
  webhooks: () => request<{ webhooks: Webhook[]; events: string[] }>('GET', '/api/webhooks'),
  saveWebhook: (h: { id?: string; name: string; url: string; events: string[] }) =>
    h.id ? request<Webhook>('PUT', `/api/webhooks/${h.id}`, h) : request<Webhook>('POST', '/api/webhooks', h),
  deleteWebhook: (id: string) => request<void>('DELETE', `/api/webhooks/${id}`),
  testWebhook: (id: string) => request<{ ok: boolean; status: string }>('POST', `/api/webhooks/${id}/test`),
  itemMarkers: (id: string) => request<{ file_id: number; duration_ms: number; markers: Marker[] }>('GET', `/api/items/${id}/markers`),
  setItemMarkers: (id: string, markers: { kind: string; start_ms: number; end_ms: number }[]) =>
    request<{ file_id: number; duration_ms: number; markers: Marker[] }>('PUT', `/api/items/${id}/markers`, { markers }),
  resetItemMarkers: (id: string) => request<void>('DELETE', `/api/items/${id}/markers`),
  optimize: (itemIds: string[], quality: number) => request<{ queued: number }>('POST', '/api/optimize', { item_ids: itemIds, quality }),
  optimized: () =>
    request<{
      total_size: number;
      versions: { file_id: number; quality: number; size: number; status: 'queued' | 'converting' | 'done' | 'failed'; error?: string; item_id: string; title: string; series_title?: string }[];
    }>('GET', '/api/optimized'),
  deleteOptimized: (fileId: number, quality: number) => request<void>('DELETE', `/api/optimized/${fileId}/${quality}`),
  preroll: (itemId: string) => request<{ title: string; url: string }[]>('GET', `/api/items/${itemId}/preroll`),
  news: () => request<{ entries: NewsEntry[]; unread: number }>('GET', '/api/news'),
  newsSeen: () => request<void>('POST', '/api/news/seen'),
  startImport: (b: { source: 'plex' } | { source: 'jellyfin'; url: string; username: string; password: string }) =>
    request<ImportSession>('POST', '/api/imports', b),
  importSession: (id: string) => request<ImportSession>('GET', `/api/imports/${id}`),
  chooseImportServer: (id: string, serverId: string) => request<ImportSession>('POST', `/api/imports/${id}/server`, { server_id: serverId }),
  runImport: (id: string, profiles: Record<string, string>) => request<ImportSession>('POST', `/api/imports/${id}/run`, { profiles }),
  maintenanceWindow: () => request<{ now: string; local_time: string; timezone: string; open: boolean; all_day: boolean; next_start?: string; next_local?: string }>('GET', '/api/maintenance/window'),
  startLibraryWork: (id: number, operation: 'refresh' | 'analyze') => request<ScanStatus>('POST', `/api/libraries/${id}/work`, { operation }),
  stopLibraryWork: (id: number, jobId: number) => request<ScanStatus>('POST', `/api/libraries/${id}/stop`, { job_id: jobId }),
  stopActivity: (id: number) => request<void>('POST', `/api/activity/${id}/stop`),
  maintenance: () => request<ScheduledTask[]>('GET', '/api/maintenance'),
  runMaintenance: (kind: string) => request<{ queued: boolean }>('POST', '/api/maintenance/' + kind + '/run'),
  /** The server's name and version (anyone may ask). */
  serverInfo: () => request<{ name: string; version: string; api_version: number }>('GET', '/api/health'),
  activity: () => request<{ running: ActivityTask[]; recent: ActivityTask[]; scheduled?: ScheduledTask[] }>('GET', '/api/activity'),
  stopPlaying: (id: string, message: string) => request<void>('POST', `/api/playing/${id}/stop`, { message }),
  playbackOptions: (itemId: string, body: { caps: PlayerCaps; file_id?: number; edition?: string }) =>
    request<PlaybackOptions>('POST', `/api/items/${itemId}/playback-options`, body),
  startPlayback: (
    itemId: string,
    body: { caps: PlayerCaps; file_id?: number; audio_stream?: number; start_ms?: number; burn_subtitle?: string; edition?: string; max_kbps?: number; rolling_hls?: boolean },
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
  /** A link to a stream that a Chromecast or Apple TV can open itself. */
  approveLink: (code: string) => request<{ device: string }>('POST', '/api/auth/link/approve', { code }),
  createToolToken: (name: string) => request<{ token: string }>('POST', '/api/v1/devices/tools', { name }),
  devices: () => request<Device[]>('GET', '/api/devices'),
  signOutDevice: (id: string) => request<void>('DELETE', `/api/devices/${id}`),
  castLink: (url: string) => request<{ url: string; local_only: boolean }>('POST', '/api/cast', { url }),
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
  createSmartPlaylist: (title: string, filter: RowFilter) => request<Playlist>('POST', '/api/playlists', { title, filter }),
  updateSmartPlaylist: (id: string, title: string, filter: RowFilter) => request<void>('PATCH', `/api/playlists/${id}`, { title, filter }),
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
  recommended: (libraryId: number) => request<{ rows: RecommendedRow[] }>('GET', `/api/libraries/${libraryId}/recommended`),
  recommendedLayout: (id: number) => request<{ rows: RecommendedLook[] }>('GET', `/api/libraries/${id}/recommended/layout`),
  saveRecommendedLayout: (id: number, rows: RecommendedLook[]) => request<{ rows: RecommendedLook[] }>('PUT', `/api/libraries/${id}/recommended/layout`, { rows }),
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
  /** For a show the answer comes once it's identified; `match_task_id` is the Activity task fetching the rest. */
  setMatch: (id: string, provider: Provider, matchId: number) =>
    request<Item & { match_task_id?: number }>('PUT', `/api/items/${id}/match`, { provider, id: matchId }),

  seasons: (seriesId: string, opts: { ordering?: number; missing?: boolean } = {}) =>
    request<SeasonsResponse>(
      'GET',
      `/api/series/${seriesId}/seasons${query({ ordering: opts.ordering, missing: opts.missing ? 1 : undefined })}`,
    ),
  orderings: (seriesId: string) => request<OrderingsResponse>('GET', `/api/series/${seriesId}/orderings`),
  setInlineSpecials: (seriesId: string, inline: boolean | null) =>
    request<OrderingsResponse>('PATCH', `/api/series/${seriesId}/specials`, { inline_specials: inline }),
  setDisplayOrdering: (seriesId: string, orderingId: number | null) =>
    request<OrderingsResponse>('PUT', `/api/series/${seriesId}/display-ordering`, { ordering_id: orderingId }),
  refreshSeries: (seriesId: string) => request<OrderingsResponse>('POST', `/api/series/${seriesId}/refresh`),

  backups: () => request<BackupsResponse>('GET', '/api/backups'),
  createBackup: () => request<Backup>('POST', '/api/backups'),
  restoreBackup: (name: string) => request<void>('POST', `/api/backups/${encodeURIComponent(name)}/restore`),
  deleteBackup: (name: string) => request<void>('DELETE', `/api/backups/${encodeURIComponent(name)}`),
  libraryFolders: (libraryId: number, path: string) =>
    request<{ path: string; trail: { name: string; path: string }[]; folders: { name: string; path: string; count: number; cover_id?: string; cover?: string }[]; items: Item[] }>(
      'GET', `/api/libraries/${libraryId}/folders${path ? `?path=${encodeURIComponent(path)}` : ''}`),
  emptyTrash: (libraryId: number) => request<{ removed: number }>('POST', `/api/libraries/${libraryId}/empty-trash`),
  emptyTrashSafely: (id: number) => request<{ removed: number; skipped?: 'busy' | 'scan_required' }>('POST', `/api/libraries/${id}/empty-trash?safe=1`),
  removeItem: (id: string) => request<void>('DELETE', `/api/items/${id}`),
};

export function posterUrl(item: Item): string | undefined {
  return item.images?.primary;
}

/** The backdrop for a full-width banner: full resolution when the provider has it. */
export function backdropUrl(item: Item): string | undefined {
  return item.images?.backdrop_full ?? item.images?.backdrop;
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

/** A group of a library's titles: automatic (a TMDB collection), manual, or smart (a filter). */
export interface Collection {
  id: string;
  library_id: number;
  /** folder: a folder of an Other videos library. */
  kind: 'auto' | 'manual' | 'smart' | 'folder';
  tmdb_id?: number;
  title: string;
  sort_title: string;
  overview?: string;
  /** Fields edited by hand, which TMDB refreshes leave alone. */
  locked_fields: string[];
  count: number;
  images: { primary?: string; backdrop?: string; backdrop_full?: string };
  smart_filter?: RowFilter;
  /** How it shows in its library's grid: "" as the library says. */
  grid_mode: '' | CollectionMode;
  /** Listing a library: its titles, when it shows in the grid in their place or beside them. */
  item_ids?: string[];
}

/** Plex's collection mode: the titles, the collection in their place, or both. */
export type CollectionMode = 'items' | 'collection' | 'both';

/** A profile's playlist of movies and episodes. */
export interface Playlist {
  id: string;
  title: string;
  count: number;
  duration_ms: number;
  updated_at: number;
  /** The first few titles' posters. */
  images?: string[];
  /** A smart playlist's rules: it's kept up to date from them. */
  smart_filter?: RowFilter;
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
/** Where a file's intro, recap or credits are, for Skip Intro and Skip Credits. */
export interface Marker {
  kind: 'intro' | 'recap' | 'credits';
  start_ms: number;
  end_ms: number;
  source: 'manual' | 'chapters' | 'theintrodb' | 'introdb' | 'detected';
}

export interface Chapter {
  start_ms: number;
  end_ms: number;
  title?: string;
}
