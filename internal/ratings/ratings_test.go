package ratings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOMDbScores(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("i") {
		case "tt0111161":
			w.Write([]byte(`{"Response":"True","Ratings":[{"Source":"Internet Movie Database","Value":"9.3/10"},
				{"Source":"Rotten Tomatoes","Value":"89%"},{"Source":"Metacritic","Value":"82/100"}]}`))
		case "tt0000001":
			w.Write([]byte(`{"Response":"True","Ratings":[]}`))
		case "limit":
			w.Write([]byte(`{"Response":"False","Error":"Request limit reached!"}`))
		default:
			w.Write([]byte(`{"Response":"False","Error":"Invalid API key!"}`))
		}
	}))
	defer srv.Close()
	t.Setenv("AMS_OMDB_URL", srv.URL)
	ctx := context.Background()

	rt, mc, err := omdbScores(ctx, srv.Client(), "key", "tt0111161")
	if err != nil || rt == nil || *rt != 89 || mc == nil || *mc != 82 {
		t.Errorf("scores = %v, %v, %v", rt, mc, err)
	}
	if rt, mc, err := omdbScores(ctx, srv.Client(), "key", "tt0000001"); err != nil || rt != nil || mc != nil {
		t.Errorf("no scores = %v, %v, %v", rt, mc, err)
	}
	if _, _, err := omdbScores(ctx, srv.Client(), "key", "limit"); !errors.Is(err, errOMDbLimit) {
		t.Errorf("limit: %v", err)
	}
	if _, _, err := omdbScores(ctx, srv.Client(), "bad", "other"); !errors.Is(err, errOMDbKey) {
		t.Errorf("bad key: %v", err)
	}
}
