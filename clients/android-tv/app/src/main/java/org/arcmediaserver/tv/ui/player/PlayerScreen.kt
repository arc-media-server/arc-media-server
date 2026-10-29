package org.arcmediaserver.tv.ui.player

import android.view.KeyEvent
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import androidx.tv.material3.Button
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Icon
import androidx.tv.material3.ListItem
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.OutlinedButton
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import kotlinx.coroutines.delay
import org.arcmediaserver.tv.ArcApplication
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.ui.components.requestWhenReady
import org.arcmediaserver.tv.data.AudioTrack
import org.arcmediaserver.tv.data.SubtitleTrack
import org.arcmediaserver.tv.playback.sameLanguage
import org.arcmediaserver.tv.ui.components.ArcIcons
import org.arcmediaserver.tv.ui.components.ErrorPane
import org.arcmediaserver.tv.ui.components.Spinner
import org.arcmediaserver.tv.ui.components.episodeCode
import org.arcmediaserver.tv.ui.details.clock
import org.arcmediaserver.tv.ui.theme.ArcColors
import java.util.Locale

private enum class Panel { None, Audio, Subtitles }

@androidx.annotation.OptIn(UnstableApi::class)
@Composable
fun PlayerRoute(itemId: String, startMs: Long, onClose: () -> Unit) {
    val context = LocalContext.current
    val container = (context.applicationContext as ArcApplication).container
    val vm: PlayerViewModel = viewModel(factory = viewModelFactory { initializer { PlayerViewModel(container, context.applicationContext, itemId, startMs) } })
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(state.ended) { if (state.ended) onClose() }

    var controls by remember { mutableStateOf(true) }
    var panel by remember { mutableStateOf(Panel.None) }
    var lastInput by remember { mutableLongStateOf(System.currentTimeMillis()) }
    // Pending seek while scrubbing with left/right: the point in the file it will go to.
    var scrub by remember { mutableStateOf<Long?>(null) }
    val rootFocus = remember { FocusRequester() }

    // Controls hide after a few seconds of playing without a key pressed, as Plex's do.
    LaunchedEffect(lastInput, state.playing, panel) {
        if (state.playing && panel == Panel.None) {
            delay(5_000)
            controls = false
        }
    }
    LaunchedEffect(scrub) {
        val target = scrub ?: return@LaunchedEffect
        delay(900)
        vm.seekTo(target)
        scrub = null
    }
    LaunchedEffect(controls) { if (!controls) rootFocus.requestWhenReady() }

    BackHandler(enabled = panel != Panel.None || controls) {
        if (panel != Panel.None) panel = Panel.None else controls = false
    }

    Box(
        Modifier
            .fillMaxSize()
            .background(Color.Black)
            .onPreviewKeyEvent { e ->
                if (e.type == KeyEventType.KeyDown) lastInput = System.currentTimeMillis()
                // Media keys work whatever has the focus.
                if (e.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                when (e.nativeKeyEvent.keyCode) {
                    KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE -> { vm.togglePlay(); controls = true; true }
                    KeyEvent.KEYCODE_MEDIA_PLAY -> { vm.player.play(); true }
                    KeyEvent.KEYCODE_MEDIA_PAUSE -> { vm.player.pause(); controls = true; true }
                    KeyEvent.KEYCODE_MEDIA_FAST_FORWARD -> { vm.seekBy(30_000); controls = true; true }
                    KeyEvent.KEYCODE_MEDIA_REWIND -> { vm.seekBy(-10_000); controls = true; true }
                    else -> false
                }
            }
            .focusRequester(rootFocus)
            .onKeyEvent { e ->
                // With the controls hidden: OK pauses, left and right skip, anything else shows them.
                if (controls || e.type != KeyEventType.KeyDown) return@onKeyEvent false
                when (e.nativeKeyEvent.keyCode) {
                    KeyEvent.KEYCODE_DPAD_CENTER, KeyEvent.KEYCODE_ENTER -> { vm.togglePlay(); controls = true; true }
                    KeyEvent.KEYCODE_DPAD_LEFT -> { vm.seekBy(-10_000); true }
                    KeyEvent.KEYCODE_DPAD_RIGHT -> { vm.seekBy(30_000); true }
                    KeyEvent.KEYCODE_DPAD_UP, KeyEvent.KEYCODE_DPAD_DOWN -> { controls = true; true }
                    else -> false
                }
            }
            .focusable(),
    ) {
        AndroidView(
            factory = { ctx ->
                PlayerView(ctx).apply {
                    useController = false
                    isFocusable = false
                    keepScreenOn = true
                    resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
                    setShutterBackgroundColor(android.graphics.Color.BLACK)
                    player = vm.player
                }
            },
            onRelease = { it.player = null },
            modifier = Modifier.fillMaxSize(),
        )
        when {
            state.stopped != null -> StoppedPane(state.stopped!!, onClose)
            state.error != null -> Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.85f))) {
                ErrorPane(state.error!!, onRetry = vm::retry, title = stringResource(R.string.player_failed))
            }
            else -> {
                if (state.loading) Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { Spinner(size = 56.dp) }
                AnimatedVisibility(controls, enter = fadeIn(), exit = fadeOut()) {
                    PlayerControls(
                        state = state,
                        scrubMs = scrub,
                        onPlayPause = vm::togglePlay,
                        onBack10 = { vm.seekBy(-10_000) },
                        onForward30 = { vm.seekBy(30_000) },
                        onScrub = { delta -> scrub = ((scrub ?: state.positionMs) + delta).coerceIn(0, state.durationMs) },
                        onAudio = { panel = Panel.Audio },
                        onSubtitles = { panel = Panel.Subtitles },
                    )
                }
                val plan = state.plan
                if (panel != Panel.None && plan != null) {
                    TrackPanel(
                        title = stringResource(if (panel == Panel.Audio) R.string.player_audio else R.string.player_subtitles),
                        options = if (panel == Panel.Audio) {
                            plan.audioTracks.mapIndexed { i, t -> TrackOption(audioName(t, i), audioDetail(t), t.index == plan.audioStream) { vm.chooseAudio(t.index); panel = Panel.None } }
                        } else {
                            listOf(TrackOption(stringResource(R.string.player_subtitles_off), null, state.subtitle == null) { vm.chooseSubtitle(null); panel = Panel.None }) +
                                plan.subtitleTracks.filter { !it.image }.mapIndexed { i, t ->
                                    TrackOption(subtitleName(t, i), subtitleDetail(t), state.subtitle?.id == t.id) { vm.chooseSubtitle(t); panel = Panel.None }
                                }
                        },
                    )
                }
            }
        }
    }
    LaunchedEffect(Unit) { rootFocus.requestWhenReady() }
}

@Composable
private fun StoppedPane(message: String, onClose: () -> Unit) {
    val focus = remember { FocusRequester() }
    Column(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.85f)), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
        Text(stringResource(R.string.player_stopped_title), style = MaterialTheme.typography.headlineSmall)
        Spacer(Modifier.height(8.dp))
        Text(message, style = MaterialTheme.typography.bodyLarge, color = ArcColors.Muted)
        Spacer(Modifier.height(24.dp))
        Button(onClick = onClose, modifier = Modifier.focusRequester(focus)) { Text(stringResource(R.string.player_close)) }
    }
    LaunchedEffect(Unit) { focus.requestWhenReady() }
}

private fun languageName(code: String?): String? =
    code?.takeIf { it.isNotBlank() && it != "und" }?.let { Locale.forLanguageTag(it).getDisplayLanguage(Locale.getDefault()).takeIf { n -> n.isNotBlank() && n != it } ?: it }

@Composable
private fun audioName(t: AudioTrack, i: Int): String = languageName(t.language) ?: stringResource(R.string.player_track, i + 1)

private val codecNames = mapOf("aac" to "AAC", "ac3" to "Dolby Digital", "eac3" to "Dolby Digital Plus", "truehd" to "Dolby TrueHD", "dts" to "DTS", "flac" to "FLAC", "mp3" to "MP3", "opus" to "Opus", "vorbis" to "Vorbis")

@Composable
private fun audioDetail(t: AudioTrack): String = listOfNotNull(
    codecNames[t.codec] ?: t.codec.uppercase().takeIf { it.isNotBlank() },
    when (t.channels) {
        0 -> null
        1 -> stringResource(R.string.player_mono)
        2 -> stringResource(R.string.player_stereo)
        6 -> "5.1"
        8 -> "7.1"
        else -> stringResource(R.string.player_channels, t.channels)
    },
    t.title,
).joinToString(" · ")

@Composable
private fun subtitleName(t: SubtitleTrack, i: Int): String = languageName(t.language) ?: t.title ?: stringResource(R.string.player_track, i + 1)

@Composable
private fun subtitleDetail(t: SubtitleTrack): String? = listOfNotNull(
    if (t.forced) stringResource(R.string.player_forced) else null,
    if (t.hearingImpaired) "SDH" else null,
    t.title?.takeIf { title -> !sameLanguage(title, t.language) && !title.equals("forced", true) && !title.equals("sdh", true) },
).joinToString(" · ").ifBlank { null }

/** The controls over the picture: what's playing, the timeline, and the buttons. */
@Composable
fun PlayerControls(
    state: PlayerUiState,
    scrubMs: Long?,
    onPlayPause: () -> Unit,
    onBack10: () -> Unit,
    onForward30: () -> Unit,
    onScrub: (Long) -> Unit,
    onAudio: () -> Unit,
    onSubtitles: () -> Unit,
) {
    val playFocus = remember { FocusRequester() }
    Box(Modifier.fillMaxSize()) {
        Box(Modifier.fillMaxWidth().height(160.dp).background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.7f), Color.Transparent))))
        Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().fillMaxHeight(0.5f).background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.85f)))))
        val item = state.item
        Column(Modifier.padding(start = 56.dp, top = 40.dp)) {
            if (item != null) {
                Text(if (item.isEpisode) item.seriesTitle ?: item.title else item.title, style = MaterialTheme.typography.headlineMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (item.isEpisode) Text(listOfNotNull(episodeCode(item), item.title).joinToString(" · "), style = MaterialTheme.typography.titleMedium, color = ArcColors.Muted)
            }
        }
        Column(Modifier.align(Alignment.BottomStart).fillMaxWidth().padding(horizontal = 56.dp, vertical = 36.dp)) {
            state.plan?.let { p ->
                val info = when (p.method) {
                    "transcode" -> stringResource(R.string.player_converting, p.reasons.joinToString(", "))
                    "remux" -> stringResource(R.string.player_repackaging, p.reasons.joinToString(", "))
                    else -> stringResource(R.string.player_direct)
                }
                Text(info, style = MaterialTheme.typography.bodySmall, color = ArcColors.Subtle, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Spacer(Modifier.height(10.dp))
            }
            Timeline(state, scrubMs, onScrub)
            Spacer(Modifier.height(18.dp))
            Box(Modifier.fillMaxWidth()) {
                Row(Modifier.align(Alignment.Center), horizontalArrangement = Arrangement.spacedBy(18.dp), verticalAlignment = Alignment.CenterVertically) {
                    RoundButton(ArcIcons.Replay, stringResource(R.string.player_back_10), onBack10)
                    RoundButton(
                        if (state.playing) ArcIcons.Pause else ArcIcons.Play,
                        stringResource(if (state.playing) R.string.player_pause else R.string.player_play),
                        onPlayPause,
                        large = true,
                        modifier = Modifier.focusRequester(playFocus),
                    )
                    RoundButton(ArcIcons.Forward, stringResource(R.string.player_forward_30), onForward30)
                }
                Row(Modifier.align(Alignment.CenterEnd), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    val plan = state.plan
                    if (plan != null && plan.audioTracks.size > 1) RoundButton(ArcIcons.Audio, stringResource(R.string.player_audio), onAudio)
                    if (plan != null && plan.subtitleTracks.any { !it.image }) RoundButton(ArcIcons.Subtitles, stringResource(R.string.player_subtitles), onSubtitles)
                }
            }
        }
    }
    LaunchedEffect(Unit) { playFocus.requestWhenReady() }
}

/** Times either side of a bar; focused, left and right move along it in 10-second steps. */
@Composable
private fun Timeline(state: PlayerUiState, scrubMs: Long?, onScrub: (Long) -> Unit) {
    var focused by remember { mutableStateOf(false) }
    val duration = state.durationMs.coerceAtLeast(1)
    val shown = scrubMs ?: state.positionMs
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(clock(shown), style = MaterialTheme.typography.titleSmall, modifier = Modifier.width(76.dp))
        Box(
            Modifier
                .weight(1f)
                .height(28.dp)
                .onFocusChanged { focused = it.isFocused }
                .onKeyEvent { e ->
                    if (e.type != KeyEventType.KeyDown) return@onKeyEvent false
                    when (e.nativeKeyEvent.keyCode) {
                        KeyEvent.KEYCODE_DPAD_LEFT -> { onScrub(-10_000); true }
                        KeyEvent.KEYCODE_DPAD_RIGHT -> { onScrub(10_000); true }
                        else -> false
                    }
                }
                .focusable(),
            contentAlignment = Alignment.CenterStart,
        ) {
            val h = if (focused) 8.dp else 5.dp
            Box(Modifier.fillMaxWidth().height(h).clip(RoundedCornerShape(4.dp)).background(Color.White.copy(alpha = 0.22f)))
            Box(Modifier.fillMaxWidth((state.bufferedMs.toFloat() / duration).coerceIn(0f, 1f)).height(h).clip(RoundedCornerShape(4.dp)).background(Color.White.copy(alpha = 0.35f)))
            Box(Modifier.fillMaxWidth((shown.toFloat() / duration).coerceIn(0f, 1f)).height(h).clip(RoundedCornerShape(4.dp)).background(ArcColors.Accent))
            if (focused) {
                androidx.compose.foundation.layout.BoxWithConstraints(Modifier.fillMaxWidth()) {
                    val x = maxWidth * (shown.toFloat() / duration).coerceIn(0f, 1f)
                    Box(Modifier.offset(x = x - 9.dp).size(18.dp).clip(CircleShape).background(Color.White))
                }
            }
        }
        Text(
            "-" + clock((state.durationMs - shown).coerceAtLeast(0)),
            style = MaterialTheme.typography.titleSmall,
            color = ArcColors.Muted,
            modifier = Modifier.width(86.dp).padding(start = 12.dp),
        )
    }
}

@Composable
private fun RoundButton(icon: ImageVector, label: String, onClick: () -> Unit, large: Boolean = false, modifier: Modifier = Modifier) {
    Surface(
        onClick = onClick,
        modifier = modifier.size(if (large) 64.dp else 48.dp),
        shape = ClickableSurfaceDefaults.shape(CircleShape),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.1f),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = Color.White.copy(alpha = 0.12f),
            contentColor = Color.White,
            focusedContainerColor = Color.White,
            focusedContentColor = Color.Black,
        ),
    ) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Icon(icon, contentDescription = label, modifier = Modifier.size(if (large) 34.dp else 24.dp))
        }
    }
}

private data class TrackOption(val name: String, val detail: String?, val selected: Boolean, val onClick: () -> Unit)

/** A panel at the right listing tracks to choose from, as Plex's audio and subtitle menus. */
@Composable
private fun TrackPanel(title: String, options: List<TrackOption>) {
    val focus = remember { FocusRequester() }
    Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.4f))) {
        Column(Modifier.align(Alignment.CenterEnd).fillMaxHeight().width(380.dp).background(ArcColors.Surface).padding(24.dp)) {
            Text(title, style = MaterialTheme.typography.headlineSmall)
            Spacer(Modifier.height(16.dp))
            LazyColumn(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                items(options) { o ->
                    ListItem(
                        selected = o.selected,
                        onClick = o.onClick,
                        modifier = if (o.selected) Modifier.focusRequester(focus) else Modifier,
                        headlineContent = { Text(o.name) },
                        supportingContent = o.detail?.let { { Text(it) } },
                        trailingContent = if (o.selected) {
                            { Icon(ArcIcons.Check, contentDescription = null, modifier = Modifier.size(20.dp)) }
                        } else null,
                    )
                }
            }
        }
    }
    LaunchedEffect(Unit) { focus.requestWhenReady() }
}
