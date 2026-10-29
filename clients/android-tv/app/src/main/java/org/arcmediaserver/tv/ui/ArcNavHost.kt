package org.arcmediaserver.tv.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import kotlinx.serialization.Serializable
import org.arcmediaserver.tv.ArcApplication
import org.arcmediaserver.tv.ui.browse.BrowseRoute
import org.arcmediaserver.tv.ui.components.LocalImageUrl
import org.arcmediaserver.tv.ui.details.DetailsRoute
import org.arcmediaserver.tv.ui.launch.Destination
import org.arcmediaserver.tv.ui.launch.LaunchRoute
import org.arcmediaserver.tv.ui.player.PlayerRoute
import org.arcmediaserver.tv.ui.profiles.ProfilesRoute
import org.arcmediaserver.tv.ui.server.ServerRoute
import org.arcmediaserver.tv.ui.signin.SignInRoute

@Serializable object LaunchDest
@Serializable object ServerDest
@Serializable object SignInDest
@Serializable object ProfilesDest
@Serializable object BrowseDest
@Serializable data class DetailsDest(val itemId: String)
@Serializable data class PlayerDest(val itemId: String, val startMs: Long)

/** Goes somewhere as a fresh start: nothing to go back to (after signing in, or out). */
private fun NavHostController.startAt(route: Any) = navigate(route) { popUpTo(0) { inclusive = true } }

private fun NavHostController.go(destination: Destination) = startAt(
    when (destination) {
        Destination.Server -> ServerDest
        Destination.SignIn -> SignInDest
        Destination.Profiles -> ProfilesDest
        Destination.Home -> BrowseDest
    },
)

@Composable
fun ArcNavHost() {
    val nav = rememberNavController()
    val container = (LocalContext.current.applicationContext as ArcApplication).container
    CompositionLocalProvider(LocalImageUrl provides { path -> container.api()?.absolute(path) ?: path }) {
        NavHost(nav, startDestination = LaunchDest) {
            composable<LaunchDest> { LaunchRoute(onDestination = nav::go, onChangeServer = { nav.startAt(ServerDest) }) }
            composable<ServerDest> { ServerRoute(onConnected = { nav.startAt(SignInDest) }) }
            composable<SignInDest> { SignInRoute(onSignedIn = nav::go, onChangeServer = { nav.startAt(ServerDest) }) }
            composable<ProfilesDest> {
                ProfilesRoute(onChosen = { nav.startAt(BrowseDest) })
            }
            composable<BrowseDest> {
                BrowseRoute(
                    onOpen = { item -> nav.navigate(DetailsDest(item.id)) },
                    onSwitchProfile = { nav.navigate(ProfilesDest) },
                    onSignedOut = { nav.startAt(SignInDest) },
                )
            }
            composable<DetailsDest> { entry ->
                val dest = entry.toRoute<DetailsDest>()
                DetailsRoute(
                    itemId = dest.itemId,
                    onPlay = { item, start -> nav.navigate(PlayerDest(item.id, start)) },
                    onOpen = { id -> nav.navigate(DetailsDest(id)) },
                )
            }
            composable<PlayerDest> { entry ->
                val dest = entry.toRoute<PlayerDest>()
                PlayerRoute(
                    dest.itemId,
                    dest.startMs,
                    onClose = { nav.popBackStack() },
                    // The next episode replaces this one, so Back still returns to where play began.
                    onPlayNext = { next -> nav.navigate(PlayerDest(next.id, next.resumeMs)) { popUpTo<PlayerDest> { inclusive = true } } },
                )
            }
        }
    }
}
