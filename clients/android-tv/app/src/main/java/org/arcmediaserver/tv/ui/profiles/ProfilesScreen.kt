package org.arcmediaserver.tv.ui.profiles

import android.view.KeyEvent
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
import androidx.tv.material3.Glow
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
import org.arcmediaserver.tv.ui.server.BackdropGlow
import org.arcmediaserver.tv.ui.theme.ArcColors

/** Asking for a profile's PIN: whose, what's been typed, and whether the last try was wrong. */
data class PinPrompt(val profile: Profile, val digits: String = "", val wrong: Boolean = false, val checking: Boolean = false)

data class ProfilesUiState(
    val profiles: LoadState<List<Profile>> = LoadState.Loading,
    val pin: PinPrompt? = null,
    /** A failure choosing a profile (not a wrong PIN, which the PIN pad shows). */
    val error: UiError? = null,
    val needsPassword: Boolean = false,
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
                LoadState.Ready(api.authStatus().profiles)
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
    ProfilesScreen(state, onChoose = vm::choose, onRetry = vm::load, onDigit = vm::typeDigit, onDelete = vm::deleteDigit)
}

@Composable
fun ProfilesScreen(
    state: ProfilesUiState,
    onChoose: (Profile) -> Unit,
    onRetry: () -> Unit,
    onDigit: (Char) -> Unit,
    onDelete: () -> Unit,
) {
    Box(Modifier.fillMaxSize().background(ArcColors.Background)) {
        BackdropGlow()
        val pin = state.pin
        if (pin != null) {
            PinPad(pin, onDigit, onDelete)
            return@Box
        }
        Loaded(state.profiles, onRetry) { profiles ->
            Column(Modifier.fillMaxSize(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                Text(stringResource(R.string.profiles_title), style = MaterialTheme.typography.displayMedium)
                Spacer(Modifier.height(56.dp))
                val first = remember { FocusRequester() }
                LazyRow(horizontalArrangement = Arrangement.spacedBy(40.dp), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 48.dp, vertical = 16.dp)) {
                    itemsIndexed(profiles, key = { _, p -> p.id }) { i, p ->
                        ProfileTile(p, onClick = { onChoose(p) }, modifier = if (i == 0) Modifier.focusRequester(first) else Modifier)
                    }
                }
                LaunchedEffect(profiles) { first.requestWhenReady() }
                val message = when {
                    state.needsPassword -> stringResource(R.string.profiles_needs_password)
                    state.error != null -> state.error.text()
                    else -> null
                }
                Spacer(Modifier.height(32.dp))
                Text(message.orEmpty(), style = MaterialTheme.typography.bodyLarge, color = ArcColors.Error, textAlign = TextAlign.Center, modifier = Modifier.width(640.dp))
            }
        }
    }
}

/** A profile's picture: its initial on its colour (web/src/components/Avatar.tsx). */
@Composable
fun Avatar(profile: Profile, size: androidx.compose.ui.unit.Dp, modifier: Modifier = Modifier) {
    Box(
        modifier.size(size).clip(CircleShape).background(Brush.linearGradient(ArcColors.profile(profile.color))),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            profile.name.trim().take(1).uppercase().ifEmpty { "?" },
            fontSize = (size.value * 0.42f).sp,
            color = Color.White,
            style = MaterialTheme.typography.displayMedium,
        )
    }
}

@Composable
private fun ProfileTile(profile: Profile, onClick: () -> Unit, modifier: Modifier = Modifier) {
    var focused by remember { mutableStateOf(false) }
    Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.width(150.dp)) {
        Surface(
            onClick = onClick,
            modifier = modifier.onFocusChanged { focused = it.isFocused },
            shape = ClickableSurfaceDefaults.shape(CircleShape),
            scale = ClickableSurfaceDefaults.scale(focusedScale = 1.12f),
            border = ClickableSurfaceDefaults.border(focusedBorder = Border(androidx.compose.foundation.BorderStroke(4.dp, ArcColors.Foreground), shape = CircleShape)),
            glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(ArcColors.AccentStrong.copy(alpha = 0.6f), 24.dp)),
            colors = ClickableSurfaceDefaults.colors(containerColor = Color.Transparent, focusedContainerColor = Color.Transparent),
        ) {
            Avatar(profile, 128.dp)
        }
        Spacer(Modifier.height(18.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                profile.name,
                style = MaterialTheme.typography.titleLarge,
                color = if (focused) ArcColors.Foreground else ArcColors.Muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (profile.hasPin) {
                Spacer(Modifier.width(6.dp))
                Icon(ArcIcons.Lock, contentDescription = stringResource(R.string.profiles_locked), tint = ArcColors.Muted, modifier = Modifier.size(16.dp))
            }
        }
    }
}

/** Four dots and a number pad; the remote's number keys work too. */
@Composable
private fun PinPad(prompt: PinPrompt, onDigit: (Char) -> Unit, onDelete: () -> Unit) {
    val five = remember { FocusRequester() }
    Column(
        Modifier.fillMaxSize().onPreviewKeyDigits(onDigit, onDelete),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Avatar(prompt.profile, 56.dp)
        Spacer(Modifier.height(12.dp))
        Text(stringResource(R.string.pin_title, prompt.profile.name), style = MaterialTheme.typography.headlineSmall)
        Spacer(Modifier.height(16.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(18.dp)) {
            repeat(4) { i ->
                Box(
                    Modifier.size(18.dp).clip(CircleShape).background(
                        when {
                            prompt.wrong -> ArcColors.Error
                            i < prompt.digits.length -> ArcColors.Foreground
                            else -> ArcColors.Surface3
                        },
                    ),
                )
            }
        }
        Spacer(Modifier.height(8.dp))
        Text(
            if (prompt.wrong) stringResource(R.string.pin_wrong) else " ",
            style = MaterialTheme.typography.bodyMedium,
            color = ArcColors.Error,
        )
        Spacer(Modifier.height(10.dp))
        val keys = listOf("123", "456", "789", " 0<")
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            keys.forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    row.forEach { k ->
                        when (k) {
                            ' ' -> Spacer(Modifier.size(56.dp))
                            '<' -> PadKey(onClick = onDelete) {
                                Icon(ArcIcons.Backspace, contentDescription = stringResource(R.string.pin_delete), modifier = Modifier.size(26.dp))
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
        modifier = modifier.size(56.dp),
        shape = ClickableSurfaceDefaults.shape(CircleShape),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.1f),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = ArcColors.Surface2,
            contentColor = ArcColors.Foreground,
            focusedContainerColor = ArcColors.Foreground,
            focusedContentColor = ArcColors.Background,
        ),
    ) {
        Box(Modifier.fillMaxSize().padding(4.dp), contentAlignment = Alignment.Center) { content() }
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
