package org.arcmediaserver.tv.ui.player

import android.content.Context
import android.net.Uri
import androidx.annotation.OptIn
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import org.arcmediaserver.tv.AppContainer
import org.arcmediaserver.tv.data.ApiException
import org.arcmediaserver.tv.data.ArcApi
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.PlaybackPlan
import org.arcmediaserver.tv.data.PlaybackRequest
import org.arcmediaserver.tv.data.PlayerCaps
import org.arcmediaserver.tv.data.ProgressReport
import org.arcmediaserver.tv.data.SubtitleTrack
import org.arcmediaserver.tv.playback.PlaybackClock
import org.arcmediaserver.tv.playback.chooseSubtitle
import org.arcmediaserver.tv.playback.fallbackCaps
import org.arcmediaserver.tv.ui.components.UiError
import org.arcmediaserver.tv.ui.components.toUiError

data class PlayerUiState(
    val item: Item? = null,
    val plan: PlaybackPlan? = null,
    val loading: Boolean = true,
    val playing: Boolean = false,
    /** Where in the file, and how long the file is. */
    val positionMs: Long = 0,
    val durationMs: Long = 0,
    val bufferedMs: Long = 0,
    val subtitle: SubtitleTrack? = null,
    val error: UiError? = null,
    /** Arc stopped this playback (from its Dashboard), with the admin's message. */
    val stopped: String? = null,
    val ended: Boolean = false,
)

/**
 * Plays a movie or episode the way Arc's web player does: ask Arc how to play it with what this
 * device can play, play the file or the converted stream, save progress every 10 seconds and on
 * pause and close, keep a converting session alive, and end it when done.
 */
@OptIn(UnstableApi::class)
class PlayerViewModel(
    private val container: AppContainer,
    context: Context,
    private val itemId: String,
    private val startMs: Long,
) : ViewModel() {
    private val api: ArcApi = container.api() ?: error("no server")
    private val log = ArcLog.Android
    private var caps: PlayerCaps = container.playerCaps
    private var plan: PlaybackPlan? = null
    private var preferredSubtitleLanguage = ""
    private var retried = false
    private var loadJob: Job? = null

    private val _state = MutableStateFlow(PlayerUiState())
    val state: StateFlow<PlayerUiState> = _state

    val player: ExoPlayer = ExoPlayer.Builder(context)
        .setRenderersFactory(DefaultRenderersFactory(context).setEnableDecoderFallback(true))
        .setMediaSourceFactory(DefaultMediaSourceFactory(context).setDataSourceFactory(OkHttpDataSource.Factory(container.mediaClient)))
        .setAudioAttributes(AudioAttributes.Builder().setUsage(C.USAGE_MEDIA).setContentType(C.AUDIO_CONTENT_TYPE_MOVIE).build(), true)
        .setHandleAudioBecomingNoisy(true)
        .build()

    init {
        player.addListener(object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                _state.update { it.copy(playing = isPlaying) }
                if (!isPlaying && player.playbackState == Player.STATE_READY && !player.playWhenReady) report("paused")
            }

            override fun onPlaybackStateChanged(playbackState: Int) {
                if (playbackState == Player.STATE_READY) _state.update { it.copy(loading = false) }
                if (playbackState == Player.STATE_BUFFERING) _state.update { it.copy(loading = true) }
                if (playbackState == Player.STATE_ENDED) {
                    val p = plan ?: return
                    report("stopped", position = p.durationMs)
                    _state.update { it.copy(ended = true) }
                }
            }

            override fun onPlayerError(error: PlaybackException) = onStreamError(error)
        })
        viewModelScope.launch {
            // The title and the profile's subtitle language, alongside the first decision.
            val item = runCatching { api.item(itemId) }.getOrNull()
            _state.update { it.copy(item = item) }
        }
        viewModelScope.launch {
            preferredSubtitleLanguage = runCatching { api.prefs().subtitleLanguage }.getOrDefault("")
            load(startMs)
        }
        viewModelScope.launch { tick() }
        viewModelScope.launch { keepAlive() }
    }

    /** Where in the file the player is now. */
    private fun position(): Long = plan?.let { PlaybackClock.filePosition(it, player.currentPosition.coerceAtLeast(0)) } ?: startMs

    /** Starts (or restarts) playback at a point in the file, optionally with another audio track. */
    fun load(fromMs: Long, audioStream: Int? = null) {
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            plan?.sessionId?.let { sid -> runCatching { api.stopTranscode(sid) } }
            val p = try {
                api.startPlayback(itemId, PlaybackRequest(caps, startMs = fromMs, fileId = plan?.fileId, audioStream = audioStream ?: plan?.audioStream?.takeIf { it >= 0 }))
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                if (e is ApiException && e.status == 403) {
                    _state.update { it.copy(stopped = e.message, loading = false) }
                } else {
                    log.warn("playback decision failed: ${e.javaClass.simpleName} ${(e as? ApiException)?.status ?: ""}")
                    _state.update { it.copy(error = e.toUiError(), loading = false) }
                }
                return@launch
            }
            plan = p
            // Why it converts is worth explaining later (CLAUDE.md, Logging).
            if (p.method == "direct") log.info("playing directly")
            else log.info("Arc is ${if (p.method == "remux") "repackaging" else "converting"}: ${p.reasons.joinToString()}${p.encoder?.let { " ($it)" } ?: ""}")
            val first = _state.value.plan == null
            val subtitle = if (first) {
                chooseSubtitle(p.subtitleTracks, preferredSubtitleLanguage, p.audioTracks.find { it.index == p.audioStream }?.language)
            } else {
                _state.value.subtitle?.takeIf { s -> p.subtitleTracks.any { it.id == s.id } }
            }
            _state.update { it.copy(plan = p, durationMs = p.durationMs, subtitle = subtitle) }
            install(p, fromMs, subtitle)
        }
    }

    private fun install(p: PlaybackPlan, fileMs: Long, subtitle: SubtitleTrack?) {
        val builder = MediaItem.Builder().setUri(api.absolute(p.url))
        if (p.hls) {
            builder.setMimeType(MimeTypes.APPLICATION_M3U8)
            // Arc's playlist grows as it converts (an HLS "event" playlist). Play from the start
            // of it at normal speed, rather than racing to its live edge.
            builder.setLiveConfiguration(MediaItem.LiveConfiguration.Builder().setMinPlaybackSpeed(1f).setMaxPlaybackSpeed(1f).build())
        }
        if (subtitle != null) {
            val shift = if (p.hls) -p.offsetMs else 0
            val url = api.absolute("/api/items/${enc(itemId)}/subtitles/${enc(subtitle.id)}/vtt?file=${p.fileId}&shift_ms=$shift")
            builder.setSubtitleConfigurations(
                listOf(
                    MediaItem.SubtitleConfiguration.Builder(Uri.parse(url))
                        .setMimeType(MimeTypes.TEXT_VTT)
                        .setLanguage(subtitle.language)
                        .setSelectionFlags(C.SELECTION_FLAG_DEFAULT or C.SELECTION_FLAG_FORCED)
                        .setId("arc-${subtitle.id}")
                        .build(),
                ),
            )
        }
        player.trackSelectionParameters = player.trackSelectionParameters.buildUpon()
            .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, subtitle == null)
            .apply { if (subtitle != null) setPreferredTextLanguage(subtitle.language) }
            .build()
        player.setMediaItem(builder.build(), PlaybackClock.playerStart(p, fileMs))
        player.prepare()
        player.playWhenReady = true
    }

    /** The file as it is (or repackaged) didn't play here: ask Arc to do more, once, as the web player does. */
    private fun onStreamError(error: PlaybackException) {
        val p = plan
        if (p != null && !retried && p.method != "transcode") {
            retried = true
            log.info("${p.method} playback failed on this device (${error.errorCodeName}); asking Arc to convert more")
            caps = fallbackCaps(caps, p.method)
            load(position())
            return
        }
        log.warn("playback failed: ${error.errorCodeName}")
        _state.update { it.copy(error = UiError.Server(error.errorCodeName, 0), loading = false) }
    }

    fun togglePlay() {
        if (player.isPlaying) player.pause() else player.play()
    }

    /** Seeks to a point in the file, asking for a new stream when the converted one doesn't reach it. */
    fun seekTo(fileMs: Long) {
        val p = plan ?: return
        val target = fileMs.coerceIn(0, (p.durationMs - 1_000).coerceAtLeast(0))
        _state.update { it.copy(positionMs = target) }
        val available = player.duration.takeIf { it != C.TIME_UNSET } ?: 0
        if (PlaybackClock.seekInStream(p, target, available)) {
            player.seekTo(PlaybackClock.playerStart(p, target))
        } else {
            load(target)
        }
    }

    fun seekBy(deltaMs: Long) = seekTo(position() + deltaMs)

    fun chooseAudio(index: Int) {
        if (index == plan?.audioStream) return
        load(position(), audioStream = index)
    }

    fun chooseSubtitle(track: SubtitleTrack?) {
        val p = plan ?: return
        _state.update { it.copy(subtitle = track) }
        install(p, position(), track)
    }

    fun retry() {
        retried = false
        load(position())
    }

    private suspend fun tick() {
        var lastReport = System.currentTimeMillis()
        while (currentCoroutineContextActive()) {
            delay(500)
            val p = plan ?: continue
            val pos = position()
            val buffered = PlaybackClock.filePosition(p, player.bufferedPosition.coerceAtLeast(0))
            _state.update { it.copy(positionMs = pos, bufferedMs = buffered) }
            if (player.isPlaying && System.currentTimeMillis() - lastReport >= REPORT_EVERY_MS) {
                lastReport = System.currentTimeMillis()
                report("playing")
            }
        }
    }

    private suspend fun keepAlive() {
        while (currentCoroutineContextActive()) {
            delay(PING_EVERY_MS)
            val sid = plan?.sessionId ?: continue
            runCatching { api.pingTranscode(sid) }.onFailure { log.debug("keeping the conversion alive failed: ${it.javaClass.simpleName}") }
        }
    }

    private suspend fun currentCoroutineContextActive() = kotlin.coroutines.coroutineContext.isActive

    /** Saves progress. Arc answers 403 with needs "stopped" when an admin ends this playback. */
    private fun report(state: String, position: Long = position()) {
        val p = plan ?: return
        val duration = p.durationMs.takeIf { it > 0 } ?: return
        val report = ProgressReport(position.coerceIn(0, duration), duration, state)
        container.appScope.launch {
            try {
                api.reportProgress(itemId, report)
            } catch (e: ApiException) {
                if (e.needs == "stopped" || e.status == 403) {
                    viewModelScope.launch {
                        player.pause()
                        _state.update { it.copy(stopped = e.message) }
                    }
                } else {
                    log.warn("saving progress failed: ${e.status}")
                }
            } catch (e: Exception) {
                log.warn("saving progress failed: ${e.javaClass.simpleName}")
            }
        }
    }

    override fun onCleared() {
        val p = plan
        if (p != null && !_state.value.ended) report("stopped")
        p?.sessionId?.let { sid -> container.appScope.launch { runCatching { api.stopTranscode(sid) } } }
        player.release()
    }

    private companion object {
        const val REPORT_EVERY_MS = 10_000L
        const val PING_EVERY_MS = 30_000L
        fun enc(s: String): String = java.net.URLEncoder.encode(s, "UTF-8").replace("+", "%20")
    }
}
