package org.arcmediaserver.tv.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject

// The parts of Arc's /api/v1 answers this app uses. Everything else is ignored, as the v1 promise
// asks ("new fields may appear, so ignore what you don't know"); see docs/API.md.

@Serializable
data class Health(
    val name: String = "",
    val version: String = "",
    @SerialName("api_version") val apiVersion: Int = 0,
)

@Serializable
data class LinkStart(val code: String, val poll: String, @SerialName("expires_in") val expiresIn: Int = 600)

@Serializable
data class TokenAnswer(val token: String)

@Serializable
data class Profile(
    val id: String,
    val name: String,
    val color: String = "violet",
    @SerialName("has_pin") val hasPin: Boolean = false,
    @SerialName("is_admin") val isAdmin: Boolean = false,
)

@Serializable
data class AuthStatus(
    @SerialName("signed_in") val signedIn: Boolean = false,
    @SerialName("password_session") val passwordSession: Boolean = false,
    val friend: Boolean = false,
    val profile: Profile? = null,
    val profiles: List<Profile> = emptyList(),
)

@Serializable
data class Library(
    val id: Long,
    val name: String,
    /** movies, shows or videos (Other videos). */
    val kind: String,
    val count: Int = 0,
)

@Serializable
data class UserData(
    @SerialName("position_ms") val positionMs: Long = 0,
    val played: Boolean = false,
)

@Serializable
data class Images(
    val primary: String? = null,
    val backdrop: String? = null,
    @SerialName("backdrop_full") val backdropFull: String? = null,
    val logo: String? = null,
    @SerialName("series_primary") val seriesPrimary: String? = null,
    @SerialName("series_backdrop") val seriesBackdrop: String? = null,
    @SerialName("series_logo") val seriesLogo: String? = null,
    @SerialName("season_primary") val seasonPrimary: String? = null,
)

@Serializable
data class MediaFile(
    val id: Long,
    @SerialName("duration_ms") val durationMs: Long = 0,
    val missing: Boolean = false,
)

@Serializable
data class Item(
    val id: String,
    @SerialName("library_id") val libraryId: Long = 0,
    /** movie, series or episode. */
    val kind: String,
    @SerialName("parent_id") val parentId: String? = null,
    val title: String,
    val year: Int? = null,
    val overview: String? = null,
    val tagline: String? = null,
    @SerialName("runtime_min") val runtimeMin: Int? = null,
    @SerialName("content_rating") val contentRating: String? = null,
    val rating: Double? = null,
    val genres: List<String> = emptyList(),
    @SerialName("series_title") val seriesTitle: String? = null,
    @SerialName("season_number") val seasonNumber: Int? = null,
    @SerialName("episode_number") val episodeNumber: Int? = null,
    @SerialName("user_data") val userData: UserData? = null,
    @SerialName("unwatched_count") val unwatchedCount: Int? = null,
    val images: Images? = null,
    val files: List<MediaFile> = emptyList(),
    /** Only on episodes in a season list: whether a file is on disk. */
    val available: Boolean = true,
) {
    val isEpisode get() = kind == "episode"
    val isSeries get() = kind == "series"
    val playable get() = kind == "movie" || kind == "episode"

    /** How long it runs, as well as Arc knows: the file's length, or the listed runtime. */
    val durationMs: Long
        get() = files.firstOrNull { !it.missing && it.durationMs > 0 }?.durationMs ?: ((runtimeMin ?: 0) * 60_000L)

    /** A resume point, if the profile is part-way through. */
    val resumeMs: Long get() = userData?.takeIf { !it.played }?.positionMs ?: 0
}

@Serializable
data class ItemPage(val total: Int = 0, val items: List<Item> = emptyList())

@Serializable
data class RecentCard(
    /** series, season or episode. */
    val type: String,
    val item: Item,
    @SerialName("season_number") val seasonNumber: Int? = null,
    val episodes: Int = 0,
    @SerialName("season_poster") val seasonPoster: String? = null,
)

@Serializable
data class HomeRowPref(
    val id: String,
    val hidden: Boolean = false,
    val title: String? = null,
    /** A custom row's browse filter: passed to /items as it is. */
    val filter: JsonObject? = null,
    val style: String? = null,
    @SerialName("shows_only") val showsOnly: Boolean = false,
)

@Serializable
data class Prefs(
    val home: List<HomeRowPref> = emptyList(),
    @SerialName("split_up_next") val splitUpNext: Boolean = false,
    @SerialName("library_order") val libraryOrder: List<Long> = emptyList(),
    @SerialName("subtitle_language") val subtitleLanguage: String = "",
    @SerialName("hide_spoilers") val hideSpoilers: Boolean = false,
)

@Serializable
data class Ordering(val id: Long, val name: String = "")

@Serializable
data class Season(
    @SerialName("season_number") val seasonNumber: Int,
    val name: String = "",
    val overview: String? = null,
    val poster: String? = null,
    val episodes: List<Item> = emptyList(),
)

@Serializable
data class SeasonsResponse(
    val series: Item,
    val ordering: Ordering? = null,
    val seasons: List<Season> = emptyList(),
    val unplaced: List<Item> = emptyList(),
)

/** What the player can play, as Arc's playback decision expects it (internal/api/playback.go). */
@Serializable
data class PlayerCaps(
    val containers: List<String>,
    val video: List<String>,
    val audio: List<String>,
    @SerialName("max_height") val maxHeight: Int,
)

@Serializable
data class PlaybackRequest(
    val caps: PlayerCaps,
    @SerialName("start_ms") val startMs: Long = 0,
    @SerialName("file_id") val fileId: Long? = null,
    @SerialName("audio_stream") val audioStream: Int? = null,
    /** A quality limit chosen in the player (kbit/s); null for the original. */
    @SerialName("max_kbps") val maxKbps: Int? = null,
)

@Serializable
data class AudioTrack(
    val index: Int,
    val language: String? = null,
    val title: String? = null,
    val codec: String = "",
    val channels: Int = 0,
    val default: Boolean = false,
)

@Serializable
data class SubtitleTrack(
    val id: String,
    val language: String? = null,
    val title: String? = null,
    val codec: String = "",
    val forced: Boolean = false,
    val default: Boolean = false,
    @SerialName("hearing_impaired") val hearingImpaired: Boolean = false,
    /** A picture format (Blu-ray, DVD): Arc can only burn these in. */
    val image: Boolean = false,
)

@Serializable
data class PlaybackPlan(
    /** direct, remux or transcode. */
    val method: String,
    val reasons: List<String> = emptyList(),
    val url: String,
    val hls: Boolean = false,
    @SerialName("session_id") val sessionId: String? = null,
    /** Where a converted stream starts in the file: add it to the player's clock. */
    @SerialName("offset_ms") val offsetMs: Long = 0,
    @SerialName("duration_ms") val durationMs: Long = 0,
    @SerialName("file_id") val fileId: Long = 0,
    @SerialName("audio_stream") val audioStream: Int = -1,
    @SerialName("audio_tracks") val audioTracks: List<AudioTrack> = emptyList(),
    @SerialName("subtitle_tracks") val subtitleTracks: List<SubtitleTrack> = emptyList(),
    val encoder: String? = null,
    @SerialName("tone_mapped") val toneMapped: Boolean = false,
    /** The title's files: editions and versions (e.g. 4K and 1080p). */
    val versions: List<Version> = emptyList(),
    val chapters: List<Chapter> = emptyList(),
    /** Intro, recap and credits, for Skip Intro and Skip Credits (times in the file). */
    val markers: List<Marker> = emptyList(),
    @SerialName("source_kbps") val sourceKbps: Int = 0,
    @SerialName("source_height") val sourceHeight: Int = 0,
    /** The ceiling away from home (0 at home or with no limit). */
    @SerialName("server_limit_kbps") val serverLimitKbps: Int = 0,
)

@Serializable
data class Version(
    @SerialName("file_id") val fileId: Long,
    val edition: String? = null,
    val width: Int = 0,
    val height: Int = 0,
    @SerialName("video_codec") val videoCodec: String? = null,
    val hdr: String? = null,
    val size: Long = 0,
)

@Serializable
data class Chapter(@SerialName("start_ms") val startMs: Long, @SerialName("end_ms") val endMs: Long = 0, val title: String? = null)

@Serializable
data class Marker(
    /** intro, recap or credits. */
    val kind: String,
    @SerialName("start_ms") val startMs: Long,
    @SerialName("end_ms") val endMs: Long,
)

@Serializable
data class ProgressReport(
    @SerialName("position_ms") val positionMs: Long,
    @SerialName("duration_ms") val durationMs: Long,
    /** playing, paused or stopped. */
    val state: String,
)
