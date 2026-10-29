package org.arcmediaserver.tv.playback

import org.arcmediaserver.tv.data.PlaybackPlan
import org.arcmediaserver.tv.data.PlayerCaps
import org.arcmediaserver.tv.data.SubtitleTrack
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackTest {
    private val direct = PlaybackPlan(method = "direct", url = "/api/items/m1/stream?file=1", durationMs = 6_000_000)
    private val converted = PlaybackPlan(method = "transcode", url = "/api/transcode/s/index.m3u8", hls = true, sessionId = "s", offsetMs = 600_000, durationMs = 6_000_000)

    @Test
    fun `a converted stream's clock starts at offset_ms`() {
        assertEquals(615_000, PlaybackClock.filePosition(converted, 15_000))
        assertEquals(15_000, PlaybackClock.filePosition(direct, 15_000))
        assertEquals(5_000, PlaybackClock.playerStart(converted, 605_000))
        // A remux can begin at a keyframe before the point asked for.
        assertEquals(0, PlaybackClock.playerStart(converted.copy(offsetMs = 610_000), 605_000))
        assertEquals(605_000, PlaybackClock.playerStart(direct, 605_000))
    }

    @Test
    fun `seeks stay in the stream only within what's converted`() {
        assertTrue(PlaybackClock.seekInStream(direct, 5_000_000, 0))
        assertTrue(PlaybackClock.seekInStream(converted, 700_000, 300_000))
        assertFalse(PlaybackClock.seekInStream(converted, 500_000, 300_000)) // before the stream starts
        assertFalse(PlaybackClock.seekInStream(converted, 950_000, 300_000)) // beyond what's converted
    }

    @Test
    fun `caps come from decoders, passthrough and the display`() {
        val caps = capsFrom(setOf("video/avc", "video/hevc", "audio/mp4a-latm", "audio/opus", "audio/flac", "audio/mpeg"), setOf("ac3", "eac3"), 2160)
        assertEquals(listOf("h264", "hevc"), caps.video)
        assertEquals(listOf("aac", "mp3", "opus", "flac", "ac3", "eac3"), caps.audio)
        assertEquals(listOf("mp4", "mkv", "webm", "mpegts"), caps.containers)
        assertEquals(2160, caps.maxHeight)
    }

    @Test
    fun `after a failure Arc is asked to do more`() {
        val caps = PlayerCaps(listOf("mp4", "mkv"), listOf("h264"), listOf("aac"), 1080)
        assertEquals(caps.copy(containers = emptyList()), fallbackCaps(caps, "direct"))
        assertEquals(caps.copy(containers = emptyList(), video = emptyList()), fallbackCaps(caps, "remux"))
    }

    private fun sub(id: String, lang: String?, forced: Boolean = false, sdh: Boolean = false, image: Boolean = false) =
        SubtitleTrack(id = id, language = lang, forced = forced, hearingImpaired = sdh, image = image)

    @Test
    fun `subtitles start in the profile's language, else forced ones for the audio`() {
        val tracks = listOf(sub("1", "eng", forced = true), sub("2", "eng", sdh = true), sub("3", "en"), sub("4", "fre"), sub("5", "jpn", image = true))
        assertEquals("3", chooseSubtitle(tracks, "eng", "jpn")?.id)
        assertEquals("4", chooseSubtitle(tracks, "fr", "eng")?.id)
        assertEquals("1", chooseSubtitle(tracks, "", "en")?.id)
        assertNull(chooseSubtitle(tracks, "", "jpn"))
        // Picture subtitles can only be burned in, which the app doesn't ask for yet.
        assertNull(chooseSubtitle(tracks, "jpn", "eng").takeIf { it?.id == "5" })
    }

    @Test
    fun `languages match by language, not spelling`() {
        assertTrue(sameLanguage("en", "eng"))
        assertTrue(sameLanguage("en-GB", "eng"))
        assertFalse(sameLanguage("en", "fr"))
        assertFalse(sameLanguage(null, "en"))
    }
}
