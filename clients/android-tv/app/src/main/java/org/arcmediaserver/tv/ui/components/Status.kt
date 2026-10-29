package org.arcmediaserver.tv.ui.components

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.data.ApiException
import org.arcmediaserver.tv.ui.theme.ArcColors
import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import javax.net.ssl.SSLException

/** Something that went wrong, in a form the screen can explain. */
sealed interface UiError {
    /** Arc's own words, e.g. "wrong PIN". */
    data class Server(val message: String, val status: Int) : UiError
    data object Unreachable : UiError
    data object Timeout : UiError
    data object Secure : UiError
    data object Unexpected : UiError
}

fun Throwable.toUiError(): UiError = when (this) {
    is ApiException -> UiError.Server(message ?: "", status)
    is UnknownHostException, is ConnectException -> UiError.Unreachable
    is SocketTimeoutException -> UiError.Timeout
    is SSLException -> UiError.Secure
    is IOException -> if (cause is kotlinx.serialization.SerializationException) UiError.Unexpected else UiError.Unreachable
    else -> UiError.Unexpected
}

/** A screen's data: loading, failed, or there. */
sealed interface LoadState<out T> {
    data object Loading : LoadState<Nothing>
    data class Failed(val error: UiError) : LoadState<Nothing>
    data class Ready<T>(val value: T) : LoadState<T>
}

@Composable
fun UiError.text(): String = when (this) {
    is UiError.Server -> if (message.isNotBlank()) message.replaceFirstChar { it.uppercase() } else stringResource(R.string.error_server, status)
    UiError.Unreachable -> stringResource(R.string.error_unreachable)
    UiError.Timeout -> stringResource(R.string.error_timeout)
    UiError.Secure -> stringResource(R.string.error_secure)
    UiError.Unexpected -> stringResource(R.string.error_unexpected)
}

/** A spinner in Arc's accent, for the moments something is on its way. */
@Composable
fun Spinner(modifier: Modifier = Modifier, size: Dp = 40.dp) {
    val transition = rememberInfiniteTransition(label = "spinner")
    val angle by transition.animateFloat(0f, 360f, infiniteRepeatable(tween(900, easing = LinearEasing), RepeatMode.Restart), label = "angle")
    Canvas(modifier.size(size)) {
        val stroke = Stroke(width = this.size.minDimension / 10, cap = StrokeCap.Round)
        drawArc(ArcColors.Surface3, 0f, 360f, false, style = stroke)
        drawArc(ArcColors.AccentGradient, angle, 100f, false, style = stroke)
    }
}

@Composable
fun LoadingPane(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize(), contentAlignment = Alignment.Center) { Spinner() }
}

/** What went wrong and a Retry button, which takes the focus so OK tries again. */
@Composable
fun ErrorPane(error: UiError, onRetry: () -> Unit, modifier: Modifier = Modifier, title: String = stringResource(R.string.error_title)) {
    val focus = remember { FocusRequester() }
    Box(modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center, modifier = Modifier.widthIn(max = 520.dp)) {
            Text(title, style = MaterialTheme.typography.headlineSmall, textAlign = TextAlign.Center)
            Spacer(Modifier.height(8.dp))
            Text(error.text(), style = MaterialTheme.typography.bodyLarge, color = ArcColors.Muted, textAlign = TextAlign.Center)
            Spacer(Modifier.height(24.dp))
            Button(onClick = onRetry, modifier = Modifier.focusRequester(focus)) { Text(stringResource(R.string.action_retry)) }
        }
    }
    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
}

/** Shows [content] once the data's there, a spinner while it comes, or the error with Retry. */
@Composable
fun <T> Loaded(state: LoadState<T>, onRetry: () -> Unit, modifier: Modifier = Modifier, content: @Composable (T) -> Unit) {
    when (state) {
        LoadState.Loading -> LoadingPane(modifier)
        is LoadState.Failed -> ErrorPane(state.error, onRetry, modifier)
        is LoadState.Ready -> content(state.value)
    }
}
