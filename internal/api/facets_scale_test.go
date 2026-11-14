package api

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

// TestLibraryFacetScaleMeasurements measures full HTTP responses, without opening media.
func TestLibraryFacetScaleMeasurements(t *testing.T) {
	if os.Getenv("ARC_RUN_FACETS_SCALE") != "1" {
		t.Skip("opt-in 100k-episode facet measurements")
	}
	h, _ := libraryPerformanceServer(t, 10000, 1000, 100)
	for _, lib := range []int{2, 1} {
		var samples []float64
		for run := 0; run < 4; run++ {
			start := time.Now()
			w := fixtureAPICall(t.Context(), h, "GET", fmt.Sprintf("/libraries/%d/facets", lib), "")
			ms := float64(time.Since(start).Microseconds()) / 1000
			if w.Code != 200 {
				t.Fatalf("HTTP%d: %s", w.Code, w.Body.String())
			}
			if run > 0 {
				samples = append(samples, ms)
			}
			t.Logf("library=%d run=%d duration_ms=%.3f bytes=%d sha256=%x", lib, run, ms, w.Body.Len(), sha256.Sum256(w.Body.Bytes()))
		}
		sort.Float64s(samples)
		t.Logf("library=%d warm_median_ms=%.3f", lib, samples[1])
	}
}
