package scanner

import (
	"context"
	"fmt"
	"os"

	"ams/internal/probe"
	"ams/internal/store"
)

// EnrichFormatFile updates only source format evidence for a saved file revision.
func (s *Scanner) EnrichFormatFile(ctx context.Context, f store.MediaFile) error {
	// The directory walk may precede this phase by minutes. Never enrich a changed/offline file.
	info, err := os.Stat(f.Path)
	if err != nil || info.Size() != f.Size || info.ModTime().Unix() != f.ModTime {
		return fmt.Errorf("media file unavailable or changed")
	}
	streams := make([]probe.Stream, len(f.Streams))
	for j, st := range f.Streams {
		// Start from what's saved, so a file brought up to date for a later addition (such as
		// aspect ratios) keeps HDR details an earlier sample found even if this sample fails.
		streams[j] = probe.Stream{Index: st.Index, Kind: st.Kind, Codec: st.Codec, ColorTransfer: st.ColorTransfer, Profile: st.Profile, ChannelLayout: st.ChannelLayout,
			DolbyVision: st.DolbyVision, DolbyVisionProfile: st.DolbyVisionProfile, DVBLCompatibilityID: st.DVBLCompatibilityID, HDR10Plus: st.HDR10Plus, HDRProbeStatus: st.HDRProbeStatus, AspectRatio: st.AspectRatio}
	}
	if err := s.prober.EnrichFormats(ctx, f.Path, streams); err != nil {
		return err
	}
	for j, st := range streams {
		f.Streams[j].Profile = st.Profile
		f.Streams[j].ChannelLayout = st.ChannelLayout
		f.Streams[j].DolbyVision = st.DolbyVision
		f.Streams[j].DolbyVisionProfile = st.DolbyVisionProfile
		f.Streams[j].DVBLCompatibilityID = st.DVBLCompatibilityID
		f.Streams[j].HDR10Plus = st.HDR10Plus
		f.Streams[j].HDRProbeStatus = st.HDRProbeStatus
		f.Streams[j].AspectRatio = st.AspectRatio
	}
	after, err := os.Stat(f.Path)
	if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return fmt.Errorf("media file unavailable or changed")
	}
	if err := s.store.SaveFormats(ctx, f); err != nil {
		return err
	}
	return nil
}
