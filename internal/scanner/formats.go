package scanner

import (
	"context"
	"os"

	"ams/internal/probe"
	"ams/internal/store"
)

func (s *Scanner) enrichPendingFormats(ctx context.Context, lib *store.Library, scanStart int64) error {
	files, err := s.store.PendingFormats(ctx, lib.ID, scanStart)
	if err != nil || len(files) == 0 {
		return err
	}
	s.phase(lib.ID, "Updating format labels", len(files))
	return each(ctx, len(files), probeWorkers, func(i int) error {
		f := files[i]
		// The directory walk may precede this phase by minutes. Never enrich a changed/offline file.
		info, err := os.Stat(f.Path)
		if err != nil || info.Size() != f.Size || info.ModTime().Unix() != f.ModTime {
			return nil
		}
		streams := make([]probe.Stream, len(f.Streams))
		for j, st := range f.Streams {
			streams[j] = probe.Stream{Index: st.Index, Kind: st.Kind, Codec: st.Codec, ColorTransfer: st.ColorTransfer, Profile: st.Profile, ChannelLayout: st.ChannelLayout}
		}
		if err := s.prober.EnrichFormats(ctx, f.Path, streams); err != nil {
			return err
		}
		for j, st := range streams {
			f.Streams[j].Profile = st.Profile
			f.Streams[j].ChannelLayout = st.ChannelLayout
			f.Streams[j].DolbyVision = st.DolbyVision
			f.Streams[j].DolbyVisionProfile = st.DolbyVisionProfile
			f.Streams[j].HDR10Plus = st.HDR10Plus
			f.Streams[j].HDRProbeStatus = st.HDRProbeStatus
		}
		after, err := os.Stat(f.Path)
		if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return nil
		}
		if err := s.store.SaveFormats(ctx, f); err != nil {
			return err
		}
		s.update(lib.ID, func(st *Status) { st.FilesProbed++; st.Done++ })
		return nil
	})
}
