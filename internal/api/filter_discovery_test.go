package api

import (
	"ams/internal/store"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestExtendedFilterValidationAndApplication(t *testing.T) {
	for _, f := range []rowFilter{{AudioChannels: 33}, {DynamicRange: "HDR"}, {WatchState: "bad"}, {WatchState: "played", Unwatched: true}, {VideoCodec: "x' OR 1=1"}, {PersonRole: "actor"}} {
		if f.validateExtended() == nil {
			t.Fatal("accepted", f)
		}
	}
	f := rowFilter{VideoCodec: "hevc", AudioCodec: "truehd", AudioChannels: 8, DynamicRange: "dolby_vision", WatchState: "played", PersonID: 42, PersonRole: "actor"}
	if err := f.validateExtended(); err != nil {
		t.Fatal(err)
	}
	var q store.ItemQuery
	f.applyExtended(&q)
	if !q.Played || q.VideoCodec != "hevc" || q.AudioChannels != 8 || q.PersonID != 42 {
		t.Fatal(q)
	}
	p := prefs{Home: []homeRow{{ID: "custom:formats", Title: "Formats", Filter: &f}}}
	p.Home[0].Filter.LibraryID = 1
	if err := p.validate(); err != nil {
		t.Fatal(err)
	}
}
func TestFilterDiscoveryIsTypedAndLibraryScoped(t *testing.T) {
	s, _ := securityServer(t)
	lib, _ := s.store.SyncLibrary(t.Context(), "Filters", "movies", "tmdb_aired", nil)
	r := httptest.NewRequest("GET", "/api/libraries/1/filters", nil)
	r.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.filterDiscovery(w, r)
	var got filterDiscoveryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 || len(got.Filters) < 15 || lib != 1 {
		t.Fatal(w.Body.String(), err)
	}
	r.SetPathValue("id", "999")
	w = httptest.NewRecorder()
	s.filterDiscovery(w, r)
	if w.Code != 404 {
		t.Fatal("missing lib accepted")
	}
}
