package store

import (
	"context"
	"testing"
	"time"
)

func TestMarkers(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "TV", "shows", "tvdb_aired", []string{"/t"})
	series, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Firefly", TVDBID: 78874, IMDBID: "tt0303461"}, "s")
	ep, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: series, Title: "The Train Job"}, "ep:s:tvdb:1")
	fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/t/Firefly/S01E01.mkv", Size: 1, ModTime: 1, DurationMS: 2_640_000}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	st.SetFileItems(ctx, fid, []string{ep}, 0)
	ord, _ := st.UpsertOrdering(ctx, series, "tvdb_aired", "", "Aired Order", "TVDB")
	st.ReplaceOrdering(ctx, ord, map[int]string{1: "Season 1"}, nil, []OrderingEntry{{EpisodeID: ep, Season: 1, Episode: 1}})

	jobs, err := st.MarkerJobs(ctx, time.Now().Unix(), 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs = %+v, %v", jobs, err)
	}
	if j := jobs[0]; j.SeriesIMDB != "tt0303461" || j.Season != 1 || j.Episode != 1 || j.DurationMS != 2_640_000 {
		t.Errorf("job = %+v", j)
	}
	if files, err := st.SeasonFiles(ctx, series, 1); err != nil || len(files) != 1 {
		t.Errorf("season files = %+v, %v", files, err)
	}

	// Found markers fill in; the user's replace them and aren't replaced back.
	st.SaveMarkers(ctx, fid, []Marker{{Kind: "intro", StartMS: 60_000, EndMS: 110_000, Source: "introdb"}})
	st.SetManualMarkers(ctx, fid, []Marker{{Kind: "intro", StartMS: 61_000, EndMS: 112_000}})
	st.SaveMarkers(ctx, fid, []Marker{{Kind: "intro", StartMS: 1, EndMS: 2, Source: "theintrodb"}})
	ms, _ := st.FileMarkers(ctx, fid)
	var intro Marker
	for _, m := range ms {
		if m.Kind == "intro" {
			intro = m
		}
	}
	if intro.Source != "manual" || intro.StartMS != 61_000 {
		t.Errorf("intro = %+v", intro)
	}
	st.MarkersChecked(ctx, fid)
	if jobs, _ := st.MarkerJobs(ctx, time.Now().Add(-time.Hour).Unix(), 10); len(jobs) != 0 {
		t.Errorf("checked file still listed: %+v", jobs)
	}
}
