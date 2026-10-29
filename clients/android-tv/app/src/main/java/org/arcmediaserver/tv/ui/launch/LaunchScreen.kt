package org.arcmediaserver.tv.ui.launch

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.tv.material3.Icon
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import org.arcmediaserver.tv.AppContainer
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.ui.arcViewModel
import org.arcmediaserver.tv.ui.components.ErrorPane
import org.arcmediaserver.tv.ui.components.LoadState
import org.arcmediaserver.tv.ui.components.Spinner
import org.arcmediaserver.tv.ui.components.toUiError
import org.arcmediaserver.tv.ui.theme.ArcColors

/** Where the app goes next, from what it knows about the server and the sign-in. */
enum class Destination { Server, SignIn, Profiles, Home }

/** Asks Arc whether this TV is still signed in, and as whom. */
suspend fun whereNext(container: AppContainer): Destination {
    val api = container.api() ?: return Destination.Server
    if (container.session.token == null) return Destination.SignIn
    val status = api.authStatus()
    return when {
        !status.signedIn -> {
            // Signed out from Arc's Settings > Devices: start again with a new code.
            ArcLog.Android.info("this TV's sign-in is no longer valid; asking for a new code")
            container.session.token = null
            Destination.SignIn
        }
        status.friend || status.profile != null -> Destination.Home
        else -> Destination.Profiles
    }
}

class LaunchViewModel(private val container: AppContainer) : ViewModel() {
    private val _state = MutableStateFlow<LoadState<Destination>>(LoadState.Loading)
    val state: StateFlow<LoadState<Destination>> = _state

    init {
        load()
    }

    fun load() {
        _state.value = LoadState.Loading
        viewModelScope.launch {
            _state.value = try {
                LoadState.Ready(whereNext(container))
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                LoadState.Failed(e.toUiError())
            }
        }
    }
}

@Composable
fun LaunchRoute(onDestination: (Destination) -> Unit, onChangeServer: () -> Unit) {
    val vm = arcViewModel { LaunchViewModel(it) }
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(state) { (state as? LoadState.Ready)?.let { onDestination(it.value) } }
    Box(Modifier.fillMaxSize().background(ArcColors.Background), contentAlignment = Alignment.Center) {
        when (val s = state) {
            is LoadState.Failed -> Column(horizontalAlignment = Alignment.CenterHorizontally) {
                ErrorPane(s.error, onRetry = vm::load, modifier = Modifier.height(320.dp))
                androidx.tv.material3.OutlinedButton(onClick = onChangeServer) {
                    androidx.tv.material3.Text(androidx.compose.ui.res.stringResource(R.string.signin_change_server))
                }
            }
            else -> Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Icon(painterResource(R.drawable.arc_mark), contentDescription = null, tint = Color.Unspecified, modifier = Modifier.size(96.dp))
                Spacer(Modifier.height(24.dp))
                Spinner(size = 28.dp)
            }
        }
    }
}
