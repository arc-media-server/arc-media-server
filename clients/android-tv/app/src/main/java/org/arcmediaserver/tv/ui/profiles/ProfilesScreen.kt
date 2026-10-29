package org.arcmediaserver.tv.ui.profiles

import android.view.KeyEvent
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.OutlinedButton
import kotlinx.coroutines.delay
import androidx.compose.foundation.background
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
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
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.arcmediaserver.tv.AppContainer
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.ui.components.requestWhenReady
import org.arcmediaserver.tv.data.ApiException
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.data.Profile
import org.arcmediaserver.tv.ui.arcViewModel
import org.arcmediaserver.tv.ui.components.ArcIcons
import org.arcmediaserver.tv.ui.components.LoadState
import org.arcmediaserver.tv.ui.components.Loaded
import org.arcmediaserver.tv.ui.components.UiError
import org.arcmediaserver.tv.ui.components.text
import org.arcmediaserver.tv.ui.components.toUiError
import org.arcmediaserver.tv.ui.theme.ArcColors

/** Asking for a profile's PIN: whose, what's been typed, and whether the last try was wrong. */
data class PinPrompt(val profile: Profile, val digits: String = "", val wrong: Boolean = false, val checking: Boolean = false)

data class ProfilesUiState(
    val profiles: LoadState<List<Profile>> = LoadState.Loading,
    val pin: PinPrompt? = null,
    /** A failure choosing a profile (not a wrong PIN, which the PIN pad shows). */
    val error: UiError? = null,
    val needsPassword: Boolean = false,
    /** Signed in with the password: the admin profile needs no PIN (else it shows a lock, as on the web). */
    val passwordSession: Boolean = true,
    val chosen: Boolean = false,
)

class ProfilesViewModel(private val container: AppContainer) : ViewModel() {
    private val _state = MutableStateFlow(ProfilesUiState())
    val state: StateFlow<ProfilesUiState> = _state

    init {
        load()
    }

    fun load() {
        val api = container.api() ?: return
        _state.update { it.copy(profiles = LoadState.Loading) }
        viewModelScope.launch {
            val result = try {
                api.authStatus().also { s -> _state.update { it.copy(passwordSession = s.passwordSession) } }.let { LoadState.Ready(it.profiles) }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                LoadState.Failed(e.toUiError())
            }
            _state.update { it.copy(profiles = result) }
        }
    }

    fun choose(profile: Profile) {
        if (profile.hasPin) {
            _state.update { it.copy(pin = PinPrompt(profile), error = null, needsPassword = false) }
        } else {
            submit(profile, null)
        }
    }

    fun typeDigit(d: Char) {
        val prompt = _state.value.pin ?: return
        if (prompt.checking || prompt.digits.length >= 4) return
        val digits = prompt.digits + d
        _state.update { it.copy(pin = prompt.copy(digits = digits, wrong = false)) }
        // PINs are four digits: check as soon as the fourth is in, as Plex does.
        if (digits.length == 4) submit(prompt.profile, digits)
    }

    fun deleteDigit() {
        val prompt = _state.value.pin ?: return
        _state.update { it.copy(pin = prompt.copy(digits = prompt.digits.dropLast(1), wrong = false)) }
    }

    fun cancelPin() = _state.update { it.copy(pin = null) }

    fun navigated() = _state.update { it.copy(chosen = false, pin = null) }

    private fun submit(profile: Profile, pin: String?) {
        val api = container.api() ?: return
        _state.update { s -> s.copy(pin = s.pin?.copy(checking = true), error = null, needsPassword = false) }
        viewModelScope.launch {
            try {
                api.chooseProfile(profile.id, pin)
                _state.update { it.copy(chosen = true) }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                val needs = (e as? ApiException)?.needs
                when (needs) {
                    "pin" -> _state.update { s -> s.copy(pin = (s.pin ?: PinPrompt(profile)).copy(digits = "", wrong = true, checking = false)) }
                    "password" -> _state.update { it.copy(pin = null, needsPassword = true) }
                    else -> {
                        ArcLog.Android.warn("choosing a profile failed: ${e.javaClass.simpleName}")
                        _state.update { it.copy(pin = null, error = e.toUiError()) }
                    }
                }
            }
        }
    }
}

@Composable
fun ProfilesRoute(onChosen: () -> Unit) {
    val vm = arcViewModel { ProfilesViewModel(it) }
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(state.chosen) {
        if (state.chosen) {
            vm.navigated()
            onChosen()
        }
    }
    androidx.activity.compose.BackHandler(enabled = state.pin != null) { vm.cancelPin() }
    ProfilesScreen(state, onChoose = vm::choose, onRetry = vm::load, onDigit = vm::typeDigit, onDelete = vm::deleteDigit, onBack = vm::cancelPin)
}

/**
 * "Who's watching?" as the web app's picker (web/src/components/auth/ProfilePicker.tsx): Arc's
 * mark and name over a soft accent glow, the heading, and round avatars with a lock badge for
 * profiles that ask for a PIN. Focus draws the web's ring and brightens the name.
 */
@Composable
fun ProfilesScreen(
    state: ProfilesUiState,
    onChoose: (Profile) -> Unit,
    onRetry: () -> Unit,
    onDigit: (Char) -> Unit,
    onDelete: () -> Unit,
    onBack: () -> Unit = {},
) {
    Box(Modifier.fillMaxSize().background(ArcColors.Background)) {
        AuthGlow()
        val pin = state.pin
        if (pin != null) {
            PinScreen(pin, onDigit, onDelete, onBack)
            return@Box
        }
        Loaded(state.profiles, onRetry) { profiles ->
            Column(Modifier.fillMaxSize(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                ArcWordmark()
                Spacer(Modifier.height(28.dp))
                Text(stringResource(R.string.profiles_title), style = MaterialTheme.typography.headlineMedium.copy(fontWeight = FontWeight.Bold, fontSize = 30.sp))
                Spacer(Modifier.height(36.dp))
                val first = remember { FocusRequester() }
                LazyRow(horizontalArrangement = Arrangement.spacedBy(32.dp), contentPadding = PaddingValues(horizontal = 48.dp, vertical = 12.dp)) {
                    itemsIndexed(profiles, key = { _, p -> p.id }) { i, p ->
                        ProfileTile(
                            p,
                            index = i,
                            locked = p.hasPin || (p.isAdmin && !state.passwordSession),
                            onClick = { onChoose(p) },
                            modifier = if (i == 0) Modifier.focusRequester(first) else Modifier,
                        )
                    }
                }
                LaunchedEffect(profiles) { first.requestWhenReady() }
                val message = when {
                    state.needsPassword -> stringResource(R.string.profiles_needs_password)
                    state.error != null -> state.error.text()
                    else -> null
                }
                Spacer(Modifier.height(24.dp))
                Text(message.orEmpty(), style = MaterialTheme.typography.bodyMedium, color = ArcColors.Warning, textAlign = TextAlign.Center, modifier = Modifier.width(560.dp))
            }
        }
    }
}

/** The web's auth backdrop: a large accent glow above the centre. */
@Composable
fun AuthGlow() {
    Box(
        Modifier.fillMaxSize().background(
            Brush.radialGradient(
                listOf(ArcColors.AccentStrong.copy(alpha = 0.18f), ArcColors.AccentStrong.copy(alpha = 0.06f), Color.Transparent),
                center = Offset(960f, -120f),
                radius = 1100f,
            ),
        ),
    )
}

/** Arc's mark and name, as at the top of the web's sign-in screens. */
@Composable
fun ArcWordmark() {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Icon(painterResource(R.drawable.arc_logo), contentDescription = null, tint = Color.Unspecified, modifier = Modifier.size(34.dp))
        Spacer(Modifier.width(10.dp))
        Text(stringResource(R.string.app_name), style = MaterialTheme.typography.headlineSmall.copy(fontWeight = FontWeight.Bold, letterSpacing = (-0.3).sp))
    }
}

/** A profile's picture: its initial on its colour (web/src/components/Avatar.tsx). */
@Composable
fun Avatar(profile: Profile, size: Dp, modifier: Modifier = Modifier) {
    Box(
        modifier.size(size).clip(CircleShape).background(Brush.linearGradient(ArcColors.profile(profile.color))),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            profile.name.trim().take(1).uppercase().ifEmpty { "?" },
            fontSize = (size.value * 0.36f).sp,
            fontWeight = FontWeight.SemiBold,
            color = Color.White,
        )
    }
}

@Composable
private fun ProfileTile(profile: Profile, index: Int, locked: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier) {
    var focused by remember { mutableStateOf(false) }
    // The web's entrance: each fades and rises in, one after another.
    val appear = remember { Animatable(0f) }
    LaunchedEffect(Unit) {
        delay(50L * index)
        appear.animateTo(1f, tween(350))
    }
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = Modifier.width(128.dp).graphicsLayer {
            alpha = appear.value
            translationY = (1f - appear.value) * 20f
        },
    ) {
        Box {
            Surface(
                onClick = onClick,
                modifier = modifier.onFocusChanged { focused = it.isFocused },
                shape = ClickableSurfaceDefaults.shape(CircleShape),
                scale = ClickableSurfaceDefaults.scale(focusedScale = 1.06f),
                border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(4.dp, ArcColors.Foreground), shape = CircleShape)),
                colors = ClickableSurfaceDefaults.colors(containerColor = Color.Transparent, focusedContainerColor = Color.Transparent),
            ) {
                Avatar(profile, 112.dp, Modifier.padding(4.dp))
            }
            if (locked) {
                Box(
                    Modifier.align(Alignment.BottomEnd).size(34.dp).clip(CircleShape).background(ArcColors.Background).padding(4.dp)
                        .clip(CircleShape).background(ArcColors.Surface3),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(ArcIcons.Lock, contentDescription = stringResource(R.string.profiles_locked), tint = ArcColors.Muted, modifier = Modifier.size(14.dp))
                }
            }
        }
        Spacer(Modifier.height(14.dp))
        Text(
            profile.name,
            style = MaterialTheme.typography.titleSmall,
            color = if (focused) ArcColors.Foreground else ArcColors.Muted,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/**
 * The web's PIN step: the avatar, the name, "Enter your PIN" and four boxes, the next one ringed in
 * the accent; a wrong PIN shakes them. A TV has no keyboard, so a number pad sits below (the
 * remote's number keys work too), and Profiles goes back.
 */
@Composable
private fun PinScreen(prompt: PinPrompt, onDigit: (Char) -> Unit, onDelete: () -> Unit, onBack: () -> Unit) {
    val five = remember { FocusRequester() }
    val shake = remember { Animatable(0f) }
    LaunchedEffect(prompt.wrong) {
        if (prompt.wrong) for (x in listOf(-10f, 10f, -6f, 6f, 0f)) shake.animateTo(x, tween(60))
    }
    Row(
        Modifier.fillMaxSize().onPreviewKeyDigits(onDigit, onDelete).padding(horizontal = 120.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(96.dp, Alignment.CenterHorizontally),
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Avatar(prompt.profile, 96.dp)
            Spacer(Modifier.height(16.dp))
            Text(prompt.profile.name, style = MaterialTheme.typography.headlineSmall)
            Spacer(Modifier.height(4.dp))
            Text(stringResource(R.string.pin_enter), style = MaterialTheme.typography.bodyMedium, color = ArcColors.Muted)
            Spacer(Modifier.height(24.dp))
            Row(Modifier.graphicsLayer { translationX = shake.value * density }, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                repeat(4) { i ->
                    val ring = when {
                        prompt.wrong -> ArcColors.Warning
                        i == prompt.digits.length -> ArcColors.Accent
                        else -> Color.White.copy(alpha = 0.08f)
                    }
                    Box(
                        Modifier.size(56.dp).clip(RoundedCornerShape(16.dp)).background(ArcColors.Surface2)
                            .border(if (i == prompt.digits.length && !prompt.wrong) 2.dp else 1.dp, ring, RoundedCornerShape(16.dp)),
                        contentAlignment = Alignment.Center,
                    ) {
                        if (i < prompt.digits.length) Text("•", fontSize = 26.sp, color = ArcColors.Foreground)
                    }
                }
            }
            Spacer(Modifier.height(12.dp))
            Text(if (prompt.wrong) stringResource(R.string.pin_wrong) else " ", style = MaterialTheme.typography.bodyMedium, color = ArcColors.Warning)
            Spacer(Modifier.height(16.dp))
            OutlinedButton(onClick = onBack, contentPadding = ButtonDefaults.ButtonWithIconContentPadding) {
                Icon(ArcIcons.ChevronLeft, contentDescription = null, modifier = Modifier.size(ButtonDefaults.IconSize))
                Spacer(Modifier.width(ButtonDefaults.IconSpacing))
                Text(stringResource(R.string.pin_back))
            }
        }
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            listOf("123", "456", "789", " 0<").forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    row.forEach { k ->
                        when (k) {
                            ' ' -> Spacer(Modifier.size(60.dp))
                            '<' -> PadKey(onClick = onDelete) {
                                Icon(ArcIcons.Delete, contentDescription = stringResource(R.string.pin_delete), modifier = Modifier.size(24.dp))
                            }
                            else -> PadKey(onClick = { onDigit(k) }, modifier = if (k == '5') Modifier.focusRequester(five) else Modifier) {
                                Text(k.toString(), style = MaterialTheme.typography.headlineSmall)
                            }
                        }
                    }
                }
            }
        }
    }
    LaunchedEffect(prompt.profile.id) { five.requestWhenReady() }
}

@Composable
private fun PadKey(onClick: () -> Unit, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    Surface(
        onClick = onClick,
        modifier = modifier.size(60.dp),
        shape = ClickableSurfaceDefaults.shape(RoundedCornerShape(16.dp)),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.08f),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = ArcColors.Surface2,
            contentColor = ArcColors.Foreground,
            focusedContainerColor = ArcColors.Foreground,
            focusedContentColor = ArcColors.Background,
        ),
    ) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { content() }
    }
}

/** The remote's 0–9 keys type digits and its Delete key removes one, wherever the focus is. */
private fun Modifier.onPreviewKeyDigits(onDigit: (Char) -> Unit, onDelete: () -> Unit): Modifier =
    onPreviewKeyEvent { e ->
        if (e.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
        when (val code = e.nativeKeyEvent.keyCode) {
            in KeyEvent.KEYCODE_0..KeyEvent.KEYCODE_9 -> {
                onDigit('0' + (code - KeyEvent.KEYCODE_0))
                true
            }
            KeyEvent.KEYCODE_DEL -> {
                onDelete()
                true
            }
            else -> false
        }
    }
