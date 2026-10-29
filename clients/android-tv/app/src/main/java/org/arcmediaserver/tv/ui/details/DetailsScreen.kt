package org.arcmediaserver.tv.ui.details

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.OutlinedButton
import androidx.tv.material3.Tab
import androidx.tv.material3.TabDefaults
import androidx.tv.material3.TabRow
import androidx.tv.material3.TabRowDefaults
import androidx.tv.material3.Text
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.Season
import org.arcmediaserver.tv.data.cardFor
import org.arcmediaserver.tv.ui.arcViewModel
import org.arcmediaserver.tv.ui.browse.runtimeText
import org.arcmediaserver.tv.ui.components.ArcIcons
import org.arcmediaserver.tv.ui.components.Artwork
import org.arcmediaserver.tv.ui.components.FocusCard
import org.arcmediaserver.tv.ui.components.Loaded
import org.arcmediaserver.tv.ui.components.ProgressLine
import org.arcmediaserver.tv.ui.components.ThumbWidth
import org.arcmediaserver.tv.ui.components.WatchBadge
import org.arcmediaserver.tv.ui.components.episodeCode
import org.arcmediaserver.tv.ui.theme.ArcColors

@Composable
fun DetailsRoute(itemId: String, onPlay: (Item, Long) -> Unit, onOpen: (String) -> Unit) {
    val vm = arcViewModel { DetailsViewModel(it, itemId) }
    val state by vm.state.collectAsStateWithLifecycle()
    var resumed by remember { mutableStateOf(false) }
    // Coming back from the player: show the new resume point and watched state.
    LifecycleResumeEffect(Unit) {
        if (resumed) vm.load(quiet = true)
        resumed = true
        onPauseOrDispose { }
    }
    Box(Modifier.fillMaxSize().background(ArcColors.Background)) {
        Loaded(state, onRetry = { vm.load() }) { details ->
            DetailsScreen(details, onPlay, onOpen)
        }
    }
}

@Composable
fun DetailsScreen(details: Details, onPlay: (Item, Long) -> Unit, onOpen: (String) -> Unit) {
    val item = details.item
    val primary = remember { FocusRequester() }
    Box(Modifier.fillMaxSize().background(ArcColors.Background)) {
        Artwork(
            url = item.images?.backdropFull ?: item.images?.backdrop ?: item.images?.seriesBackdrop,
            label = "",
            modifier = Modifier.align(Alignment.TopEnd).fillMaxWidth(0.78f).fillMaxHeight(0.85f),
        )
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0.18f to ArcColors.Background, 0.62f to ArcColors.Background.copy(alpha = 0.35f), 1f to Color.Transparent)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.35f to Color.Transparent, 0.85f to ArcColors.Background)))
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(top = 56.dp, bottom = 40.dp)) {
            item { Header(details, onPlay, onOpen, primary) }
            if (item.isSeries) {
                item { Seasons(details, onPlay) }
            }
        }
    }
    LaunchedEffect(item.id) { runCatching { primary.requestFocus() } }
}

@Composable
private fun Header(details: Details, onPlay: (Item, Long) -> Unit, onOpen: (String) -> Unit, primary: FocusRequester) {
    val item = details.item
    Column(Modifier.padding(start = 56.dp).width(520.dp)) {
        if (item.isEpisode) {
            Text(item.seriesTitle.orEmpty(), style = MaterialTheme.typography.titleLarge, color = ArcColors.Muted)
            Spacer(Modifier.height(4.dp))
        }
        Text(item.title, style = MaterialTheme.typography.displayMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
        Spacer(Modifier.height(10.dp))
        val unwatched = item.unwatchedCount ?: 0
        val meta = listOfNotNull(
            if (item.isEpisode) episodeCode(item) else null,
            item.year?.toString(),
            item.contentRating,
            item.runtimeMin?.takeIf { it > 0 && !item.isSeries }?.let { runtimeText(it) },
            if (item.isSeries) pluralStringResource(R.plurals.episodes_left, unwatched, unwatched).takeIf { unwatched > 0 } else null,
            item.rating?.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
        )
        Text(meta.joinToString("  ·  "), style = MaterialTheme.typography.titleMedium, color = ArcColors.Muted)
        if (item.genres.isNotEmpty()) {
            Spacer(Modifier.height(4.dp))
            Text(item.genres.take(3).joinToString(", "), style = MaterialTheme.typography.bodyMedium, color = ArcColors.Subtle)
        }
        item.tagline?.takeIf { it.isNotBlank() }?.let {
            Spacer(Modifier.height(14.dp))
            Text(it, style = MaterialTheme.typography.titleMedium, color = ArcColors.Foreground)
        }
        item.overview?.takeIf { it.isNotBlank() }?.let {
            Spacer(Modifier.height(10.dp))
            Text(it, style = MaterialTheme.typography.bodyLarge, color = ArcColors.Foreground.copy(alpha = 0.85f), maxLines = 4, overflow = TextOverflow.Ellipsis)
        }
        Spacer(Modifier.height(24.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            val target = if (item.isSeries) details.next else item
            if (target != null) {
                val resume = target.resumeMs
                val code = if (item.isSeries) episodeCode(target) else null
                val label = when {
                    resume > 0 && code != null -> stringResource(R.string.action_resume_episode, code)
                    resume > 0 -> stringResource(R.string.action_resume, clock(resume))
                    code != null -> stringResource(R.string.action_play_episode, code)
                    else -> stringResource(R.string.action_play)
                }
                ActionButton(ArcIcons.Play, label, primary = true, modifier = Modifier.focusRequester(primary)) { onPlay(target, resume) }
                if (resume > 0) {
                    ActionButton(ArcIcons.Restart, stringResource(R.string.action_play_from_start)) { onPlay(target, 0) }
                }
            }
            if (item.isEpisode && item.parentId != null) {
                ActionButton(ArcIcons.Tv, stringResource(R.string.action_go_to_show)) { onOpen(item.parentId) }
            }
        }
    }
}

@Composable
private fun ActionButton(icon: ImageVector, label: String, primary: Boolean = false, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val content: @Composable androidx.compose.foundation.layout.RowScope.() -> Unit = {
        Icon(icon, contentDescription = null, modifier = Modifier.size(ButtonDefaults.IconSize))
        Spacer(Modifier.width(ButtonDefaults.IconSpacing))
        Text(label)
    }
    if (primary) {
        Button(onClick = onClick, modifier = modifier, contentPadding = ButtonDefaults.ButtonWithIconContentPadding, content = content)
    } else {
        OutlinedButton(onClick = onClick, modifier = modifier, contentPadding = ButtonDefaults.ButtonWithIconContentPadding, content = content)
    }
}

/** e.g. 1:02:05 or 12:34. */
fun clock(ms: Long): String {
    val s = ms / 1000
    val h = s / 3600
    val m = (s % 3600) / 60
    val sec = s % 60
    return if (h > 0) "%d:%02d:%02d".format(h, m, sec) else "%d:%02d".format(m, sec)
}

@Composable
private fun Seasons(details: Details, onPlay: (Item, Long) -> Unit) {
    if (details.seasons.isEmpty()) {
        Text(stringResource(R.string.details_no_episodes), style = MaterialTheme.typography.bodyLarge, color = ArcColors.Muted, modifier = Modifier.padding(start = 56.dp, top = 32.dp))
        return
    }
    var selected by rememberSaveable(details.item.id) { mutableIntStateOf(details.initialSeason.coerceIn(0, details.seasons.lastIndex)) }
    Column(Modifier.padding(top = 36.dp)) {
        TabRow(
            selectedTabIndex = selected,
            modifier = Modifier.padding(start = 48.dp),
            indicator = { tabPositions, doesTabRowHaveFocus ->
                TabRowDefaults.PillIndicator(
                    currentTabPosition = tabPositions[selected],
                    doesTabRowHaveFocus = doesTabRowHaveFocus,
                    activeColor = ArcColors.Foreground,
                    inactiveColor = ArcColors.Surface3,
                )
            },
        ) {
            details.seasons.forEachIndexed { i, season ->
                Tab(
                    selected = i == selected,
                    onFocus = { selected = i },
                    colors = TabDefaults.pillIndicatorTabColors(
                        contentColor = ArcColors.Muted,
                        selectedContentColor = ArcColors.Foreground,
                        focusedContentColor = ArcColors.Background,
                        focusedSelectedContentColor = ArcColors.Background,
                    ),
                ) {
                    Text(seasonName(season), style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp))
                }
            }
        }
        val season = details.seasons[selected]
        val listState = rememberLazyListState()
        LaunchedEffect(selected) {
            val idx = season.episodes.indexOfFirst { it.id == details.next?.id }
            listState.scrollToItem(idx.coerceAtLeast(0))
        }
        LazyRow(
            state = listState,
            contentPadding = PaddingValues(horizontal = 56.dp, vertical = 18.dp),
            horizontalArrangement = Arrangement.spacedBy(20.dp),
        ) {
            items(season.episodes, key = { it.id }) { ep -> EpisodeCard(ep, onClick = { onPlay(ep, ep.resumeMs) }) }
        }
    }
}

@Composable
fun seasonName(season: Season): String = season.name.takeIf { it.isNotBlank() }
    ?: if (season.seasonNumber == 0) stringResource(R.string.specials) else stringResource(R.string.season_number, season.seasonNumber)

/** An episode: its picture with progress and a tick, its number and name, and how long it is. */
@Composable
private fun EpisodeCard(ep: Item, onClick: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    val card = cardFor(ep)
    Column(Modifier.width(ThumbWidth)) {
        FocusCard(onClick = onClick, onFocused = { focused = it }) {
            Artwork(card.thumb, label = ep.title, modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f), contentScale = ContentScale.Crop)
            WatchBadge(ep)
            ProgressLine(card.progress)
        }
        Spacer(Modifier.height(10.dp))
        Text(
            listOfNotNull(ep.episodeNumber?.let { "$it." }, ep.title).joinToString(" "),
            style = MaterialTheme.typography.titleSmall,
            color = if (focused) ArcColors.Foreground else ArcColors.Foreground.copy(alpha = 0.85f),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        val sub = listOfNotNull(
            ep.runtimeMin?.takeIf { it > 0 }?.let { runtimeText(it) },
            if (!ep.available) stringResource(R.string.details_missing) else null,
        ).joinToString(" · ")
        if (sub.isNotEmpty()) Text(sub, style = MaterialTheme.typography.bodySmall, color = ArcColors.Muted)
        if (focused && !ep.overview.isNullOrBlank()) {
            Text(ep.overview, style = MaterialTheme.typography.bodySmall, color = ArcColors.Muted, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.heightIn(max = 40.dp))
        }
    }
}
