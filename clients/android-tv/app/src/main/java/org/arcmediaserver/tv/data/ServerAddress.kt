package org.arcmediaserver.tv.data

import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** Arc's port unless the address says otherwise. */
const val DEFAULT_PORT = 8099

/**
 * Turns what someone typed with a remote into a server's base URL.
 *
 * "192.168.1.20" becomes http://192.168.1.20:8099; "arc.example.com" the same with its name;
 * an address with a scheme or port keeps them, so "https://arc.example.com" stays on 443.
 * Paths, queries and fragments are dropped: the API lives at the root. Returns null for something
 * that can't be an address.
 */
fun normalizeServerAddress(input: String): HttpUrl? {
    var text = input.trim()
    if (text.isEmpty() || text.any { it.isWhitespace() }) return null
    val hasScheme = text.startsWith("http://", ignoreCase = true) || text.startsWith("https://", ignoreCase = true)
    if (hasScheme && text.substringAfter("://").trimEnd('/').isEmpty()) return null
    text = text.trimEnd('/')
    if (!hasScheme) {
        if ("://" in text) return null
        text = "http://$text"
    }
    val parsed = text.toHttpUrlOrNull() ?: return null
    if (parsed.host.isEmpty()) return null
    val afterScheme = text.substringAfter("://").substringBefore('/').substringBefore('?').substringBefore('#')
    // "[::1]:8099" or "host:8099" name a port; a bare IPv6 address in brackets doesn't.
    val explicitPort = Regex(""":(\d+)$""").containsMatchIn(afterScheme)
    return parsed.newBuilder()
        .apply { if (!hasScheme && !explicitPort) port(DEFAULT_PORT) }
        .encodedPath("/")
        .query(null)
        .fragment(null)
        .username("")
        .password("")
        .build()
}

/** How an address is shown back to people: no trailing slash, no default ports. */
fun HttpUrl.display(): String = toString().trimEnd('/')
