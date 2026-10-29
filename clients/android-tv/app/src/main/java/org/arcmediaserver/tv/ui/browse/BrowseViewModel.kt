package org.arcmediaserver.tv.ui.browse

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.arcmediaserver.tv.AppContainer
import org.arcmediaserver.tv.data.ArcApi
import org.arcmediaserver.tv.data.ArcLog
import org.arcmediaserver.tv.data.HomeRowSpec
import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.Library
import org.arcmediaserver.tv.data.MediaCard
import org.arcmediaserver.tv.data.Profile
import org.arcmediaserver.tv.data.RowSource
import org.arcmediaserver.tv.data.cardFor
import org.arcmediaserver.tv.data.customRowQuery
import org.arcmediaserver.tv.data.resolveHomeRows
import org.arcmediaserver.tv.ui.components.LoadState
import org.arcmediaserver.tv.ui.components.toUiError

data class HomeRowState(val spec: HomeRowSpec, val cards: LoadState<List<MediaCard>>)

/** What the sidebar lists and who's watching. */
data class BrowseFrame(val libraries: List<Library>, val profile: Profile?, val canSwitchProfile: Boolean)

data class LibraryPage(val library: Library, val items: List<Item>, val total: Int, val loadingMore: Boolean = false)

sealed interface Section {
    data object Home : Section
    data class Library(val id: Long) : Section
}

data class BrowseUiState(
    val frame: LoadState<BrowseFrame> = LoadState.Loading,
    val section: Section = Section.Home,
    val rows: List<HomeRowState> = emptyList(),
    val library: LoadState<LibraryPage> = LoadState.Loading,
)

/** Loads one Home row's cards. */
suspend fun loadRow(api: ArcApi, spec: HomeRowSpec): List<MediaCard> = when (val s = spec.source) {
    is RowSource.Continue -> api.continueWatching(s.withUpNext).map { cardFor(it) }
    RowSource.NextUp -> api.nextUp().map { cardFor(it) }
    RowSource.Watchlist -> api.watchlist().map { cardFor(it) }
    is RowSource.Recent ->
        if (s.library.kind == "shows" && !s.showsOnly) api.recentShows(s.library.id).map { cardFor(it) }
        else api.items(libraryQuery(s.library, sort = "added", limit = 24)).items.map { cardFor(it) }
    is RowSource.Custom -> api.items(customRowQuery(s.filter, s.library)).items.map { cardFor(it) }
}

fun libraryQuery(library: Library, sort: String, limit: Int, offset: Int = 0): Map<String, String> = buildMap {
    put("library_id", library.id.toString())
    put("kind", if (library.kind == "shows") "series" else "movie")
    put("sort", sort)
    put("limit", limit.toString())
    if (offset > 0) put("offset", offset.toString())
}

class BrowseViewModel(private val container: AppContainer) : ViewModel() {
    private val _state = MutableStateFlow(BrowseUiState())
    val state: StateFlow<BrowseUiState> = _state

    init {
        load()
    }

    /** Loads the sidebar and Home; called again by Retry and when coming back from playing. */
    fun load() {
        val api = container.api() ?: return
        _state.update { it.copy(frame = if (it.frame is LoadState.Ready) it.frame else LoadState.Loading) }
        viewModelScope.launch {
            try {
                val started = System.currentTimeMillis()
                val (libraries, prefs, status) = coroutineScope {
                    val libs = async { api.libraries() }
                    val prefs = async { api.prefs() }
                    val status = async { api.authStatus() }
                    Triple(libs.await(), prefs.await(), status.await())
                }
                val order = prefs.libraryOrder
                val sorted = libraries.sortedBy { lib -> order.indexOf(lib.id).let { if (it < 0) Int.MAX_VALUE else it } }
                _state.update {
                    it.copy(frame = LoadState.Ready(BrowseFrame(sorted, status.profile, canSwitchProfile = !status.friend)))
                }
                val specs = resolveHomeRows(prefs, sorted)
                val current = _state.value.rows.associateBy { it.spec.id }
                _state.update { s -> s.copy(rows = specs.map { spec -> current[spec.id]?.copy(spec = spec) ?: HomeRowState(spec, LoadState.Loading) }) }
                specs.map { spec -> async { loadRowInto(api, spec) } }.awaitAll()
                ArcLog.Android.debug("home loaded: ${specs.size} rows in ${System.currentTimeMillis() - started} ms")
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                ArcLog.Android.warn("home failed to load: ${e.javaClass.simpleName}")
                _state.update { it.copy(frame = LoadState.Failed(e.toUiError())) }
            }
        }
    }

    fun retryRow(spec: HomeRowSpec) {
        val api = container.api() ?: return
        setRow(spec, LoadState.Loading)
        viewModelScope.launch { loadRowInto(api, spec) }
    }

    private suspend fun loadRowInto(api: ArcApi, spec: HomeRowSpec) {
        val result = try {
            LoadState.Ready(loadRow(api, spec))
        } catch (e: Exception) {
            if (e is kotlinx.coroutines.CancellationException) throw e
            ArcLog.Android.warn("home row ${spec.id} failed: ${e.javaClass.simpleName}")
            LoadState.Failed(e.toUiError())
        }
        setRow(spec, result)
    }

    private fun setRow(spec: HomeRowSpec, cards: LoadState<List<MediaCard>>) = _state.update { s ->
        s.copy(rows = s.rows.map { if (it.spec.id == spec.id) it.copy(cards = cards) else it })
    }

    fun show(section: Section) {
        if (section == _state.value.section && section !is Section.Library) return
        _state.update { it.copy(section = section) }
        if (section is Section.Library) loadLibrary(section.id)
    }

    fun loadLibrary(id: Long) {
        val api = container.api() ?: return
        val lib = (state.value.frame as? LoadState.Ready)?.value?.libraries?.find { it.id == id } ?: return
        _state.update { it.copy(library = LoadState.Loading) }
        viewModelScope.launch {
            val result = try {
                val page = api.items(libraryQuery(lib, sort = "title", limit = PAGE))
                LoadState.Ready(LibraryPage(lib, page.items, page.total))
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                ArcLog.Android.warn("library ${lib.id} failed to load: ${e.javaClass.simpleName}")
                LoadState.Failed(e.toUiError())
            }
            _state.update { it.copy(library = result) }
        }
    }

    /** Loads the next page when the focus nears the end of what's loaded. */
    fun nearEnd() {
        val api = container.api() ?: return
        val page = (state.value.library as? LoadState.Ready)?.value ?: return
        if (page.loadingMore || page.items.size >= page.total) return
        _state.update { it.copy(library = LoadState.Ready(page.copy(loadingMore = true))) }
        viewModelScope.launch {
            val more = try {
                api.items(libraryQuery(page.library, sort = "title", limit = PAGE, offset = page.items.size)).items
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                ArcLog.Android.warn("library ${page.library.id} page failed: ${e.javaClass.simpleName}")
                emptyList()
            }
            _state.update { s ->
                val cur = (s.library as? LoadState.Ready)?.value ?: return@update s
                if (cur.library.id != page.library.id) return@update s
                s.copy(library = LoadState.Ready(cur.copy(items = cur.items + more, loadingMore = false, total = if (more.isEmpty()) cur.items.size else cur.total)))
            }
        }
    }

    fun signOut(done: () -> Unit) {
        val api = container.api()
        container.appScope.launch {
            runCatching { api?.signOut() }
            container.session.token = null
            ArcLog.Android.info("signed out")
            kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.Main) { done() }
        }
    }

    private companion object {
        const val PAGE = 60
    }
}
