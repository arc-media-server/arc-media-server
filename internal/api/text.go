package api

import "unicode/utf8"

// utf8BytePrefix preserves the existing byte limit without splitting a UTF-8 character.
// Callers receive valid UTF-8 from JSON decoding; short strings are returned unchanged.
func utf8BytePrefix(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}
