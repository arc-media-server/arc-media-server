package org.arcmediaserver.tv.ui.details

import org.arcmediaserver.tv.data.Item
import org.arcmediaserver.tv.data.Season
import org.arcmediaserver.tv.data.UserData
import org.junit.Assert.assertEquals
import org.junit.Test

class NextEpisodeTest {
    private fun ep(id: String, played: Boolean = false, pos: Long = 0, available: Boolean = true) =
        Item(id = id, kind = "episode", title = id, userData = UserData(pos, played), available = available)

    @Test
    fun `carries on with the episode part-way through`() {
        val seasons = listOf(Season(1, episodes = listOf(ep("a", played = true), ep("b"), ep("c", pos = 120_000))))
        assertEquals("c", nextEpisode(seasons)?.id)
    }

    @Test
    fun `else the first not watched, specials last and missing files skipped`() {
        val seasons = listOf(
            Season(0, episodes = listOf(ep("special"))),
            Season(1, episodes = listOf(ep("a", played = true), ep("b", available = false), ep("c"))),
        )
        assertEquals("c", nextEpisode(seasons)?.id)
    }

    @Test
    fun `everything watched starts from the beginning`() {
        val seasons = listOf(Season(1, episodes = listOf(ep("a", played = true), ep("b", played = true))))
        assertEquals("a", nextEpisode(seasons)?.id)
    }
}
