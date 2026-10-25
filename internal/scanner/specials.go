package scanner

import "ams/internal/metadata"

// specialPlacements resolves aired hints once, before display orders may change.
// An absent/invalid target or missing cross-provider mapping never guesses a position.
func specialPlacements(o metadata.Ordering, stable func(int64) string) map[string]string {
	positions := map[[2]int]*metadata.Episode{}
	first := map[int]int{}
	for _, e := range o.Entries {
		if e.Season <= 0 || e.Number <= 0 || e.Episode == nil {
			continue
		}
		key := [2]int{e.Season, e.Number}
		if positions[key] == nil {
			positions[key] = e.Episode
		}
		if first[e.Season] == 0 || e.Number < first[e.Season] {
			first[e.Season] = e.Number
		}
	}
	before := map[string]string{}
	for _, e := range o.Entries {
		ep := e.Episode
		if e.Season != 0 || ep == nil || ep.AirsBeforeSeason == nil || *ep.AirsBeforeSeason <= 0 {
			continue
		}
		season, number := *ep.AirsBeforeSeason, 0
		if ep.AirsBeforeEpisode != nil {
			number = *ep.AirsBeforeEpisode
		}
		if number < 0 {
			continue
		}
		if number == 0 {
			number = first[season]
		}
		target := positions[[2]int{season, number}]
		if target != nil && stable(ep.ID) != "" && stable(target.ID) != "" && stable(ep.ID) != stable(target.ID) {
			before[stable(ep.ID)] = stable(target.ID)
		}
	}
	return before
}
