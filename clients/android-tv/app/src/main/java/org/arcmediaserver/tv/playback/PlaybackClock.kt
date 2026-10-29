package org.arcmediaserver.tv.playback

import org.arcmediaserver.tv.data.PlaybackPlan
import org.arcmediaserver.tv.data.SubtitleTrack
import java.util.Locale

/**
 * Converted streams start where playback was asked to start, not at the file's beginning: Arc
 * says where in `offset_ms`, and the player's clock counts from there (docs/API.md "Playing";
 * web/src/components/Player.tsx). These turn one clock into the other.
 */
object PlaybackClock {
    /** Where in the file the player is. */
    fun filePosition(plan: PlaybackPlan, playerMs: Long): Long = if (plan.hls) plan.offsetMs + playerMs else playerMs

    /** Where the player should start for a point in the file (a remux may begin at an earlier keyframe). */
    fun playerStart(plan: PlaybackPlan, fileMs: Long): Long = if (plan.hls) (fileMs - plan.offsetMs).coerceAtLeast(0) else fileMs

    /**
     * Whether a seek to [fileMs] can stay in the current stream: always for the file as it is; for a
     * converted stream, only within what's been converted so far ([availableMs] of it). Anything
     * else asks Arc for a new stream starting there, as the web player does.
     */
    fun seekInStream(plan: PlaybackPlan, fileMs: Long, availableMs: Long): Boolean {
        if (!plan.hls) return true
        val rel = fileMs - plan.offsetMs
        return rel >= 0 && rel <= availableMs - 2_000
    }
}

/** Language codes compared by language: "en", "eng" and "en-GB" match. */
fun sameLanguage(a: String?, b: String?): Boolean {
    if (a.isNullOrBlank() || b.isNullOrBlank()) return false
    fun key(code: String): String {
        val base = code.substringBefore('-').substringBefore('_').lowercase()
        return runCatching { Locale.forLanguageTag(base).isO3Language }.getOrNull()?.takeIf { it.isNotEmpty() } ?: base
    }
    return key(a) == key(b)
}

/**
 * The subtitles to start with (web/src/lib/playback.ts chooseSubtitle, default mode): the profile's
 * last language as full subtitles, or else forced ones for the audio's language. Picture formats
 * are left out: Arc can only burn them in, which this app doesn't ask for yet.
 */
fun chooseSubtitle(tracks: List<SubtitleTrack>, preferred: String, audioLanguage: String?): SubtitleTrack? {
    val text = tracks.filter { !it.image }
    if (preferred.isNotBlank()) {
        val matching = text.filter { sameLanguage(it.language, preferred) }
        (matching.firstOrNull { !it.forced && !it.hearingImpaired } ?: matching.firstOrNull { !it.forced } ?: matching.firstOrNull())?.let { return it }
    }
    return text.firstOrNull { it.forced && (it.language.isNullOrBlank() || audioLanguage.isNullOrBlank() || sameLanguage(it.language, audioLanguage)) }
}
