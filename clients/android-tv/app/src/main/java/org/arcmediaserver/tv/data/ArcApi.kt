package org.arcmediaserver.tv.data

import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.serialization.DeserializationStrategy
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.Call
import okhttp3.Callback
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** A failure Arc answered with: its status, its plain-words message and what it needs next, if anything. */
class ApiException(val status: Int, message: String, val needs: String? = null) : IOException(message)

/** The answer to asking whether a TV sign-in code has been approved. */
sealed interface LinkPoll {
    data object Waiting : LinkPoll
    data object Expired : LinkPoll
    data class Approved(val token: String) : LinkPoll
}

val ArcJson = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
    coerceInputValues = true
    encodeDefaults = true
}

/**
 * Arc's stable API (/api/v1), signed in with a device token. Every call is made on OkHttp's
 * threads and can be cancelled with its coroutine.
 */
class ArcApi(
    val baseUrl: HttpUrl,
    private val client: OkHttpClient,
    private val token: () -> String?,
    private val log: ArcLog = ArcLog.Android,
) {
    suspend fun health(): Health = get("health", Health.serializer())

    suspend fun startLink(device: String): LinkStart =
        send("POST", "auth/link", JsonObject(mapOf("device" to JsonPrimitive(device))).toString(), LinkStart.serializer())

    suspend fun pollLink(poll: String): LinkPoll {
        val (status, body) = raw("GET", "auth/link/$poll", null, route = "auth/link/{poll}")
        return when (status) {
            202 -> LinkPoll.Waiting
            200 -> LinkPoll.Approved(decode(body, TokenAnswer.serializer(), "auth/link/{poll}").token)
            404 -> LinkPoll.Expired
            else -> throw failure(status, body, "auth/link/{poll}")
        }
    }

    suspend fun authStatus(): AuthStatus = get("auth/status", AuthStatus.serializer())

    suspend fun chooseProfile(id: String, pin: String?): Profile {
        val body = buildMap {
            put("id", JsonPrimitive(id))
            if (!pin.isNullOrEmpty()) put("pin", JsonPrimitive(pin))
        }
        return send("POST", "auth/profile", JsonObject(body).toString(), Profile.serializer())
    }

    suspend fun signOut() {
        raw("POST", "auth/logout", "{}")
    }

    suspend fun libraries(): List<Library> = get("libraries", kotlinx.serialization.builtins.ListSerializer(Library.serializer()))

    suspend fun prefs(): Prefs = get("me/prefs", Prefs.serializer())

    suspend fun continueWatching(withUpNext: Boolean): List<Item> =
        get(if (withUpNext) "home/continue?with_up_next=1" else "home/continue", ItemList, route = "home/continue")

    suspend fun nextUp(): List<Item> = get("home/next-up", ItemList)

    suspend fun watchlist(): List<Item> = get("watchlist", ItemList)

    /** Browse a library: [params] are /items' query parameters (library_id, kind, sort, limit...). */
    suspend fun items(params: Map<String, String>): ItemPage {
        val query = params.entries.joinToString("&") { (k, v) -> "${enc(k)}=${enc(v)}" }
        return get("items?$query", ItemPage.serializer(), route = "items")
    }

    suspend fun recentShows(libraryId: Long): List<RecentCard> =
        get("libraries/$libraryId/recent-shows", kotlinx.serialization.builtins.ListSerializer(RecentCard.serializer()), route = "libraries/{id}/recent-shows")

    suspend fun item(id: String): Item = get("items/${enc(id)}", Item.serializer(), route = "items/{id}")

    suspend fun seasons(seriesId: String): SeasonsResponse =
        get("series/${enc(seriesId)}/seasons", SeasonsResponse.serializer(), route = "series/{id}/seasons")

    suspend fun startPlayback(itemId: String, request: PlaybackRequest): PlaybackPlan =
        send("POST", "items/${enc(itemId)}/playback", ArcJson.encodeToString(PlaybackRequest.serializer(), request), PlaybackPlan.serializer(), route = "items/{id}/playback")

    suspend fun reportProgress(itemId: String, report: ProgressReport) {
        val (status, body) = raw("PUT", "items/${enc(itemId)}/progress", ArcJson.encodeToString(ProgressReport.serializer(), report), route = "items/{id}/progress")
        if (status !in 200..299) throw failure(status, body, "items/{id}/progress")
    }

    suspend fun pingTranscode(sessionId: String) {
        val (status, body) = raw("POST", "transcode/${enc(sessionId)}/ping", "{}", route = "transcode/{sid}/ping")
        if (status !in 200..299) throw failure(status, body, "transcode/{sid}/ping")
    }

    suspend fun stopTranscode(sessionId: String) {
        raw("DELETE", "transcode/${enc(sessionId)}", null, route = "transcode/{sid}")
    }

    /**
     * An address Arc gave (artwork, a stream, a playlist) as a full URL. Arc answers with its
     * unversioned /api/ paths; the same endpoints live under /api/v1/, which is what apps use.
     */
    fun absolute(path: String): String {
        val versioned = when {
            path.startsWith("/api/v1/") -> path
            path.startsWith("/api/") -> "/api/v1/" + path.removePrefix("/api/")
            else -> path
        }
        return baseUrl.resolve(versioned)?.toString() ?: (baseUrl.toString().trimEnd('/') + versioned)
    }

    // --- plumbing ---

    private suspend fun <T> get(path: String, de: DeserializationStrategy<T>, route: String = path): T = send("GET", path, null, de, route)

    private suspend fun <T> send(method: String, path: String, json: String?, de: DeserializationStrategy<T>, route: String = path): T {
        val (status, body) = raw(method, path, json, route)
        if (status !in 200..299) throw failure(status, body, route)
        return decode(body, de, route)
    }

    private fun <T> decode(body: String, de: DeserializationStrategy<T>, route: String): T = try {
        ArcJson.decodeFromString(de, body)
    } catch (e: SerializationException) {
        log.warn("unexpected answer from $route: ${e.message}")
        throw IOException("unexpected answer from the server", e)
    } catch (e: IllegalArgumentException) {
        log.warn("unexpected answer from $route: ${e.message}")
        throw IOException("unexpected answer from the server", e)
    }

    private fun failure(status: Int, body: String, route: String): ApiException {
        val obj = runCatching { ArcJson.decodeFromString(JsonObject.serializer(), body) }.getOrNull()
        val message = obj?.get("error")?.jsonPrimitive?.contentOrNull ?: "the server answered $status"
        val needs = obj?.get("needs")?.jsonPrimitive?.contentOrNull
        // Signed out, a wrong PIN or a missing profile are expected; anything else is worth a warning.
        if (status >= 500) log.warn("$route failed: $status $message") else log.debug("$route answered $status")
        return ApiException(status, message, needs)
    }

    /** Sends a request to /api/v1/[path]; [route] names it in logs without ids or query strings. */
    private suspend fun raw(method: String, path: String, json: String?, route: String = path): Pair<Int, String> {
        val url = baseUrl.toString().trimEnd('/') + "/api/v1/" + path
        val builder = Request.Builder().url(url).header("Accept", "application/json")
        token()?.let { builder.header("Authorization", "Bearer $it") }
        val body = json?.toRequestBody(JSON)
        builder.method(method, body ?: if (method == "POST" || method == "PUT") ByteArray(0).toRequestBody(JSON) else null)
        val call = client.newCall(builder.build())
        val response = try {
            call.await()
        } catch (e: IOException) {
            log.warn("$route: can't reach the server (${e.javaClass.simpleName})")
            throw e
        }
        return response.use { it.code to (it.body.string()) }
    }

    private companion object {
        val JSON = "application/json".toMediaType()
        val ItemList = kotlinx.serialization.builtins.ListSerializer(Item.serializer())
        fun enc(s: String): String = java.net.URLEncoder.encode(s, "UTF-8").replace("+", "%20")
    }
}

private suspend fun Call.await(): Response = suspendCancellableCoroutine { cont ->
    enqueue(object : Callback {
        override fun onResponse(call: Call, response: Response) = cont.resume(response) { _, r, _ -> r.close() }
        override fun onFailure(call: Call, e: IOException) {
            if (!cont.isCancelled) cont.resumeWithException(e)
        }
    })
    cont.invokeOnCancellation { runCatching { cancel() } }
}

/** A filter's value as a query parameter, or null for something /items doesn't take. */
internal fun JsonPrimitive.asQueryValue(): String? = when {
    isString -> content
    booleanOrNull != null -> if (booleanOrNull == true) "1" else null
    else -> contentOrNull
}
