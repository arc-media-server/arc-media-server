package org.arcmediaserver.tv

import android.content.Context
import android.os.Build
import android.provider.Settings
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import org.arcmediaserver.tv.data.ArcApi
import org.arcmediaserver.tv.data.SessionStore
import org.arcmediaserver.tv.data.sessionStore
import org.arcmediaserver.tv.playback.deviceCaps
import java.util.concurrent.TimeUnit

/** The app's long-lived objects, made once in [ArcApplication]. */
class AppContainer(private val context: Context) {
    val session: SessionStore = sessionStore(context)

    /** Work that must finish even when its screen has gone (telling Arc playback stopped). */
    val appScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    /** Sends the device token to Arc, and only to Arc (artwork may redirect to TMDB). */
    private val auth = Interceptor { chain ->
        val req = chain.request()
        val server = session.server
        val token = session.token
        if (server != null && token != null && req.url.host == server.host && req.url.port == server.port) {
            chain.proceed(req.newBuilder().header("Authorization", "Bearer $token").build())
        } else {
            chain.proceed(req)
        }
    }

    private val baseClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .build()

    /** For artwork and streams: signed in to the current server. */
    val mediaClient: OkHttpClient = baseClient.newBuilder().addInterceptor(auth).build()

    private var cachedApi: ArcApi? = null

    /** The API of the server chosen, or null before one is. */
    fun api(): ArcApi? {
        val server = session.server ?: return null
        cachedApi?.takeIf { it.baseUrl == server }?.let { return it }
        return ArcApi(server, baseClient, token = { session.token }).also { cachedApi = it }
    }

    /** An API for checking an address before it's saved. */
    fun apiFor(server: HttpUrl): ArcApi = ArcApi(server, baseClient, token = { null })

    val playerCaps by lazy { deviceCaps(context) }

    /** How this TV appears in Arc's Settings > Devices: the name it was given, e.g. "Living Room TV". */
    val deviceName: String by lazy {
        val named = runCatching { Settings.Global.getString(context.contentResolver, Settings.Global.DEVICE_NAME) }.getOrNull()
        named?.takeIf { it.isNotBlank() } ?: "${Build.MANUFACTURER} ${Build.MODEL}".trim()
    }
}
