package org.arcmediaserver.tv.data

import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

/** What a Home row shows. Built-in rows are named by the app; custom rows keep their own title. */
sealed interface RowSource {
    data class Continue(val withUpNext: Boolean) : RowSource
    data object NextUp : RowSource
    data object Watchlist : RowSource
    data class Recent(val library: Library, val showsOnly: Boolean) : RowSource
    data class Custom(val title: String, val library: Library, val filter: JsonObject) : RowSource
}

enum class CardStyle { Poster, Thumb }

data class HomeRowSpec(val id: String, val source: RowSource, val style: CardStyle)

/**
 * The profile's Home rows in its order, as the web app arranges them (web/src/lib/home.ts):
 * the saved layout first, then built-in rows it hasn't placed where they'd be by default. Rows the
 * TV app can't show yet (seasonal, collections, playlists) are left out; hidden rows too.
 */
fun resolveHomeRows(prefs: Prefs, libraries: List<Library>): List<HomeRowSpec> {
    val out = mutableListOf<Pair<HomeRowSpec?, String>>() // spec (null: placed but not shown here), id
    val placed = mutableSetOf<String>()
    fun builtIn(id: String, saved: HomeRowPref?): HomeRowSpec? {
        val style = saved?.style
        fun look(default: CardStyle) = when (style) {
            "poster" -> CardStyle.Poster
            "thumb", "arc" -> CardStyle.Thumb
            else -> default
        }
        return when {
            id == "continue" -> HomeRowSpec(id, RowSource.Continue(withUpNext = !prefs.splitUpNext), look(CardStyle.Thumb))
            id == "next-up" -> if (prefs.splitUpNext) HomeRowSpec(id, RowSource.NextUp, look(CardStyle.Thumb)) else null
            id == "watchlist" -> HomeRowSpec(id, RowSource.Watchlist, look(CardStyle.Poster))
            id.startsWith("recent:") -> libraries.find { "recent:${it.id}" == id }?.let { lib ->
                HomeRowSpec(id, RowSource.Recent(lib, saved?.showsOnly == true), look(if (lib.kind == "videos") CardStyle.Thumb else CardStyle.Poster))
            }
            else -> null
        }
    }
    for (row in prefs.home) {
        placed += row.id
        val spec = when {
            row.hidden -> null
            row.id.startsWith("custom:") -> {
                val filter = row.filter
                val libId = (filter?.get("library_id") as? JsonPrimitive)?.content?.toLongOrNull()
                val lib = libraries.find { it.id == libId }
                if (filter == null || lib == null || row.title.isNullOrBlank()) null
                else HomeRowSpec(row.id, RowSource.Custom(row.title, lib, filter), if (row.style == "thumb" || row.style == "arc" || lib.kind == "videos") CardStyle.Thumb else CardStyle.Poster)
            }
            else -> builtIn(row.id, row)
        }
        out += spec to row.id
    }
    val defaults = listOf("continue", "next-up", "watchlist", "seasonal") + libraries.map { "recent:${it.id}" }
    defaults.forEachIndexed { i, id ->
        if (id in placed) return@forEachIndexed
        val spec = builtIn(id, null)
        // After the row that comes before it by default, or first.
        val prev = defaults.subList(0, i).asReversed().firstOrNull { p -> out.any { it.second == p } }
        val at = if (prev == null) 0 else out.indexOfFirst { it.second == prev } + 1
        out.add(at, spec to id)
    }
    return out.mapNotNull { it.first }
}

/** A card on a row: a title, or a season of a show with new episodes. */
data class MediaCard(
    val key: String,
    val item: Item,
    /** A season of new episodes (Recently Added in a TV library): its number and how many. */
    val seasonNumber: Int? = null,
    val newEpisodes: Int = 0,
    val poster: String?,
    val thumb: String?,
    /** How far through, 0–1, or null when not started. */
    val progress: Float?,
)

/** Turns an item into a card, choosing artwork as the web's PosterCard and EpisodeCard do. */
fun cardFor(item: Item, key: String = item.id): MediaCard {
    val img = item.images
    val poster = if (item.isEpisode) img?.seriesPrimary ?: img?.primary else img?.primary
    val thumb = if (item.isEpisode) img?.primary ?: img?.seriesBackdrop ?: img?.backdrop else img?.backdrop ?: img?.primary
    val duration = item.durationMs
    val progress = if (item.resumeMs > 0 && duration > 0) (item.resumeMs.toFloat() / duration).coerceIn(0.02f, 1f) else null
    return MediaCard(
        key = key,
        item = item,
        poster = poster,
        thumb = thumb,
        progress = progress,
    )
}

/** A TV library's Recently Added card: a new show, new episodes of a season, or one episode. */
fun cardFor(card: RecentCard): MediaCard {
    val base = cardFor(card.item, key = "${card.type}:${card.item.id}:${card.seasonNumber ?: ""}")
    return if (card.type == "season") {
        base.copy(seasonNumber = card.seasonNumber, newEpisodes = card.episodes, poster = card.seasonPoster ?: base.poster)
    } else {
        base
    }
}

/** /items query parameters for a custom row's filter (web/src/lib/home.ts rowQuery). */
fun customRowQuery(filter: JsonObject, library: Library, limit: Int = 24): Map<String, String> {
    val keys = listOf(
        "library_id", "genre", "language", "label", "decade", "resolution", "unwatched", "added_within",
        "min_runtime", "max_runtime", "min_rating", "sort",
    )
    val out = linkedMapOf<String, String>()
    for (k in keys) {
        val v = (filter[k] as? JsonPrimitive)?.asQueryValue() ?: continue
        if (v.isNotEmpty() && v != "0") out[k] = v
    }
    out["kind"] = if (library.kind == "shows") "series" else "movie"
    out["limit"] = limit.toString()
    return out
}
