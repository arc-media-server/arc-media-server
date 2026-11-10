package api

// normalizePlaybackPlan gives every successful playback/preview branch the same array
// contract, including early needs-analysis/no-video decisions. This applies only
// to playback lists; a profile's null library selection has different semantics.
func normalizePlaybackPlan(p playbackPlan) playbackPlan {
	out := p
	out.Reasons = playbackArray(out.Reasons)
	out.ReasonCodes = playbackArray(out.ReasonCodes)
	out.AudioTracks = playbackArray(out.AudioTracks)
	out.SubtitleTracks = playbackArray(out.SubtitleTracks)
	out.Versions = playbackArray(out.Versions)
	out.Chapters = playbackArray(out.Chapters)
	out.Markers = playbackArray(out.Markers)
	return out
}

func playbackArray[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}
