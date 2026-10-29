package org.arcmediaserver.tv.screenshots

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalInspectionMode
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.test.ext.junit.runners.AndroidJUnit4
import coil3.asImage
import coil3.compose.AsyncImagePreviewHandler
import coil3.compose.LocalAsyncImagePreviewHandler
import com.github.takahirom.roborazzi.captureRoboImage
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.ui.browse.BrowseScreen
import org.arcmediaserver.tv.ui.browse.BrowseUiState
import org.arcmediaserver.tv.ui.browse.LibraryPage
import org.arcmediaserver.tv.ui.browse.Section
import org.arcmediaserver.tv.ui.components.LoadState
import org.arcmediaserver.tv.ui.components.UiError
import org.arcmediaserver.tv.ui.details.Details
import org.arcmediaserver.tv.ui.details.DetailsScreen
import org.arcmediaserver.tv.ui.player.PlayerControls
import org.arcmediaserver.tv.ui.player.PlayerUiState
import org.arcmediaserver.tv.ui.profiles.PinPrompt
import org.arcmediaserver.tv.ui.profiles.ProfilesScreen
import org.arcmediaserver.tv.ui.profiles.ProfilesUiState
import org.arcmediaserver.tv.ui.server.ServerProblem
import org.arcmediaserver.tv.ui.server.ServerScreen
import org.arcmediaserver.tv.ui.server.ServerUiState
import org.arcmediaserver.tv.ui.signin.SignInScreen
import org.arcmediaserver.tv.ui.signin.SignInUiState
import org.arcmediaserver.tv.ui.theme.ArcTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Renders each screen at a TV's 1920x1080 (960x540dp at xhdpi) on the JVM, into
 * app/screenshots/tv-*.png: `./gradlew recordRoborazziDebug`. Look at them after UI changes.
 */
@OptIn(coil3.annotation.ExperimentalCoilApi::class)
@RunWith(AndroidJUnit4::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [36], qualifiers = "w960dp-h540dp-land-television-xhdpi")
class ScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private fun shot(name: String, content: @Composable () -> Unit) {
        compose.setContent {
            CompositionLocalProvider(
                LocalInspectionMode provides true,
                LocalAsyncImagePreviewHandler provides AsyncImagePreviewHandler { request -> Fixtures.art(request.data.toString()).asImage() },
            ) {
                ArcTheme { Box(Modifier.fillMaxSize().background(org.arcmediaserver.tv.ui.theme.ArcColors.Background)) { content() } }
            }
        }
        compose.waitForIdle()
        // Let focus land (lazy rows compose their items a frame later) and the focus animation finish.
        compose.mainClock.advanceTimeBy(1_500)
        compose.waitForIdle()
        compose.onRoot().captureRoboImage("screenshots/tv-$name.png")
    }

    @Test
    fun server() = shot("server") {
        ServerScreen(ServerUiState(address = "192.168.1.20"), onEdit = {}, onConnect = {})
    }

    @Test
    fun serverError() = shot("server-error") {
        ServerScreen(ServerUiState(address = "192.168.1.99", problem = ServerProblem.Failed(UiError.Unreachable)), onEdit = {}, onConnect = {})
    }

    @Test
    fun signIn() = shot("sign-in") {
        SignInScreen(SignInUiState(server = "http://192.168.1.20:8099", code = "K7QF"), onNewCode = {}, onChangeServer = {})
    }

    @Test
    fun profiles() = shot("profiles") {
        ProfilesScreen(ProfilesUiState(profiles = LoadState.Ready(Fixtures.profiles)), onChoose = {}, onRetry = {}, onDigit = {}, onDelete = {})
    }

    @Test
    fun pin() = shot("pin") {
        ProfilesScreen(
            ProfilesUiState(profiles = LoadState.Ready(Fixtures.profiles), pin = PinPrompt(Fixtures.profiles[1], digits = "12")),
            onChoose = {}, onRetry = {}, onDigit = {}, onDelete = {},
        )
    }

    @Composable
    private fun browse(state: BrowseUiState, drawerOpen: Boolean = false) {
        BrowseScreen(
            state, onSection = {}, onOpen = {}, onRetry = {}, onRetryRow = {}, onRetryLibrary = {}, onNearEnd = {},
            onSwitchProfile = {}, onSignOut = {},
            drawerValue = if (drawerOpen) androidx.tv.material3.DrawerValue.Open else androidx.tv.material3.DrawerValue.Closed,
        )
    }

    @Test
    fun home() = shot("home") {
        browse(BrowseUiState(frame = LoadState.Ready(Fixtures.frame), rows = Fixtures.homeRows))
    }

    @Test
    fun homeSidebar() = shot("home-sidebar") {
        browse(BrowseUiState(frame = LoadState.Ready(Fixtures.frame), rows = Fixtures.homeRows), drawerOpen = true)
    }

    @Test
    fun library() = shot("library") {
        browse(
            BrowseUiState(
                frame = LoadState.Ready(Fixtures.frame),
                section = Section.Library(1),
                library = LoadState.Ready(LibraryPage(Fixtures.libraries[0], Fixtures.movies + Fixtures.movies.map { it.copy(id = it.id + "b", title = it.title + " II") }, 16)),
            ),
        )
    }

    @Test
    fun homeError() = shot("home-error") {
        browse(BrowseUiState(frame = LoadState.Failed(UiError.Timeout)))
    }

    @Test
    fun movie() = shot("movie") {
        DetailsScreen(Details(Fixtures.movies[0]), onPlay = { _: Item, _: Long -> }, onOpen = {})
    }

    @Test
    fun show() = shot("show-episodes") {
        DetailsScreen(
            Details(Fixtures.show, Fixtures.seasons, next = Fixtures.seasons[0].episodes[2], initialSeason = 0),
            onPlay = { _: Item, _: Long -> },
            onOpen = {},
        )
    }

    @Test
    fun playerControls() = shot("player") {
        Box(Modifier.fillMaxSize()) {
            androidx.compose.foundation.Image(
                bitmap = Fixtures.art("art:backdrop:Hollow Creek").asImageBitmap(),
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
                contentScale = androidx.compose.ui.layout.ContentScale.Crop,
            )
            PlayerControls(
                state = PlayerUiState(
                    item = Fixtures.seasons[0].episodes[2],
                    plan = Fixtures.plan,
                    loading = false,
                    playing = true,
                    positionMs = 21 * 60_000L + 14_000,
                    durationMs = Fixtures.plan.durationMs,
                    bufferedMs = 26 * 60_000L,
                ),
                scrubMs = null,
                onPlayPause = {}, onBack10 = {}, onForward30 = {}, onScrub = {}, onAudio = {}, onSubtitles = {},
            )
        }
    }
}
