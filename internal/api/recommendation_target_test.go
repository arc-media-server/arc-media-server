package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"ams/internal/perftest"
)

func TestTargetedRecommendationsKeepFullViewIdentityAndContent(t *testing.T) {
	_, st := libraryPerformanceServer(t, 40, 4, 8)
	user, err := st.GetUser(t.Context(), perftest.UserID)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{store: st}
	request := httptest.NewRequest("GET", "/api/recommended", nil)
	request = request.WithContext(context.WithValue(request.Context(), userKey, user))
	for _, libraryID := range []int64{1, 2} {
		lib, err := st.GetLibrary(t.Context(), libraryID)
		if err != nil {
			t.Fatal(err)
		}
		full, err := s.recommendedRowsLimit(request, lib, recommendedLimit)
		if err != nil {
			t.Fatal(err)
		}
		if len(full) < 4 {
			t.Fatalf("fixture coverage insufficient: library=%d rows=%d", libraryID, len(full))
		}
		for _, row := range full {
			filtered, err := s.recommendedRowsLimitTarget(request, lib, recommendedLimit, row.ID)
			if err != nil || len(filtered) != 1 {
				t.Fatalf("library=%d row=%s filtered=%d err=%v", libraryID, row.Type, len(filtered), err)
			}
			want, _ := json.Marshal(row)
			got, _ := json.Marshal(filtered[0])
			if !bytes.Equal(got, want) {
				t.Errorf("targeted %s changed public content:\ngot %s\nwant %s", row.Type, got, want)
			}
		}
		absent, err := s.recommendedRowsLimitTarget(request, lib, recommendedLimit, "not-a-current-row")
		if err != nil || len(absent) != 0 {
			t.Fatalf("stale row accepted: %+v %v", absent, err)
		}
	}
}
