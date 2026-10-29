package org.arcmediaserver.tv.ui.details

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import org.arcmediaserver.tv.AppContainer
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.Season
import org.arcmediaserver.tv.ui.components.LoadState
import org.arcmediaserver.tv.ui.components.toUiError

/**
 * A title's page. For a show, its seasons and episodes come in the show's display order, which
 * Arc works out (GET /series/{id}/seasons without an ordering), so the TV shows what the web does.
 */
data class Details(
    val item: Item,
    val seasons: List<Season> = emptyList(),
    /** For a show: the episode Play starts (the one part-way through, or the next unwatched). */
    val next: Item? = null,
    /** The season to open on: the next episode's. */
    val initialSeason: Int = 0,
)

/** The episode to carry on with: one part-way through, else the first not watched, specials last. */
fun nextEpisode(seasons: List<Season>): Item? {
    val ordered = seasons.filter { it.seasonNumber > 0 } + seasons.filter { it.seasonNumber <= 0 }
    val episodes = ordered.flatMap { it.episodes }.filter { it.available }
    return episodes.firstOrNull { it.resumeMs > 0 } ?: episodes.firstOrNull { it.userData?.played != true } ?: episodes.firstOrNull()
}

class DetailsViewModel(private val container: AppContainer, private val itemId: String) : ViewModel() {
    private val _state = MutableStateFlow<LoadState<Details>>(LoadState.Loading)
    val state: StateFlow<LoadState<Details>> = _state

    init {
        load()
    }

    /** Loads the page; [quiet] keeps what's shown while it refreshes (coming back from playing). */
    fun load(quiet: Boolean = false) {
        val api = container.api() ?: return
        if (!quiet || _state.value !is LoadState.Ready) _state.value = LoadState.Loading
        viewModelScope.launch {
            _state.value = try {
                val item = api.item(itemId)
                if (item.isSeries) {
                    val seasons = api.seasons(item.id).seasons.filter { it.episodes.isNotEmpty() }
                    val next = nextEpisode(seasons)
                    val idx = seasons.indexOfFirst { s -> s.episodes.any { it.id == next?.id } }.coerceAtLeast(0)
                    LoadState.Ready(Details(item, seasons, next, idx))
                } else {
                    LoadState.Ready(Details(item))
                }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                ArcLog.Android.warn("details failed to load: ${e.javaClass.simpleName}")
                if (quiet && _state.value is LoadState.Ready) return@launch
                LoadState.Failed(e.toUiError())
            }
        }
    }
}
