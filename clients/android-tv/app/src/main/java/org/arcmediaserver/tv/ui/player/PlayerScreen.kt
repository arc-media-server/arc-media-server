package org.arcmediaserver.tv.ui.player

import android.view.KeyEvent
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
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
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Icon
import androidx.tv.material3.ListItem
import androidx.tv.material3.ListItemDefaults
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import kotlinx.coroutines.delay
import org.arcmediaserver.tv.ArcApplication
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.data.AudioTrack
import org.arcmediaserver.tv.data.Chapter
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.Marker
import org.arcmediaserver.tv.data.PlaybackPlan
import org.arcmediaserver.tv.data.SubtitleTrack
import org.arcmediaserver.tv.data.Version
import org.arcmediaserver.tv.playback.sameLanguage
import org.arcmediaserver.tv.ui.components.ArcIcons
import org.arcmediaserver.tv.ui.components.ErrorPane
import org.arcmediaserver.tv.ui.components.Spinner
import org.arcmediaserver.tv.ui.components.episodeCode
import org.arcmediaserver.tv.ui.components.requestWhenReady
import org.arcmediaserver.tv.ui.details.clock
import org.arcmediaserver.tv.ui.theme.ArcColors
import java.util.Locale

/** The player's menus, as the web player's: each opens as a panel at the right. */
enum class PlayerMenu { None, Audio, Subtitles, Version, Quality, Chapters, Speed, Info }

/** Quality limits the player offers (kbit/s), as web/src/lib/playback.ts `qualities`. */
val qualities = listOf(20000, 12000, 8000, 4000, 3000, 2000, 1500, 720)
val speeds = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f)

/** e.g. "4 Mbps 720p" or "720 kbps 360p" (web qualityName). */
fun qualityName(kbps: Int): String {
    val height = when {
        kbps >= 8000 -> 1080
        kbps >= 2000 -> 720
        kbps >= 1500 -> 480
        else -> 360
    }
    val rate = if (kbps >= 1000) "${if (kbps % 1000 == 0) (kbps / 1000).toString() else (kbps / 1000.0).toString()} Mbps" else "$kbps kbps"
    return "$rate ${height}p"
}

/** The skip offered now: an intro or recap while it plays, or the credits (web Player.tsx). */
fun skippable(markers: List<Marker>, positionMs: Long): Marker? =
    markers.firstOrNull { positionMs >= it.startMs && positionMs < it.endMs - 1_500 }

@androidx.annotation.OptIn(UnstableApi::class)
@Composable
fun PlayerRoute(itemId: String, startMs: Long, onClose: () -> Unit, onPlayNext: (Item) -> Unit) {
    val context = LocalContext.current
    val container = (context.applicationContext as ArcApplication).container
    val vm: PlayerViewModel = viewModel(factory = viewModelFactory { initializer { PlayerViewModel(container, context.applicationContext, itemId, startMs) } })
    val state by vm.state.collectAsStateWithLifecycle()
    // At the end, carry on with the next episode (as the web's Up Next does), else close.
    LaunchedEffect(state.ended) {
        if (state.ended) state.next?.let(onPlayNext) ?: onClose()
    }

    var controls by remember { mutableStateOf(true) }
    var menu by remember { mutableStateOf(PlayerMenu.None) }
    var lastInput by remember { mutableLongStateOf(System.currentTimeMillis()) }
    var scrub by remember { mutableStateOf<Long?>(null) }
    val rootFocus = remember { FocusRequester() }

    LaunchedEffect(lastInput, state.playing, menu) {
        if (state.playing && menu == PlayerMenu.None) {
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

    // Back closes a menu, then hides the controls, then leaves the player.
    BackHandler(enabled = menu != PlayerMenu.None || controls) {
        if (menu != PlayerMenu.None) menu = PlayerMenu.None else controls = false
    }

    Box(
        Modifier
            .fillMaxSize()
            .background(Color.Black)
            .onPreviewKeyEvent { e ->
                if (e.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                lastInput = System.currentTimeMillis()
                when (e.nativeKeyEvent.keyCode) {
                    KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE -> { vm.togglePlay(); controls = true; true }
                    KeyEvent.KEYCODE_MEDIA_PLAY -> { vm.player.play(); true }
                    KeyEvent.KEYCODE_MEDIA_PAUSE -> { vm.player.pause(); controls = true; true }
                    KeyEvent.KEYCODE_MEDIA_FAST_FORWARD -> { vm.seekBy(10_000); controls = true; true }
                    KeyEvent.KEYCODE_MEDIA_REWIND -> { vm.seekBy(-10_000); controls = true; true }
                    KeyEvent.KEYCODE_MEDIA_NEXT -> { state.next?.let(onPlayNext); state.next != null }
                    else -> false
                }
            }
            .focusRequester(rootFocus)
            .onKeyEvent { e ->
                // Controls hidden: OK pauses, left and right go back and forward 10 seconds (the web
                // player's J and L), up and down show the controls.
                if (controls || e.type != KeyEventType.KeyDown) return@onKeyEvent false
                when (e.nativeKeyEvent.keyCode) {
                    KeyEvent.KEYCODE_DPAD_CENTER, KeyEvent.KEYCODE_ENTER -> { vm.togglePlay(); controls = true; true }
                    KeyEvent.KEYCODE_DPAD_LEFT -> { vm.seekBy(-10_000); true }
                    KeyEvent.KEYCODE_DPAD_RIGHT -> { vm.seekBy(10_000); true }
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
                PlayerControls(
                    state = state,
                    visible = controls,
                    scrubMs = scrub,
                    onPlayPause = vm::togglePlay,
                    onBack10 = { vm.seekBy(-10_000) },
                    onForward10 = { vm.seekBy(10_000) },
                    onScrub = { delta -> scrub = ((scrub ?: state.positionMs) + delta).coerceIn(0, state.durationMs) },
                    onSkip = { vm.seekTo(it.endMs) },
                    onNext = { state.next?.let(onPlayNext) },
                    onMenu = { menu = it },
                    onClose = onClose,
                )
                val plan = state.plan
                if (plan != null) {
                    PlayerMenuPanel(
                        menu = menu,
                        state = state,
                        plan = plan,
                        onAudio = { vm.chooseAudio(it); menu = PlayerMenu.None },
                        onSubtitle = { vm.chooseSubtitle(it); menu = PlayerMenu.None },
                        onVersion = { vm.chooseVersion(it); menu = PlayerMenu.None },
                        onQuality = { vm.chooseQuality(it); menu = PlayerMenu.None },
                        onChapter = { vm.seekTo(it); menu = PlayerMenu.None },
                        onSpeed = { vm.setSpeed(it); menu = PlayerMenu.None },
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

/**
 * The controls over the picture, as the web player's (web/src/components/Player.tsx): the title
 * and Close at the top; at the bottom the timeline, then play/pause, next, back and forward
 * 10 seconds and the time on the left, and audio, subtitles, version, quality, chapters, speed and
 * playback info on the right. Skip Intro / Skip Credits / Play Next appear at the right when the
 * markers say so, and take the focus even while the controls are hidden.
 */
@Composable
fun PlayerControls(
    state: PlayerUiState,
    visible: Boolean,
    scrubMs: Long?,
    onPlayPause: () -> Unit,
    onBack10: () -> Unit,
    onForward10: () -> Unit,
    onScrub: (Long) -> Unit,
    onSkip: (Marker) -> Unit,
    onNext: () -> Unit,
    onMenu: (PlayerMenu) -> Unit,
    onClose: () -> Unit,
) {
    val plan = state.plan
    val playFocus = remember { FocusRequester() }
    val skipFocus = remember { FocusRequester() }
    var focusedLabel by remember { mutableStateOf<String?>(null) }
    val skip = plan?.let { skippable(it.markers, state.positionMs) }
    Box(Modifier.fillMaxSize()) {
        AnimatedVisibility(visible, enter = fadeIn(), exit = fadeOut()) {
            Box(Modifier.fillMaxSize()) {
                // Top: what's playing, and Close.
                Box(Modifier.fillMaxWidth().height(150.dp).background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.7f), Color.Transparent))))
                Row(Modifier.fillMaxWidth().padding(horizontal = 48.dp, vertical = 32.dp), verticalAlignment = Alignment.Top) {
                    val item = state.item
                    Column(Modifier.weight(1f)) {
                        if (item != null) {
                            Text(item.title, style = MaterialTheme.typography.headlineSmall, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            val sub = if (item.isEpisode) listOfNotNull(item.seriesTitle, episodeCode(item)).joinToString(" · ") else item.year?.toString()
                            if (!sub.isNullOrEmpty()) Text(sub, style = MaterialTheme.typography.titleSmall, color = Color.White.copy(alpha = 0.6f))
                        }
                    }
                    ControlButton(ArcIcons.Close, stringResource(R.string.player_close_player), onClose, onLabel = { focusedLabel = it })
                }
                // Bottom: timeline and buttons.
                Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().fillMaxHeight(0.45f).background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.4f), Color.Black.copy(alpha = 0.8f)))))
                Column(Modifier.align(Alignment.BottomStart).fillMaxWidth().padding(start = 48.dp, end = 48.dp, bottom = 28.dp)) {
                    Timeline(state, scrubMs, onScrub, plan?.chapters.orEmpty())
                    Spacer(Modifier.height(12.dp))
                    Box(Modifier.fillMaxWidth().height(20.dp)) {
                        focusedLabel?.let { Text(it, style = MaterialTheme.typography.labelMedium, color = Color.White.copy(alpha = 0.8f)) }
                    }
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        val playLabel = stringResource(if (state.playing) R.string.player_pause else R.string.player_play)
                        ControlButton(if (state.playing) ArcIcons.Pause else ArcIcons.Play, playLabel, onPlayPause, Modifier.focusRequester(playFocus), onLabel = { focusedLabel = it })
                        state.next?.let { next ->
                            ControlButton(ArcIcons.SkipForward, stringResource(R.string.player_next_title, next.title), onNext, onLabel = { focusedLabel = it })
                        }
                        ControlButton(ArcIcons.RotateCcw, stringResource(R.string.player_back_10), onBack10, onLabel = { focusedLabel = it })
                        ControlButton(ArcIcons.RotateCw, stringResource(R.string.player_forward_10), onForward10, onLabel = { focusedLabel = it })
                        Spacer(Modifier.width(8.dp))
                        Text(clock(scrubMs ?: state.positionMs), style = MaterialTheme.typography.labelLarge, color = Color.White.copy(alpha = 0.8f))
                        Text(" / " + if (state.durationMs > 0) clock(state.durationMs) else "–:––", style = MaterialTheme.typography.labelLarge, color = Color.White.copy(alpha = 0.4f))
                        Spacer(Modifier.weight(1f))
                        if (plan != null) {
                            if (plan.audioTracks.size > 1) ControlButton(ArcIcons.Languages, stringResource(R.string.player_audio), { onMenu(PlayerMenu.Audio) }, onLabel = { focusedLabel = it })
                            if (plan.subtitleTracks.any { !it.image }) ControlButton(ArcIcons.Captions, stringResource(R.string.player_subtitles), { onMenu(PlayerMenu.Subtitles) }, onLabel = { focusedLabel = it })
                            if (plan.versions.size > 1) ControlButton(ArcIcons.Film, stringResource(R.string.player_version), { onMenu(PlayerMenu.Version) }, onLabel = { focusedLabel = it })
                            ControlButton(ArcIcons.Sliders, stringResource(R.string.player_quality), { onMenu(PlayerMenu.Quality) }, onLabel = { focusedLabel = it })
                            if (plan.chapters.size > 1) ControlButton(ArcIcons.ListOrdered, stringResource(R.string.player_chapters), { onMenu(PlayerMenu.Chapters) }, onLabel = { focusedLabel = it })
                        }
                        ControlButton(ArcIcons.Gauge, stringResource(R.string.player_speed), { onMenu(PlayerMenu.Speed) }, onLabel = { focusedLabel = it })
                        if (plan != null) ControlButton(ArcIcons.Info, stringResource(R.string.player_info), { onMenu(PlayerMenu.Info) }, onLabel = { focusedLabel = it })
                    }
                }
            }
        }
        // Skip Intro / Recap / Credits, or Play Next in the credits when there's a next episode.
        if (skip != null) {
            val label = when {
                skip.kind == "intro" -> stringResource(R.string.player_skip_intro)
                skip.kind == "recap" -> stringResource(R.string.player_skip_recap)
                state.next != null -> stringResource(R.string.player_play_next)
                else -> stringResource(R.string.player_skip_credits)
            }
            Button(
                onClick = { if (skip.kind == "credits" && state.next != null) onNext() else onSkip(skip) },
                modifier = Modifier.align(Alignment.BottomEnd).padding(end = 48.dp, bottom = if (visible) 150.dp else 48.dp).focusRequester(skipFocus),
                contentPadding = ButtonDefaults.ButtonWithIconContentPadding,
                colors = ButtonDefaults.colors(containerColor = ArcColors.AccentStrong, contentColor = Color.White, focusedContainerColor = Color.White, focusedContentColor = Color.Black),
            ) {
                Text(label)
                Spacer(Modifier.width(ButtonDefaults.IconSpacing))
                Icon(ArcIcons.SkipForward, contentDescription = null, modifier = Modifier.size(ButtonDefaults.IconSize))
            }
            LaunchedEffect(skip.kind, skip.startMs) { if (!visible) skipFocus.requestWhenReady() }
        }
    }
    LaunchedEffect(visible) { if (visible) playFocus.requestWhenReady() }
}

/** A round icon button; its label shows above the row while it has the focus. */
@Composable
private fun ControlButton(icon: ImageVector, label: String, onClick: () -> Unit, modifier: Modifier = Modifier, onLabel: (String?) -> Unit) {
    Surface(
        onClick = onClick,
        modifier = modifier.size(44.dp).onFocusChanged { if (it.isFocused) onLabel(label) },
        shape = ClickableSurfaceDefaults.shape(CircleShape),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.1f),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = Color.Transparent,
            contentColor = Color.White,
            focusedContainerColor = Color.White,
            focusedContentColor = Color.Black,
        ),
    ) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Icon(icon, contentDescription = label, modifier = Modifier.size(22.dp))
        }
    }
}

/** The chapter at a point, by index. */
private fun chapterAt(chapters: List<Chapter>, ms: Long): Int = chapters.indexOfLast { it.startMs <= ms }

/** The timeline: focused, left and right move along it in 10-second steps; chapters are marked. */
@Composable
private fun Timeline(state: PlayerUiState, scrubMs: Long?, onScrub: (Long) -> Unit, chapters: List<Chapter>) {
    var focused by remember { mutableStateOf(false) }
    val duration = state.durationMs.coerceAtLeast(1)
    val shown = scrubMs ?: state.positionMs
    Box(
        Modifier
            .fillMaxWidth()
            .height(24.dp)
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
        val h = if (focused) 8.dp else 4.dp
        Box(Modifier.fillMaxWidth().height(h).clip(RoundedCornerShape(4.dp)).background(Color.White.copy(alpha = 0.2f)))
        Box(Modifier.fillMaxWidth((state.bufferedMs.toFloat() / duration).coerceIn(0f, 1f)).height(h).clip(RoundedCornerShape(4.dp)).background(Color.White.copy(alpha = 0.3f)))
        Box(Modifier.fillMaxWidth((shown.toFloat() / duration).coerceIn(0f, 1f)).height(h).clip(RoundedCornerShape(4.dp)).background(ArcColors.Accent))
        BoxWithConstraints(Modifier.fillMaxWidth()) {
            chapters.drop(1).forEach { c ->
                Box(Modifier.offset(x = maxWidth * (c.startMs.toFloat() / duration).coerceIn(0f, 1f)).width(2.dp).height(h).background(Color.Black.copy(alpha = 0.6f)))
            }
            if (focused) {
                val x = maxWidth * (shown.toFloat() / duration).coerceIn(0f, 1f)
                Box(Modifier.offset(x = x - 8.dp).size(16.dp).clip(CircleShape).background(Color.White))
            }
        }
    }
}

private fun languageName(code: String?): String? =
    code?.takeIf { it.isNotBlank() && it != "und" }?.let {
        Locale.forLanguageTag(it).getDisplayLanguage(Locale.getDefault()).takeIf { n -> n.isNotBlank() && !n.equals(it, ignoreCase = true) } ?: it
    }

private val codecNames = mapOf(
    "aac" to "AAC", "ac3" to "Dolby Digital", "eac3" to "Dolby Digital Plus", "truehd" to "Dolby TrueHD", "dts" to "DTS",
    "flac" to "FLAC", "mp3" to "MP3", "opus" to "Opus", "vorbis" to "Vorbis", "h264" to "H.264", "hevc" to "HEVC", "av1" to "AV1", "vp9" to "VP9",
)

@Composable
private fun audioName(t: AudioTrack, i: Int): String = languageName(t.language) ?: stringResource(R.string.player_track, i + 1)

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

@Composable
private fun versionName(v: Version): Pair<String, String> {
    val res = when {
        v.height >= 2000 -> "4K"
        v.height > 0 -> "${v.height}p"
        else -> ""
    }
    val rest = listOfNotNull(v.hdr, v.videoCodec?.let { codecNames[it] ?: it.uppercase() }, if (v.size > 0) "%.1f GB".format(v.size / 1e9) else null)
    return if (!v.edition.isNullOrBlank()) v.edition to (listOf(res) + rest).filter { it.isNotBlank() }.joinToString(" · ")
    else res.ifBlank { stringResource(R.string.player_version) } to rest.joinToString(" · ")
}

private data class Option(val name: String, val detail: String?, val selected: Boolean, val onClick: () -> Unit)

/** Each menu's choices, from the same rules the web player's menus follow. */
@Composable
private fun PlayerMenuPanel(
    menu: PlayerMenu,
    state: PlayerUiState,
    plan: PlaybackPlan,
    onAudio: (Int) -> Unit,
    onSubtitle: (SubtitleTrack?) -> Unit,
    onVersion: (Long) -> Unit,
    onQuality: (Int) -> Unit,
    onChapter: (Long) -> Unit,
    onSpeed: (Float) -> Unit,
) {
    AnimatedVisibility(menu != PlayerMenu.None, enter = slideInHorizontally { it } + fadeIn(), exit = slideOutHorizontally { it } + fadeOut()) {
        val title: String
        var body: String? = null
        val options: List<Option>
        when (menu) {
            PlayerMenu.Audio -> {
                title = stringResource(R.string.player_audio)
                options = plan.audioTracks.mapIndexed { i, t -> Option(audioName(t, i), audioDetail(t), t.index == plan.audioStream) { onAudio(t.index) } }
            }
            PlayerMenu.Subtitles -> {
                title = stringResource(R.string.player_subtitles)
                options = listOf(Option(stringResource(R.string.player_subtitles_off), null, state.subtitle == null) { onSubtitle(null) }) +
                    plan.subtitleTracks.filter { !it.image }.mapIndexed { i, t -> Option(subtitleName(t, i), subtitleDetail(t), state.subtitle?.id == t.id) { onSubtitle(t) } }
            }
            PlayerMenu.Version -> {
                title = stringResource(R.string.player_version)
                options = plan.versions.map { v -> versionName(v).let { (n, d) -> Option(n, d.ifBlank { null }, v.fileId == plan.fileId) { onVersion(v.fileId) } } }
            }
            PlayerMenu.Quality -> {
                title = stringResource(R.string.player_quality)
                val cap = plan.serverLimitKbps
                val choices = listOf(-1, 0) + qualities.filter { (plan.sourceKbps == 0 || it < plan.sourceKbps) && (cap == 0 || it < cap) }
                options = choices.map { q ->
                    val name = when {
                        q == -1 -> stringResource(R.string.player_automatic)
                        q > 0 -> qualityName(q)
                        cap > 0 -> stringResource(R.string.player_maximum_capped, qualityName(cap))
                        else -> stringResource(R.string.player_original)
                    }
                    Option(name, null, q == state.quality) { onQuality(q) }
                }
                if (cap > 0) body = stringResource(R.string.player_away_limit, qualityName(cap))
            }
            PlayerMenu.Chapters -> {
                title = stringResource(R.string.player_chapters)
                val at = chapterAt(plan.chapters, state.positionMs)
                options = plan.chapters.mapIndexed { i, c ->
                    Option(c.title?.takeIf { it.isNotBlank() } ?: stringResource(R.string.player_chapter_number, i + 1), clock(c.startMs), i == at) { onChapter(c.startMs) }
                }
            }
            PlayerMenu.Speed -> {
                title = stringResource(R.string.player_speed)
                options = speeds.map { s -> Option(if (s == 1f) stringResource(R.string.player_normal) else "${s.toString().removeSuffix(".0")}×", null, s == state.speed) { onSpeed(s) } }
            }
            PlayerMenu.Info, PlayerMenu.None -> {
                val (t, d) = methodLabel(plan)
                title = t
                body = d
                options = emptyList()
            }
        }
        OptionPanel(title, options, body)
    }
}

/** How the file is being sent (web/src/lib/playback.ts methodLabel), for Playback info. */
@Composable
fun methodLabel(p: PlaybackPlan): Pair<String, String> {
    val reasons = p.reasons.joinToString(", ")
    return when (p.method) {
        "direct" -> stringResource(R.string.method_direct_title) to stringResource(R.string.method_direct)
        "remux" -> stringResource(R.string.method_remux_title) to stringResource(R.string.method_remux, reasons)
        else -> stringResource(R.string.method_converting_title) to when {
            !p.encoder.isNullOrBlank() && p.toneMapped -> stringResource(R.string.method_hardware_hdr, reasons, p.encoder)
            !p.encoder.isNullOrBlank() -> stringResource(R.string.method_hardware, reasons, p.encoder)
            p.toneMapped -> stringResource(R.string.method_software_hdr, reasons)
            else -> stringResource(R.string.method_software, reasons)
        }
    }
}

@Composable
private fun OptionPanel(title: String, options: List<Option>, body: String?) {
    val focus = remember { FocusRequester() }
    Box(Modifier.fillMaxSize()) {
        Column(
            Modifier.align(Alignment.CenterEnd).fillMaxHeight().width(360.dp).background(ArcColors.Surface.copy(alpha = 0.97f)).padding(horizontal = 20.dp, vertical = 28.dp),
        ) {
            Text(title, style = MaterialTheme.typography.titleLarge)
            Spacer(Modifier.height(12.dp))
            body?.let {
                Text(it, style = MaterialTheme.typography.bodyMedium, color = ArcColors.Muted)
                Spacer(Modifier.height(12.dp))
            }
            if (options.isEmpty()) {
                // Playback info has nothing to choose: a focusable close target keeps D-pad focus here.
                Box(Modifier.focusRequester(focus).focusable())
            } else {
                LazyColumn(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    items(options) { o ->
                        ListItem(
                            selected = o.selected,
                            onClick = o.onClick,
                            modifier = if (o.selected || o === options.first() && options.none { it.selected }) Modifier.focusRequester(focus) else Modifier,
                            headlineContent = { Text(o.name, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                            supportingContent = o.detail?.let { { Text(it, maxLines = 1, overflow = TextOverflow.Ellipsis) } },
                            leadingContent = {
                                Box(Modifier.size(20.dp)) {
                                    if (o.selected) Icon(ArcIcons.Check, contentDescription = null, tint = ArcColors.Accent, modifier = Modifier.size(20.dp))
                                }
                            },
                            colors = ListItemDefaults.colors(
                                containerColor = Color.Transparent,
                                selectedContainerColor = Color.Transparent,
                                focusedContainerColor = ArcColors.Surface3,
                                contentColor = ArcColors.Foreground,
                                selectedContentColor = ArcColors.Foreground,
                                focusedContentColor = ArcColors.Foreground,
                            ),
                        )
                    }
                }
            }
        }
    }
    LaunchedEffect(title) { focus.requestWhenReady() }
}
