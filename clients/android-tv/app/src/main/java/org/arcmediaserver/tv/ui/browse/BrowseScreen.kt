package org.arcmediaserver.tv.ui.browse

import androidx.compose.animation.Crossfade
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectableGroup
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.tv.material3.DrawerValue
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.ModalNavigationDrawer
import androidx.tv.material3.NavigationDrawerItem
import androidx.tv.material3.NavigationDrawerItemDefaults
import androidx.tv.material3.NavigationDrawerScope
import androidx.tv.material3.Text
import androidx.tv.material3.rememberDrawerState
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.ui.components.requestWhenReady
import org.arcmediaserver.tv.data.CardStyle
import org.arcmediaserver.tv.data.HomeRowSpec
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.Library
import org.arcmediaserver.tv.data.MediaCard
import org.arcmediaserver.tv.data.RowSource
import org.arcmediaserver.tv.data.cardFor
import org.arcmediaserver.tv.ui.arcViewModel
import org.arcmediaserver.tv.ui.components.ArcIcons
import org.arcmediaserver.tv.ui.components.Artwork
import org.arcmediaserver.tv.ui.components.LoadState
import org.arcmediaserver.tv.ui.components.Loaded
import org.arcmediaserver.tv.ui.components.LoadingPane
import org.arcmediaserver.tv.ui.components.MediaCardView
import org.arcmediaserver.tv.ui.components.PosterWidth
import org.arcmediaserver.tv.ui.components.Spinner
import org.arcmediaserver.tv.ui.components.episodeCode
import org.arcmediaserver.tv.ui.components.text
import org.arcmediaserver.tv.ui.profiles.Avatar
import org.arcmediaserver.tv.ui.theme.ArcColors

@Composable
fun BrowseRoute(onOpen: (Item) -> Unit, onSwitchProfile: () -> Unit, onSignedOut: () -> Unit) {
    val vm = arcViewModel { BrowseViewModel(it) }
    val state by vm.state.collectAsStateWithLifecycle()
    BrowseScreen(
        state = state,
        onSection = vm::show,
        onOpen = onOpen,
        onRetry = vm::load,
        onRetryRow = vm::retryRow,
        onRetryLibrary = { (state.section as? Section.Library)?.let { vm.loadLibrary(it.id) } },
        onNearEnd = vm::nearEnd,
        onSwitchProfile = onSwitchProfile,
        onSignOut = { vm.signOut(onSignedOut) },
    )
}

@Composable
fun BrowseScreen(
    state: BrowseUiState,
    onSection: (Section) -> Unit,
    onOpen: (Item) -> Unit,
    onRetry: () -> Unit,
    onRetryRow: (HomeRowSpec) -> Unit,
    onRetryLibrary: () -> Unit,
    onNearEnd: () -> Unit,
    onSwitchProfile: () -> Unit,
    onSignOut: () -> Unit,
    drawerValue: DrawerValue = DrawerValue.Closed,
) {
    Box(Modifier.fillMaxSize().background(ArcColors.Background)) {
        Loaded(state.frame, onRetry) { frame ->
            val drawer = rememberDrawerState(drawerValue)
            ModalNavigationDrawer(
                drawerState = drawer,
                scrimBrush = Brush.horizontalGradient(listOf(ArcColors.Background.copy(alpha = 0.92f), Color.Transparent)),
                drawerContent = { value ->
                    Sidebar(frame, state.section, value, onSection, onSwitchProfile, onSignOut)
                },
            ) {
                Box(Modifier.fillMaxSize().padding(start = 72.dp)) {
                    Crossfade(state.section, label = "section") { section ->
                        when (section) {
                            Section.Home -> HomeContent(state.rows, onOpen, onRetryRow)
                            is Section.Library -> LibraryContent(state.library, onOpen, onRetryLibrary, onNearEnd)
                        }
                    }
                }
            }
        }
    }
}

/** The left rail, as in Plex and Google TV: icons that open into labels when focused. */
@Composable
private fun NavigationDrawerScope.Sidebar(
    frame: BrowseFrame,
    section: Section,
    value: DrawerValue,
    onSection: (Section) -> Unit,
    onSwitchProfile: () -> Unit,
    onSignOut: () -> Unit,
) {
    Column(
        Modifier.fillMaxHeight().background(if (value == DrawerValue.Open) ArcColors.Surface else Color.Transparent).padding(12.dp).selectableGroup(),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.padding(start = 8.dp, top = 12.dp, bottom = 20.dp)) {
            Icon(painterResource(R.drawable.arc_mark), contentDescription = stringResource(R.string.app_name), tint = Color.Unspecified, modifier = Modifier.size(32.dp))
        }
        SidebarItem(ArcIcons.Home, stringResource(R.string.nav_home), section == Section.Home) { onSection(Section.Home) }
        frame.libraries.forEach { lib ->
            SidebarItem(libraryIcon(lib), lib.name, section == Section.Library(lib.id)) { onSection(Section.Library(lib.id)) }
        }
        Spacer(Modifier.weight(1f))
        val profile = frame.profile
        if (profile != null && frame.canSwitchProfile) {
            NavigationDrawerItem(
                selected = false,
                onClick = onSwitchProfile,
                leadingContent = { Avatar(profile, 26.dp) },
                supportingContent = { Text(stringResource(R.string.nav_switch_profile)) },
                colors = itemColors(),
            ) { Text(profile.name, maxLines = 1, overflow = TextOverflow.Ellipsis) }
        }
        SidebarItem(ArcIcons.SignOut, stringResource(R.string.nav_sign_out), false, onSignOut)
    }
}

@Composable
private fun itemColors() = NavigationDrawerItemDefaults.colors(
    focusedContainerColor = ArcColors.Foreground,
    focusedContentColor = ArcColors.Background,
    selectedContainerColor = ArcColors.Surface3,
    selectedContentColor = ArcColors.Foreground,
    contentColor = ArcColors.Muted,
)

@Composable
private fun NavigationDrawerScope.SidebarItem(icon: ImageVector, label: String, selected: Boolean, onClick: () -> Unit) {
    NavigationDrawerItem(
        selected = selected,
        onClick = onClick,
        leadingContent = { Icon(icon, contentDescription = null, modifier = Modifier.size(22.dp)) },
        colors = itemColors(),
    ) { Text(label, maxLines = 1, overflow = TextOverflow.Ellipsis) }
}

fun libraryIcon(lib: Library): ImageVector = when (lib.kind) {
    "shows" -> ArcIcons.Tv
    "videos" -> ArcIcons.Folder
    else -> ArcIcons.Movie
}

@Composable
fun rowTitle(spec: HomeRowSpec): String = when (val s = spec.source) {
    is RowSource.Continue -> stringResource(R.string.row_continue)
    RowSource.NextUp -> stringResource(R.string.row_next_up)
    RowSource.Watchlist -> stringResource(R.string.row_watchlist)
    is RowSource.Recent -> stringResource(R.string.row_recent, s.library.name)
    is RowSource.Custom -> s.title
}

/**
 * Home: the focused title's artwork and details at the top (as Plex and Google TV show them), and
 * the profile's rows below. Empty rows are left out, as Plex does.
 */
@Composable
fun HomeContent(rows: List<HomeRowState>, onOpen: (Item) -> Unit, onRetryRow: (HomeRowSpec) -> Unit) {
    var focused by remember { mutableStateOf<MediaCard?>(null) }
    val visible = rows.filter { r -> !(r.cards is LoadState.Ready && r.cards.value.isEmpty()) }
    if (rows.isNotEmpty() && rows.all { it.cards is LoadState.Ready && it.cards.value.isEmpty() }) {
        EmptyHome()
        return
    }
    if (rows.isEmpty()) {
        LoadingPane()
        return
    }
    val firstCard = remember { FocusRequester() }
    var focusedOnce by remember { mutableStateOf(false) }
    Box(Modifier.fillMaxSize()) {
        Hero(focused ?: visible.firstNotNullOfOrNull { (it.cards as? LoadState.Ready)?.value?.firstOrNull() })
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(top = 230.dp, bottom = 48.dp),
            verticalArrangement = Arrangement.spacedBy(26.dp),
        ) {
            items(visible, key = { it.spec.id }) { row ->
                val isFirst = row === visible.firstOrNull { it.cards is LoadState.Ready }
                HomeRow(
                    row,
                    onOpen = onOpen,
                    onFocused = { focused = it },
                    onRetry = { onRetryRow(row.spec) },
                    firstCard = if (isFirst) firstCard else null,
                )
            }
        }
    }
    val ready = visible.any { it.cards is LoadState.Ready }
    LaunchedEffect(ready) {
        if (ready && !focusedOnce) {
            focusedOnce = firstCard.requestWhenReady()
        }
    }
}

@Composable
private fun EmptyHome() {
    Column(Modifier.fillMaxSize(), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
        Text(stringResource(R.string.home_empty_title), style = MaterialTheme.typography.headlineSmall)
        Spacer(Modifier.height(8.dp))
        Text(stringResource(R.string.home_empty_body), style = MaterialTheme.typography.bodyLarge, color = ArcColors.Muted)
    }
}

/** The focused title, large: its backdrop fading into the page, its name and what it's about. */
@Composable
private fun Hero(card: MediaCard?) {
    if (card == null) return
    val item = card.item
    Box(Modifier.fillMaxWidth().height(330.dp)) {
        Crossfade(card.key, label = "hero") { _ ->
            Box(Modifier.fillMaxSize()) {
                Artwork(
                    url = item.images?.backdropFull ?: item.images?.backdrop ?: item.images?.seriesBackdrop,
                    label = "",
                    modifier = Modifier.align(Alignment.TopEnd).fillMaxHeight().fillMaxWidth(0.72f),
                )
            }
        }
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0.25f to ArcColors.Background, 0.75f to ArcColors.Background.copy(alpha = 0.2f), 1f to Color.Transparent)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to ArcColors.Background)))
        Column(Modifier.padding(start = 48.dp, top = 40.dp).width(460.dp)) {
            Text(
                if (item.isEpisode) item.seriesTitle ?: item.title else item.title,
                style = MaterialTheme.typography.displaySmall,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.height(6.dp))
            val meta = listOfNotNull(
                if (item.isEpisode) listOfNotNull(episodeCode(item), item.title).joinToString(" · ") else null,
                item.year?.toString()?.takeIf { !item.isEpisode },
                item.contentRating,
                item.runtimeMin?.takeIf { it > 0 }?.let { runtimeText(it) },
            ).filter { it.isNotBlank() }
            if (meta.isNotEmpty()) Text(meta.joinToString("  ·  "), style = MaterialTheme.typography.titleSmall, color = ArcColors.Muted, maxLines = 1)
            Spacer(Modifier.height(10.dp))
            item.overview?.let {
                Text(it, style = MaterialTheme.typography.bodyMedium, color = ArcColors.Foreground.copy(alpha = 0.85f), maxLines = 3, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

@Composable
fun runtimeText(minutes: Int): String =
    if (minutes >= 60) stringResource(R.string.hours_minutes, minutes / 60, minutes % 60) else stringResource(R.string.minutes, minutes)

@Composable
private fun HomeRow(row: HomeRowState, onOpen: (Item) -> Unit, onFocused: (MediaCard) -> Unit, onRetry: () -> Unit, firstCard: FocusRequester?) {
    Column {
        Text(rowTitle(row.spec), style = MaterialTheme.typography.titleLarge, modifier = Modifier.padding(start = 48.dp, bottom = 12.dp))
        when (val cards = row.cards) {
            LoadState.Loading -> Box(Modifier.padding(start = 48.dp).height(120.dp), contentAlignment = Alignment.CenterStart) { Spinner(size = 28.dp) }
            is LoadState.Failed -> Row(Modifier.padding(start = 48.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(stringResource(R.string.row_failed) + " " + cards.error.text(), style = MaterialTheme.typography.bodyMedium, color = ArcColors.Muted, modifier = Modifier.weight(1f, fill = false))
                Spacer(Modifier.width(16.dp))
                androidx.tv.material3.OutlinedButton(onClick = onRetry) { Text(stringResource(R.string.action_retry)) }
            }
            is LoadState.Ready -> LazyRow(
                contentPadding = PaddingValues(horizontal = 48.dp, vertical = 10.dp),
                horizontalArrangement = Arrangement.spacedBy(20.dp),
            ) {
                items(cards.value, key = { it.key }) { card ->
                    val mod = Modifier.onFocusChanged { if (it.hasFocus) onFocused(card) }
                    MediaCardView(
                        card,
                        landscape = row.spec.style == CardStyle.Thumb,
                        onClick = { onOpen(card.item) },
                        modifier = if (firstCard != null && card === cards.value.first()) mod.focusRequester(firstCard) else mod,
                    )
                }
            }
        }
    }
}

/** A library's titles in a grid, A to Z, loading more as the focus nears the end. */
@Composable
fun LibraryContent(state: LoadState<LibraryPage>, onOpen: (Item) -> Unit, onRetry: () -> Unit, onNearEnd: () -> Unit) {
    Loaded(state, onRetry) { page ->
        val first = remember { FocusRequester() }
        Column(Modifier.fillMaxSize()) {
            Row(Modifier.padding(start = 48.dp, top = 36.dp, bottom = 8.dp), verticalAlignment = Alignment.Bottom) {
                Text(page.library.name, style = MaterialTheme.typography.displaySmall)
                Spacer(Modifier.width(14.dp))
                Text(pluralStringResource(R.plurals.library_count, page.total, page.total), style = MaterialTheme.typography.titleMedium, color = ArcColors.Muted, modifier = Modifier.padding(bottom = 6.dp))
            }
            if (page.items.isEmpty()) {
                Text(stringResource(R.string.library_empty), style = MaterialTheme.typography.bodyLarge, color = ArcColors.Muted, modifier = Modifier.padding(start = 48.dp, top = 24.dp))
                return@Column
            }
            val landscape = page.library.kind == "videos"
            LazyVerticalGrid(
                columns = GridCells.Adaptive(if (landscape) 220.dp else 112.dp),
                contentPadding = PaddingValues(start = 48.dp, end = 48.dp, top = 16.dp, bottom = 48.dp),
                horizontalArrangement = Arrangement.spacedBy(22.dp),
                verticalArrangement = Arrangement.spacedBy(26.dp),
            ) {
                itemsIndexed(page.items, key = { _, it -> it.id }) { i, item ->
                    val mod = Modifier.fillMaxWidth().onFocusChanged { if (it.hasFocus && i >= page.items.size - 18) onNearEnd() }
                    MediaCardView(cardFor(item), landscape = landscape, onClick = { onOpen(item) }, modifier = if (i == 0) mod.focusRequester(first) else mod)
                }
            }
        }
        LaunchedEffect(page.library.id) { first.requestWhenReady() }
    }
}
