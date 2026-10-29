package org.arcmediaserver.tv.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ServerAddressTest {
    private fun n(s: String) = normalizeServerAddress(s)?.toString()

    @Test
    fun `a bare address gets http and Arc's port`() {
        assertEquals("http://192.168.1.20:8099/", n("192.168.1.20"))
        assertEquals("http://arc.local:8099/", n("  arc.local/ "))
        assertEquals("http://nas:8099/", n("NAS"))
    }

    @Test
    fun `a port typed is kept`() {
        assertEquals("http://192.168.1.20:32400/", n("192.168.1.20:32400"))
        assertEquals("http://[::1]:9000/", n("[::1]:9000"))
    }

    @Test
    fun `a bracketed IPv6 address without a port gets Arc's`() {
        assertEquals("http://[fe80::1]:8099/", n("[fe80::1]"))
    }

    @Test
    fun `a scheme keeps its own default port`() {
        assertEquals("https://arc.example.com/", n("https://arc.example.com"))
        assertEquals("http://arc.example.com/", n("HTTP://arc.example.com/"))
        assertEquals("https://arc.example.com:8443/", n("https://arc.example.com:8443"))
    }

    @Test
    fun `paths, queries and credentials are dropped`() {
        assertEquals("http://10.0.0.5:8099/", n("10.0.0.5/library/2?x=1#top"))
        assertEquals("https://arc.example.com/", n("https://user:pw@arc.example.com/web"))
    }

    @Test
    fun `nonsense is refused`() {
        assertNull(n(""))
        assertNull(n("   "))
        assertNull(n("two words"))
        assertNull(n("ftp://arc.example.com"))
        assertNull(n("http://"))
    }

    @Test
    fun `display hides the trailing slash`() {
        assertEquals("http://192.168.1.20:8099", normalizeServerAddress("192.168.1.20")!!.display())
    }
}
