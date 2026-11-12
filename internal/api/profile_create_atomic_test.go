package api

import (
	"ams/internal/auth"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// A rejected administrator create must preserve this invariant: any rejected create leaves no new profile.
func TestRejectedProfileCreateLeavesNoAccount(t *testing.T) {
	s, admin := securityServer(t)
	admin.IsAdmin = true
	before, err := s.store.CountUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v1/profiles", strings.NewReader(`{"name":"Rejected child","max_age":7,"library_ids":"bad","exclude_labels":["private"]}`))
	r = r.WithContext(context.WithValue(r.Context(), userKey, admin))
	w := httptest.NewRecorder()
	s.createProfile(w, r)
	if w.Code != 400 {
		t.Fatalf("status=%d body=%s; want 400", w.Code, w.Body.String())
	}
	users, err := s.store.ListUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Name == "Rejected child" {
			t.Logf("unexpected persisted profile: max_age=%d libraries_nil=%t excluded_labels=%v is_friend=%t", u.MaxAge, u.LibraryIDs == nil, u.ExcludeLabels, u.IsFriend)
		}
	}
	if len(users) != before {
		t.Fatalf("rejected create persisted profile: before=%d after=%d response=%s", before, len(users), w.Body.String())
	}
}

func TestProfileCreateKeepsAllNoneAndFriendGrantSemantics(t *testing.T) {
	s, admin := securityServer(t)
	admin.IsAdmin = true
	for _, tc := range []struct {
		name, extra        string
		all, friend, grant bool
	}{
		{"Default", "", true, false, false}, {"All", `,"library_ids":null`, true, false, false},
		{"None", `,"library_ids":[]`, false, false, false},
		{"Friend", `,"friend":true,"library_ids":[],"max_age":7,"exclude_labels":["private"],"allow_downloads":true`, false, true, true},
	} {
		r := httptest.NewRequest("POST", "/api/v1/profiles", strings.NewReader(`{"name":"`+tc.name+`"`+tc.extra+`}`))
		r = r.WithContext(context.WithValue(r.Context(), userKey, admin))
		w := httptest.NewRecorder()
		s.createProfile(w, r)
		if w.Code != 201 {
			t.Fatalf("%s got %d: %s", tc.name, w.Code, w.Body)
		}
		var dto profileDTO
		if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
			t.Fatal(err)
		}
		saved, err := s.store.GetUser(t.Context(), dto.ID)
		if err != nil {
			t.Fatal(err)
		}
		if (saved.LibraryIDs == nil) != tc.all || saved.IsFriend != tc.friend || dto.AllowDownloads != tc.grant {
			t.Fatalf("%s lost semantics: %+v", tc.name, dto)
		}
		if tc.friend {
			invited, err := s.store.UserByInvite(t.Context(), auth.HashToken(dto.InviteToken))
			if err != nil || invited == nil || invited.ID != dto.ID || saved.MaxAge != 7 {
				t.Fatal("invite/restrictions not committed together")
			}
		}
	}
}
