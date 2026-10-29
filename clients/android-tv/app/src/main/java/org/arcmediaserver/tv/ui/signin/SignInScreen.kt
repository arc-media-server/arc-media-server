package org.arcmediaserver.tv.ui.signin

import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.OutlinedButton
import androidx.tv.material3.Text
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.arcmediaserver.tv.AppContainer
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.ui.components.requestWhenReady
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.data.LinkPoll
import org.arcmediaserver.tv.data.display
import org.arcmediaserver.tv.ui.arcViewModel
import org.arcmediaserver.tv.ui.components.ErrorPane
import org.arcmediaserver.tv.ui.components.Spinner
import org.arcmediaserver.tv.ui.components.UiError
import org.arcmediaserver.tv.ui.components.toUiError
import org.arcmediaserver.tv.ui.launch.Destination
import org.arcmediaserver.tv.ui.launch.whereNext
import org.arcmediaserver.tv.ui.server.BackdropGlow
import org.arcmediaserver.tv.ui.theme.ArcColors

data class SignInUiState(
    val server: String = "",
    val code: String? = null,
    val renewed: Boolean = false,
    val error: UiError? = null,
    val done: Destination? = null,
)

/**
 * The TV sign-in code flow (docs/API.md, "On a TV, with a code"): show a code, ask every few
 * seconds whether someone entered it in Arc, and keep the device token it gives.
 */
class SignInViewModel(private val container: AppContainer, private val pollEveryMs: Long = 3_000) : ViewModel() {
    private val _state = MutableStateFlow(SignInUiState(server = container.session.server?.display().orEmpty()))
    val state: StateFlow<SignInUiState> = _state
    private var job: Job? = null

    init {
        start()
    }

    fun start(renewed: Boolean = false) {
        job?.cancel()
        val api = container.api() ?: return
        _state.update { it.copy(code = null, error = null, renewed = renewed) }
        job = viewModelScope.launch {
            try {
                val link = api.startLink(container.deviceName)
                _state.update { it.copy(code = link.code) }
                var failures = 0
                while (true) {
                    delay(pollEveryMs)
                    val answer = try {
                        api.pollLink(link.poll).also { failures = 0 }
                    } catch (e: Exception) {
                        if (e is kotlinx.coroutines.CancellationException) throw e
                        // A blip on the network is retried quietly; a few in a row are shown.
                        if (++failures >= 3) throw e
                        continue
                    }
                    when (answer) {
                        LinkPoll.Waiting -> Unit
                        LinkPoll.Expired -> {
                            ArcLog.Android.info("sign-in code ran out; asking for a new one")
                            start(renewed = true)
                            return@launch
                        }
                        is LinkPoll.Approved -> {
                            container.session.token = answer.token
                            ArcLog.Android.info("signed in with a TV code")
                            _state.update { it.copy(done = whereNext(container)) }
                            return@launch
                        }
                    }
                }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                ArcLog.Android.warn("TV sign-in failed: ${e.javaClass.simpleName}")
                _state.update { it.copy(error = e.toUiError()) }
            }
        }
    }
}

@Composable
fun SignInRoute(onSignedIn: (Destination) -> Unit, onChangeServer: () -> Unit) {
    val vm = arcViewModel { SignInViewModel(it) }
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(state.done) { state.done?.let(onSignedIn) }
    SignInScreen(state, onNewCode = { vm.start() }, onChangeServer = onChangeServer)
}

@Composable
fun SignInScreen(state: SignInUiState, onNewCode: () -> Unit, onChangeServer: () -> Unit) {
    Box(Modifier.fillMaxSize().background(ArcColors.Background)) {
        BackdropGlow()
        val error = state.error
        if (error != null) {
            ErrorPane(error, onRetry = onNewCode)
            return@Box
        }
        Row(Modifier.fillMaxSize().padding(horizontal = 96.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Icon(painterResource(R.drawable.arc_mark), contentDescription = null, tint = Color.Unspecified, modifier = Modifier.size(64.dp))
                Spacer(Modifier.height(20.dp))
                Text(stringResource(R.string.signin_title), style = MaterialTheme.typography.displaySmall)
                Spacer(Modifier.height(28.dp))
                Step(1, stringResource(R.string.signin_step_open))
                Spacer(Modifier.height(14.dp))
                Step(2, stringResource(R.string.signin_step_link, state.server.removePrefix("http://").removePrefix("https://") + "/link"))
                Spacer(Modifier.height(14.dp))
                Step(3, stringResource(R.string.signin_step_code))
                Spacer(Modifier.height(36.dp))
                val focus = remember { FocusRequester() }
                Row(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                    OutlinedButton(onClick = onNewCode, modifier = Modifier.focusRequester(focus)) { Text(stringResource(R.string.signin_new_code)) }
                    OutlinedButton(onClick = onChangeServer) { Text(stringResource(R.string.signin_change_server)) }
                }
                LaunchedEffect(Unit) { focus.requestWhenReady() }
            }
            Spacer(Modifier.width(64.dp))
            Column(Modifier.weight(1f), horizontalAlignment = Alignment.CenterHorizontally) {
                val code = state.code
                if (code == null) {
                    Spinner()
                } else {
                    CodeBoxes(code)
                    Spacer(Modifier.height(28.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Spinner(size = 16.dp)
                        Spacer(Modifier.width(10.dp))
                        Text(
                            stringResource(if (state.renewed) R.string.signin_expired else R.string.signin_waiting),
                            style = MaterialTheme.typography.bodyLarge,
                            color = ArcColors.Muted,
                        )
                    }
                }
                Spacer(Modifier.height(12.dp))
                Text(stringResource(R.string.signin_server, state.server), style = MaterialTheme.typography.bodyMedium, color = ArcColors.Subtle)
            }
        }
    }
}

@Composable
private fun Step(number: Int, text: String) {
    Row(verticalAlignment = Alignment.Top) {
        Box(Modifier.size(28.dp).background(ArcColors.Surface3, CircleShape), contentAlignment = Alignment.Center) {
            Text(number.toString(), style = MaterialTheme.typography.labelLarge, color = ArcColors.AccentLight)
        }
        Spacer(Modifier.width(14.dp))
        Text(text, style = MaterialTheme.typography.bodyLarge, color = ArcColors.Foreground, modifier = Modifier.padding(top = 2.dp))
    }
}

/** The code, one character to a box, large enough to read across a room. */
@Composable
private fun CodeBoxes(code: String) {
    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        code.forEach { c ->
            Box(
                Modifier
                    .size(width = 84.dp, height = 108.dp)
                    .background(ArcColors.Surface, RoundedCornerShape(12.dp))
                    .border(2.dp, ArcColors.Surface3, RoundedCornerShape(12.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Text(c.toString(), fontSize = 56.sp, fontWeight = FontWeight.SemiBold, color = ArcColors.Foreground)
            }
        }
    }
}
