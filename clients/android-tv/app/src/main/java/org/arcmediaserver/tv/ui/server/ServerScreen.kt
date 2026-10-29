package org.arcmediaserver.tv.ui.server

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.tv.material3.Button
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.arcmediaserver.tv.AppContainer
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.ui.components.requestWhenReady
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.data.display
import org.arcmediaserver.tv.data.normalizeServerAddress
import org.arcmediaserver.tv.ui.arcViewModel
import org.arcmediaserver.tv.ui.components.Spinner
import org.arcmediaserver.tv.ui.components.UiError
import org.arcmediaserver.tv.ui.components.text
import org.arcmediaserver.tv.ui.components.toUiError
import org.arcmediaserver.tv.ui.theme.ArcColors

sealed interface ServerProblem {
    data object Invalid : ServerProblem
    data object NotArc : ServerProblem
    data class Failed(val error: UiError) : ServerProblem
}

data class ServerUiState(
    val address: String = "",
    val checking: Boolean = false,
    val problem: ServerProblem? = null,
    val connected: Boolean = false,
)

class ServerViewModel(private val container: AppContainer) : ViewModel() {
    private val _state = MutableStateFlow(ServerUiState(address = container.session.server?.display().orEmpty()))
    val state: StateFlow<ServerUiState> = _state

    fun edit(text: String) = _state.update { it.copy(address = text, problem = null) }

    fun navigated() = _state.update { it.copy(connected = false) }

    fun connect() {
        val url = normalizeServerAddress(_state.value.address)
        if (url == null) {
            _state.update { it.copy(problem = ServerProblem.Invalid) }
            return
        }
        _state.update { it.copy(checking = true, problem = null) }
        viewModelScope.launch {
            val problem = try {
                val health = container.apiFor(url).health()
                if (health.apiVersion < 1) ServerProblem.NotArc else null.also {
                    ArcLog.Android.info("connected to ${health.name} ${health.version} (API v${health.apiVersion})")
                    container.session.server = url
                }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                // A web page that isn't Arc answers health with HTML (or 404).
                if (e.toUiError() == UiError.Unexpected || (e as? org.arcmediaserver.tv.data.ApiException)?.status == 404) ServerProblem.NotArc
                else ServerProblem.Failed(e.toUiError())
            }
            _state.update { it.copy(checking = false, problem = problem, connected = problem == null) }
        }
    }
}

@Composable
fun ServerRoute(onConnected: () -> Unit) {
    val vm = arcViewModel { ServerViewModel(it) }
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(state.connected) {
        if (state.connected) {
            vm.navigated()
            onConnected()
        }
    }
    ServerScreen(state, onEdit = vm::edit, onConnect = vm::connect)
}

@Composable
fun ServerScreen(state: ServerUiState, onEdit: (String) -> Unit, onConnect: () -> Unit) {
    val fieldFocus = remember { FocusRequester() }
    Box(Modifier.fillMaxSize().background(ArcColors.Background), contentAlignment = Alignment.Center) {
        BackdropGlow()
        Column(Modifier.width(560.dp), horizontalAlignment = Alignment.Start) {
            Icon(painterResource(R.drawable.arc_mark), contentDescription = null, tint = androidx.compose.ui.graphics.Color.Unspecified, modifier = Modifier.size(72.dp))
            Spacer(Modifier.height(20.dp))
            Text(stringResource(R.string.server_title), style = MaterialTheme.typography.displaySmall)
            Spacer(Modifier.height(8.dp))
            Text(stringResource(R.string.server_body), style = MaterialTheme.typography.bodyLarge, color = ArcColors.Muted)
            Spacer(Modifier.height(28.dp))
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                AddressField(state.address, onEdit, onConnect, Modifier.weight(1f).focusRequester(fieldFocus))
                Button(onClick = onConnect, enabled = !state.checking) {
                    Text(stringResource(if (state.checking) R.string.server_checking else R.string.server_connect))
                }
            }
            Spacer(Modifier.height(14.dp))
            val problem = state.problem
            when {
                state.checking -> Row(verticalAlignment = Alignment.CenterVertically) {
                    Spinner(size = 18.dp)
                    Spacer(Modifier.width(10.dp))
                    Text(stringResource(R.string.server_checking), style = MaterialTheme.typography.bodyMedium, color = ArcColors.Muted)
                }
                problem != null -> Text(
                    when (problem) {
                        ServerProblem.Invalid -> stringResource(R.string.server_invalid)
                        ServerProblem.NotArc -> stringResource(R.string.server_not_arc)
                        is ServerProblem.Failed -> problem.error.text()
                    },
                    style = MaterialTheme.typography.bodyMedium,
                    color = ArcColors.Error,
                )
                else -> Text(stringResource(R.string.server_port_note), style = MaterialTheme.typography.bodyMedium, color = ArcColors.Subtle)
            }
        }
    }
    LaunchedEffect(Unit) { fieldFocus.requestWhenReady() }
}

@Composable
private fun AddressField(value: String, onEdit: (String) -> Unit, onDone: () -> Unit, modifier: Modifier) {
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(10.dp)
    BasicTextField(
        value = value,
        onValueChange = onEdit,
        singleLine = true,
        textStyle = MaterialTheme.typography.titleLarge.copy(color = ArcColors.Foreground),
        cursorBrush = SolidColor(ArcColors.Accent),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go, autoCorrectEnabled = false),
        keyboardActions = KeyboardActions(onGo = { onDone() }),
        modifier = modifier.onFocusChanged { focused = it.isFocused },
        decorationBox = { inner ->
            Box(
                Modifier
                    .fillMaxWidth()
                    .background(if (focused) ArcColors.Surface2 else ArcColors.Surface, shape)
                    .border(BorderStroke(2.dp, if (focused) ArcColors.Accent else ArcColors.Surface3), shape)
                    .padding(horizontal = 18.dp, vertical = 14.dp),
            ) {
                if (value.isEmpty()) Text(stringResource(R.string.server_hint), style = MaterialTheme.typography.titleLarge, color = ArcColors.Subtle)
                inner()
            }
        },
    )
}

/** The soft violet light behind sign-in screens. */
@Composable
fun BackdropGlow() {
    Box(
        Modifier.fillMaxSize().background(
            androidx.compose.ui.graphics.Brush.radialGradient(
                listOf(ArcColors.AccentStrong.copy(alpha = 0.22f), ArcColors.Background.copy(alpha = 0f)),
                center = androidx.compose.ui.geometry.Offset(0f, 0f),
                radius = 1400f,
            ),
        ),
    )
}
