package org.arcmediaserver.tv.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Border
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import coil3.compose.AsyncImage
import org.arcmediaserver.tv.R
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.MediaCard
import org.arcmediaserver.tv.ui.theme.ArcColors

/** Turns an address Arc gave for artwork into one to load (the server's, under /api/v1). */
val LocalImageUrl = compositionLocalOf<(String) -> String> { { it } }

val CardShape = RoundedCornerShape(10.dp)
val PosterWidth = 132.dp
val ThumbWidth = 236.dp

/**
 * Artwork with a quiet placeholder underneath: the title on a dark gradient, which is what shows
 * for titles Arc couldn't identify (no artwork) and while the picture loads.
 */
@Composable
fun Artwork(url: String?, label: String, modifier: Modifier = Modifier, contentScale: ContentScale = ContentScale.Crop) {
    val resolve = LocalImageUrl.current
    Box(modifier.background(Brush.linearGradient(listOf(ArcColors.Surface3, ArcColors.Surface)))) {
        Text(
            label,
            style = MaterialTheme.typography.titleSmall,
            color = ArcColors.Muted,
            textAlign = TextAlign.Center,
            maxLines = 3,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.align(Alignment.Center).padding(12.dp),
        )
        if (url != null) {
            AsyncImage(model = resolve(url), contentDescription = null, contentScale = contentScale, modifier = Modifier.fillMaxSize())
        }
    }
}

/**
 * A focusable card: it grows a little, takes an accent outline and a soft glow when focused,
 * as cards do on Google TV and in Plex.
 */
@Composable
fun FocusCard(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    onFocused: (Boolean) -> Unit = {},
    content: @Composable BoxScope.() -> Unit,
) {
    Card(
        onClick = onClick,
        modifier = modifier.onFocusChanged { onFocused(it.isFocused) },
        shape = CardDefaults.shape(CardShape),
        scale = CardDefaults.scale(focusedScale = 1.08f),
        border = CardDefaults.border(focusedBorder = Border(BorderStroke(3.dp, ArcColors.Foreground), shape = CardShape)),
        glow = CardDefaults.glow(focusedGlow = Glow(elevationColor = ArcColors.AccentStrong.copy(alpha = 0.55f), elevation = 18.dp)),
        colors = CardDefaults.colors(containerColor = ArcColors.Surface, focusedContainerColor = ArcColors.Surface),
    ) {
        Box(Modifier.fillMaxWidth()) { content() }
    }
}

/** How far through a title is, along the bottom of its card. */
@Composable
fun BoxScope.ProgressLine(progress: Float?) {
    if (progress == null) return
    Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().height(4.dp).background(Color.Black.copy(alpha = 0.55f))) {
        Box(Modifier.fillMaxWidth(progress).height(4.dp).background(ArcColors.Accent))
    }
}

/** A tick in the corner for something watched; the number of episodes left for a show. */
@Composable
fun BoxScope.WatchBadge(item: Item) {
    val unwatched = item.unwatchedCount ?: 0
    when {
        item.isSeries && unwatched > 0 -> Box(
            Modifier.align(Alignment.TopEnd).padding(6.dp).clip(RoundedCornerShape(6.dp)).background(ArcColors.AccentStrong).padding(horizontal = 7.dp, vertical = 2.dp),
        ) { Text(unwatched.toString(), style = MaterialTheme.typography.labelMedium, color = Color.White) }
        item.userData?.played == true -> Box(
            Modifier.align(Alignment.TopEnd).padding(6.dp).size(22.dp).clip(CircleShape).background(ArcColors.AccentStrong),
            contentAlignment = Alignment.Center,
        ) { Icon(ArcIcons.Check, contentDescription = stringResource(R.string.watched), modifier = Modifier.size(15.dp), tint = Color.White) }
    }
}

/** The line under a card: "S1 · E3" for an episode, "Season 2 · 3 new episodes", or the year. */
@Composable
fun cardSubtitle(card: MediaCard): String? {
    val item = card.item
    return when {
        card.seasonNumber != null -> stringResource(R.string.season_number, card.seasonNumber) + " · " +
            pluralStringResource(R.plurals.new_episodes, card.newEpisodes, card.newEpisodes)
        item.isEpisode -> episodeCode(item)
        else -> item.year?.toString()
    }
}

@Composable
fun episodeCode(item: Item): String? {
    val s = item.seasonNumber ?: return null
    val e = item.episodeNumber ?: return stringResource(R.string.season_number, s)
    return stringResource(R.string.episode_code, s, e)
}

/** A card's title: the show's for an episode or a season, as Plex labels them on Home. */
fun cardTitle(card: MediaCard): String = if (card.item.isEpisode) card.item.seriesTitle ?: card.item.title else card.item.title

@Composable
fun MediaCardView(card: MediaCard, landscape: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier) {
    var focused by remember { mutableStateOf(false) }
    val width: Dp = if (landscape) ThumbWidth else PosterWidth
    Column(modifier.width(width)) {
        FocusCard(onClick = onClick, onFocused = { focused = it }) {
            Artwork(
                url = if (landscape) card.thumb else card.poster,
                label = cardTitle(card),
                modifier = Modifier.fillMaxWidth().aspectRatio(if (landscape) 16f / 9f else 2f / 3f),
            )
            WatchBadge(card.item)
            ProgressLine(card.progress)
        }
        Spacer(Modifier.height(10.dp))
        Text(
            cardTitle(card),
            style = MaterialTheme.typography.titleSmall,
            color = if (focused) ArcColors.Foreground else ArcColors.Foreground.copy(alpha = 0.85f),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        val sub = cardSubtitle(card)
        val line = if (card.item.isEpisode && landscape) listOfNotNull(sub, card.item.title.takeIf { it != cardTitle(card) }).joinToString(" · ") else sub
        if (!line.isNullOrEmpty()) {
            Text(line, style = MaterialTheme.typography.bodySmall, color = ArcColors.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}
