package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/auth"
)

func TestArrangeRecommendedFamilies(t *testing.T) {
	rows := []recommendedRow{{Type: "continue"}, {Type: "because_you_watched", Name: "A"}, {Type: "recently_added"}, {Type: "because_you_watched", Name: "B"}, {Type: "new_family"}}
	got := arrangeRecommended(rows, []recommendedLook{{Type: "because_you_watched", Style: "arc"}, {Type: "continue", Hidden: true}})
	if len(got) != 4 || got[0].Name != "A" || got[1].Name != "B" || got[0].Style != "arc" || got[2].Type != "recently_added" || got[3].Type != "new_family" {
		t.Fatalf("arranged: %+v", got)
	}
}

func TestRecommendedLayoutValidationAccessAndPersistence(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := t.Context()
	owner, err := s.store.CreateUser(ctx, "Owner", true, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for token, id := range map[string]string{"viewer": viewer.ID, "owner": owner.ID} {
		if err := s.store.CreateSession(ctx, auth.HashToken(token), id, "password", "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := s.store.SyncLibrary(ctx, "Layout", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	request := func(token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", fmt.Sprintf("/api/libraries/%d/recommended/layout", lib), strings.NewReader(body))
		r.SetPathValue("id", fmt.Sprint(lib))
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		w := httptest.NewRecorder()
		s.adminOnly(s.setRecommendedLayout)(w, r)
		return w
	}
	valid := `{"rows":[{"type":"recently_added","hidden":true,"style":"arc"}]}`
	if w := request("viewer", valid); w.Code != 403 {
		t.Fatalf("viewer: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{}`, `{"rows":null}`, `{"rows":[{"type":"unknown"}]}`, `{"rows":[{"type":"continue","style":"bad"}]}`, `{"rows":[{"type":"continue"},{"type":"continue"}]}`, `{"rows":[{"type":"continue","extra":true}]}`} {
		if w := request("owner", body); w.Code != 400 {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
	if w := request("owner", valid); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	raw, err := s.store.RecommendedLayout(ctx, lib)
	if err != nil || !strings.Contains(raw, `"hidden":true`) {
		t.Fatalf("persist: %s %v", raw, err)
	}
	if w := request("owner", `{"rows":[]}`); w.Code != 200 {
		t.Fatalf("reset: %d", w.Code)
	}
	raw, err = s.store.RecommendedLayout(ctx, lib)
	if err != nil || raw != "[]" {
		t.Fatalf("reset persisted: %s %v", raw, err)
	}
}
