package org.arcmediaserver.tv

import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.arcmediaserver.tv.data.ApiException
import org.arcmediaserver.tv.data.ArcApi
import org.arcmediaserver.tv.data.ArcJson
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.data.LinkPoll
import org.arcmediaserver.tv.data.PlaybackRequest
import org.arcmediaserver.tv.data.PlayerCaps
import org.arcmediaserver.tv.data.ProgressReport
import org.arcmediaserver.tv.data.resolveHomeRows
import org.arcmediaserver.tv.ui.browse.libraryQuery
import org.arcmediaserver.tv.ui.browse.loadRow
import org.arcmediaserver.tv.ui.details.nextEpisode
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test

/**
 * The app's API client against a real Arc, end to end: TV sign-in code, profile with PIN, Home,
 * a title, a show's seasons, playback decisions, the HLS playlist, progress and the session's end.
 *
 * Skipped unless ARC_E2E_URL (a scratch Arc with the test library, never anyone's real server)
 * and ARC_E2E_PASSWORD (its household password, to approve the code as the admin) are set; a
 * profile named "Sam" with PIN 2468 must exist. See README.md, "Testing against a real Arc".
 */
class LiveServerTest {
    private val url: String = System.getenv("ARC_E2E_URL").orEmpty()
    private val password: String = System.getenv("ARC_E2E_PASSWORD").orEmpty()
    private val http = OkHttpClient()
    private val json = "application/json".toMediaType()

    private fun step(s: String) = println("E2E: $s")

    /** What the browser does: sign in as the admin and approve the code shown on the TV. */
    private fun approveAsAdmin(code: String) {
        fun post(path: String, body: String, token: String?): Pair<Int, String> {
            val req = Request.Builder().url("${url.trimEnd('/')}/api/v1/$path").post(body.toRequestBody(json))
            token?.let { req.header("Authorization", "Bearer $it") }
            return http.newCall(req.build()).execute().use { it.code to it.body.string() }
        }
        val (_, login) = post("auth/login", """{"password":"$password","device":"E2E browser"}""", null)
        val admin = ArcJson.decodeFromString(JsonObject.serializer(), login)["token"]!!.jsonPrimitive.content
        val status = ArcApi(url.toHttpUrl(), http, token = { admin }, log = ArcLog.Print)
        val adminId = runBlocking { status.authStatus() }.profiles.first { it.isAdmin }.id
        assertEquals(200, post("auth/profile", """{"id":"$adminId"}""", admin).first)
        val (code2, body) = post("auth/link/approve", """{"code":"$code"}""", admin)
        assertTrue("approve answered $code2 $body", code2 in 200..299)
    }

    @Test
    fun signInBrowseAndPlay() = runBlocking {
        assumeTrue("set ARC_E2E_URL and ARC_E2E_PASSWORD to run against a real Arc", url.isNotEmpty() && password.isNotEmpty())
        var token: String? = null
        val api = ArcApi(url.toHttpUrl(), http, token = { token }, log = ArcLog.Print)

        val health = api.health()
        step("health: ${health.name} ${health.version}, api_version ${health.apiVersion}")
        assertEquals(1, health.apiVersion)

        // TV sign-in code.
        val link = api.startLink("E2E Android TV")
        step("sign-in code ${link.code} (expires in ${link.expiresIn}s)")
        assertEquals(LinkPoll.Waiting, api.pollLink(link.poll))
        approveAsAdmin(link.code)
        val approved = api.pollLink(link.poll)
        assertTrue("poll answered $approved", approved is LinkPoll.Approved)
        token = (approved as LinkPoll.Approved).token
        step("approved; device token received")
        assertEquals(LinkPoll.Expired, api.pollLink(link.poll)) // answered once only

        // Profiles: a wrong PIN, then the right one.
        val status = api.authStatus()
        assertTrue(status.signedIn)
        assertNull(status.profile)
        step("profiles: ${status.profiles.joinToString { it.name + if (it.hasPin) " (PIN)" else "" }}")
        val sam = status.profiles.first { it.name == "Sam" }
        assertTrue(sam.hasPin)
        try {
            api.chooseProfile(sam.id, "1111")
            error("a wrong PIN was accepted")
        } catch (e: ApiException) {
            assertEquals("pin", e.needs)
            step("wrong PIN refused: ${e.status} ${e.message}")
        }
        api.chooseProfile(sam.id, "2468")
        assertEquals(sam.id, api.authStatus().profile?.id)
        step("using profile ${sam.name}")

        // Wait for the first scan.
        var libraries = api.libraries()
        repeat(60) {
            if (libraries.isNotEmpty() && libraries.all { it.count > 0 }) return@repeat
            delay(1_000)
            libraries = api.libraries()
        }
        step("libraries: ${libraries.joinToString { "${it.name} (${it.kind}, ${it.count})" }}")
        val movies = libraries.first { it.kind == "movies" }
        val shows = libraries.first { it.kind == "shows" }

        // Home rows as the app resolves and loads them.
        val prefs = api.prefs()
        val rows = resolveHomeRows(prefs, libraries)
        for (row in rows) step("home row ${row.id}: ${loadRow(api, row).size} cards")
        assertTrue(rows.any { it.id == "recent:${movies.id}" })

        // A movie and a show.
        val movieList = api.items(libraryQuery(movies, sort = "title", limit = 60)).items
        step("movies: ${movieList.joinToString { it.title }}")
        val showList = api.items(libraryQuery(shows, sort = "title", limit = 60)).items
        step("shows: ${showList.joinToString { it.title }}")
        val firefly = showList.first { it.title.contains("Firefly") }
        val seasons = api.seasons(firefly.id)
        step("Firefly in its display order: ${seasons.seasons.joinToString { "S${it.seasonNumber}: ${it.episodes.size}" }}, ${seasons.unplaced.size} unplaced")
        val episode = nextEpisode(seasons.seasons.filter { it.episodes.isNotEmpty() })
        assertNotNull(episode)

        val movie = movieList.first { it.title.contains("Matrix") }
        val detail = api.item(movie.id)
        step("item ${detail.title}: ${detail.files.size} files, ${detail.durationMs} ms")

        // Playback: the file as it is for a player that takes MP4 and H.264 ...
        val caps = PlayerCaps(listOf("mp4", "mkv", "webm", "mpegts"), listOf("h264", "hevc"), listOf("aac", "mp3", "ac3", "eac3"), 1080)
        val direct = api.startPlayback(movie.id, PlaybackRequest(caps))
        step("${movie.title}: ${direct.method} ${direct.reasons} ${direct.url}")
        assertEquals("direct", direct.method)
        val head = http.newCall(Request.Builder().url(api.absolute(direct.url)).header("Authorization", "Bearer $token").header("Range", "bytes=0-1023").build()).execute()
        head.use { assertTrue("stream answered ${it.code}", it.code == 206 || it.code == 200) }

        // ... and a converted stream, part-way in, for a player that can't open the container.
        val noContainers = caps.copy(containers = emptyList())
        val hls = api.startPlayback(episode!!.id, PlaybackRequest(noContainers, startMs = 1_000))
        step("${episode.title}: ${hls.method} ${hls.reasons} hls=${hls.hls} offset_ms=${hls.offsetMs} session=${hls.sessionId != null}")
        assertTrue(hls.hls)
        assertNotNull(hls.sessionId)
        var manifest = ""
        repeat(30) {
            val res = http.newCall(Request.Builder().url(api.absolute(hls.url)).header("Authorization", "Bearer $token").build()).execute()
            res.use { if (it.code == 200) manifest = it.body.string() }
            if (manifest.contains("#EXTINF")) return@repeat
            delay(500)
        }
        step("HLS playlist: ${manifest.lines().count { it.startsWith("#EXTINF") }} segments so far")
        assertTrue(manifest.startsWith("#EXTM3U"))
        // A segment, resolved against the playlist's address as the player does.
        val segment = manifest.lines().firstOrNull { it.isNotBlank() && !it.startsWith("#") }
        if (segment != null) {
            val segUrl = api.absolute(hls.url).toHttpUrl().resolve(segment)!!
            http.newCall(Request.Builder().url(segUrl).header("Authorization", "Bearer $token").build()).execute().use {
                assertEquals(200, it.code)
                step("first segment: ${it.body.bytes().size} bytes")
            }
        }

        // The test library's files are a few seconds long: play to the end, as the player reports it.
        api.reportProgress(episode.id, ProgressReport(hls.durationMs / 2, hls.durationMs, "playing"))
        api.pingTranscode(hls.sessionId!!)
        api.reportProgress(episode.id, ProgressReport(hls.durationMs, hls.durationMs, "stopped"))
        api.stopTranscode(hls.sessionId)
        step("progress saved, session pinged and ended")
        val after = api.item(episode.id)
        step("${after.title} watched: ${after.userData?.played}")
        assertEquals(true, after.userData?.played)
        val next = nextEpisode(api.seasons(firefly.id).seasons.filter { it.episodes.isNotEmpty() })
        step("Play on the show page now starts: ${next?.title}")
        assertTrue(next?.id != episode.id)
        val cont = api.continueWatching(withUpNext = true)
        step("Continue Watching (with Up Next): ${cont.joinToString { it.title }}")
    }
}
