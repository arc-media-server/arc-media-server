# Preserve names in TMDB episode groups

TMDB episode groups display their own labels, such as Cour 1, Cour 5 and OVA. The library
and show preferences still apply to standard orders. Manually edited labels take priority.
A group with no name uses its season number, and Season 0 still defaults to Specials.

This affects labels only. The existing API, episode positions, file links and watch
history use the same records. There are no new logs or provider requests.
