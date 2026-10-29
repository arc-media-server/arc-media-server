package org.arcmediaserver.tv.playback

import android.content.Context
import android.hardware.display.DisplayManager
import android.media.MediaCodecList
import android.view.Display
import androidx.annotation.OptIn
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.audio.AudioCapabilities
import org.arcmediaserver.tv.data.PlayerCaps

/** Arc's codec names (as ffprobe calls them) and the decoder MIME types that play them. */
internal val videoMimes = linkedMapOf(
    "h264" to "video/avc",
    "hevc" to "video/hevc",
    "vp9" to "video/x-vnd.on2.vp9",
    "av1" to "video/av01",
    "mpeg2video" to "video/mpeg2",
    "mpeg4" to "video/mp4v-es",
)

internal val audioMimes = linkedMapOf(
    "aac" to "audio/mp4a-latm",
    "mp3" to "audio/mpeg",
    "opus" to "audio/opus",
    "flac" to "audio/flac",
    "vorbis" to "audio/vorbis",
    "ac3" to "audio/ac3",
    "eac3" to "audio/eac3",
    "dts" to "audio/vnd.dts",
    "truehd" to "audio/true-hd",
)

/** Formats the TV can send on to a receiver or soundbar as they are (HDMI passthrough). */
internal val passthroughEncodings = mapOf(
    "ac3" to C.ENCODING_AC3,
    "eac3" to C.ENCODING_E_AC3,
    "dts" to C.ENCODING_DTS,
    "truehd" to C.ENCODING_DOLBY_TRUEHD,
)

/**
 * What this device can play, worked out from what it has rather than a list per model:
 * the decoders Android lists, the sound formats the TV or receiver takes by passthrough, and the
 * tallest picture the display can show.
 *
 * Containers are the ones Media3 opens itself (MP4, Matroska/WebM, MPEG-TS), so such files play
 * as they are, which is what Plex does on Android TV too.
 */
fun capsFrom(decoders: Set<String>, passthrough: Set<String>, maxHeight: Int): PlayerCaps = PlayerCaps(
    containers = listOf("mp4", "mkv", "webm", "mpegts"),
    video = videoMimes.filterValues { it in decoders }.keys.toList(),
    audio = audioMimes.filter { (name, mime) -> mime in decoders || name in passthrough }.keys.toList(),
    maxHeight = maxHeight,
)

/**
 * Caps for trying again after the player failed: a file played as it is failed, so have it
 * repackaged; repackaged failed, so have the video converted too (web/src/lib/playback.ts).
 */
fun fallbackCaps(caps: PlayerCaps, failedMethod: String): PlayerCaps =
    if (failedMethod == "direct") caps.copy(containers = emptyList()) else caps.copy(containers = emptyList(), video = emptyList())

@OptIn(UnstableApi::class)
fun deviceCaps(context: Context): PlayerCaps {
    val decoders = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos
        .filter { !it.isEncoder }
        .flatMap { it.supportedTypes.map(String::lowercase) }
        .toSet()
    // Deprecated in Media3 1.11 in favour of the player's own audio output handling, but it is
    // still the way to ask, before playing, what the TV or receiver takes by passthrough.
    @Suppress("DEPRECATION")
    val audio = AudioCapabilities.getCapabilities(context, AudioAttributes.DEFAULT, null)
    val passthrough = passthroughEncodings.filterValues { audio.supportsEncoding(it) }.keys
    return capsFrom(decoders, passthrough, displayHeight(context))
}

/** The tallest mode the display offers (a 4K TV often runs its menus at 1080p). */
private fun displayHeight(context: Context): Int {
    val display = context.getSystemService(DisplayManager::class.java)?.getDisplay(Display.DEFAULT_DISPLAY)
        ?: return 1080
    val modes = display.supportedModes.map { minOf(it.physicalHeight, it.physicalWidth) }
    return (modes + minOf(display.mode.physicalHeight, display.mode.physicalWidth)).maxOrNull()?.takeIf { it > 0 } ?: 1080
}
