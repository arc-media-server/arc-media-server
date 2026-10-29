package org.arcmediaserver.tv.data

import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import java.io.IOException

/** The API client against a stand-in server: paths, the token, answers and failures. */
class ArcApiTest {
    private val server = MockWebServer()
    private var token: String? = "secret-token"
    private lateinit var api: ArcApi

    @Before
    fun setUp() {
        server.start()
        api = ArcApi(server.url("/"), OkHttpClient(), token = { token }, log = ArcLog.Print)
    }

    @After
    fun tearDown() = server.close()

    private fun answer(code: Int, body: String) = server.enqueue(MockResponse.Builder().code(code).body(body).build())

    @Test
    fun `calls go to api v1 with the device token`() = runTest {
        answer(200, """{"name":"Arc Media Server","version":"0.9.0","api_version":1,"status":"ok","new_field":[1,2]}""")
        val health = api.health()
        assertEquals(1, health.apiVersion)
        assertEquals("0.9.0", health.version)
        val req = server.takeRequest()
        assertEquals("/api/v1/health", req.url.encodedPath)
        assertEquals("Bearer secret-token", req.headers["Authorization"])
    }

    @Test
    fun `no token, no Authorization header`() = runTest {
        token = null
        answer(200, """{"api_version":1}""")
        api.health()
        assertNull(server.takeRequest().headers["Authorization"])
    }

    @Test
    fun `TV sign-in code polling`() = runTest {
        answer(200, """{"code":"ABCD","poll":"p0ll","expires_in":600}""")
        val link = api.startLink("Living Room TV")
        assertEquals("ABCD", link.code)
        val req = server.takeRequest()
        assertEquals("POST", req.method)
        assertEquals("""{"device":"Living Room TV"}""", req.body?.utf8())

        answer(202, """{"status":"waiting"}""")
        assertEquals(LinkPoll.Waiting, api.pollLink(link.poll))
        assertEquals("/api/v1/auth/link/p0ll", server.takeRequest().url.encodedPath)
        answer(200, """{"token":"device-token"}""")
        assertEquals(LinkPoll.Approved("device-token"), api.pollLink(link.poll))
        answer(404, """{"error":"this code has run out; ask for a new one"}""")
        assertEquals(LinkPoll.Expired, api.pollLink(link.poll))
    }

    @Test
    fun `errors carry Arc's words and what it needs`() = runTest {
        answer(401, """{"error":"wrong PIN","needs":"pin"}""")
        try {
            api.chooseProfile("p1", "1234")
            fail("expected an error")
        } catch (e: ApiException) {
            assertEquals(401, e.status)
            assertEquals("wrong PIN", e.message)
            assertEquals("pin", e.needs)
        }
        assertEquals("""{"id":"p1","pin":"1234"}""", server.takeRequest().body?.utf8())
    }

    @Test
    fun `an answer that isn't JSON is an IOException, not a crash`() = runTest {
        answer(200, "<html>not arc</html>")
        try {
            api.health()
            fail("expected an error")
        } catch (e: IOException) {
            assertTrue(e !is ApiException)
        }
    }

    @Test
    fun `playback decision sends the caps in the API's shape`() = runTest {
        answer(
            200,
            """{"method":"transcode","reasons":["DTS audio"],"url":"/api/transcode/s1/index.m3u8","hls":true,"session_id":"s1",
               "offset_ms":120000,"duration_ms":5400000,"file_id":7,"audio_stream":1,"audio_tracks":[{"index":1,"codec":"dts","channels":6,"default":true}],
               "subtitle_tracks":[{"id":"s:3","language":"eng","codec":"subrip","forced":false,"default":false,"hearing_impaired":false,"external":false,"image":false}],
               "versions":[],"chapters":[],"markers":[],"source_kbps":8000,"source_height":1080,"server_limit_kbps":0}""",
        )
        val caps = PlayerCaps(listOf("mp4", "mkv"), listOf("h264", "hevc"), listOf("aac", "ac3"), 2160)
        val plan = api.startPlayback("ep:1:tvdb:2", PlaybackRequest(caps, startMs = 120_000))
        assertEquals("transcode", plan.method)
        assertEquals(120_000, plan.offsetMs)
        assertEquals("s1", plan.sessionId)
        assertEquals(1, plan.audioTracks.single().index)
        val req = server.takeRequest()
        assertEquals("/api/v1/items/ep%3A1%3Atvdb%3A2/playback", req.url.encodedPath)
        assertEquals(
            """{"caps":{"containers":["mp4","mkv"],"video":["h264","hevc"],"audio":["aac","ac3"],"max_height":2160},"start_ms":120000}""",
            req.body?.utf8(),
        )
    }

    @Test
    fun `progress is a PUT with position, duration and state`() = runTest {
        answer(200, """{"position_ms":60000,"played":false}""")
        api.reportProgress("m1", ProgressReport(60_000, 7_200_000, "paused"))
        val req = server.takeRequest()
        assertEquals("PUT", req.method)
        assertEquals("""{"position_ms":60000,"duration_ms":7200000,"state":"paused"}""", req.body?.utf8())
    }

    @Test
    fun `an admin stopping playback is a 403 needing stopped`() = runTest {
        answer(403, """{"error":"Time for bed","needs":"stopped"}""")
        try {
            api.reportProgress("m1", ProgressReport(1, 2, "playing"))
            fail("expected an error")
        } catch (e: ApiException) {
            assertEquals("stopped", e.needs)
            assertEquals("Time for bed", e.message)
        }
    }

    @Test
    fun `addresses Arc gives are made absolute under api v1`() {
        val base = server.url("/").toString().trimEnd('/')
        assertEquals("$base/api/v1/transcode/s1/index.m3u8", api.absolute("/api/transcode/s1/index.m3u8"))
        assertEquals("$base/api/v1/items/m1/images/primary?v=abc", api.absolute("/api/items/m1/images/primary?v=abc"))
        assertEquals("$base/api/v1/health", api.absolute("/api/v1/health"))
    }

    @Test
    fun `items take their query, lists ignore unknown fields`() = runTest {
        answer(200, """{"total":1,"offset":0,"items":[{"id":"m1","kind":"movie","title":"Heat","year":1995,"sort_title":"heat","metadata_state":"matched","locked_fields":[],"added_at":1,"images":{"primary":"/api/items/m1/images/primary"},"user_data":{"position_ms":600000,"played":false},"runtime_min":170}]}""")
        val page = api.items(mapOf("library_id" to "1", "kind" to "movie", "sort" to "added", "limit" to "24"))
        val heat = page.items.single()
        assertEquals("Heat", heat.title)
        assertEquals(600_000, heat.resumeMs)
        assertEquals(170 * 60_000L, heat.durationMs)
        val req = server.takeRequest()
        assertEquals("/api/v1/items", req.url.encodedPath)
        assertEquals("1", req.url.queryParameter("library_id"))
        assertEquals("added", req.url.queryParameter("sort"))
    }
}
