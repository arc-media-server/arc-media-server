package api

import (
	"ams/internal/auth"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// This checks the missing bcrypt work, not exact network timing or constant-time name lookup.
// Use the fastest of three real default-cost comparisons and a generous 8x margin; slow fixture
// IO can hide the old defect but cannot make the fixed implementation skip its bcrypt work.
func TestFriendUnknownAndUnacceptedNamesPayPasswordCost(t *testing.T) {
	s, _ := securityServer(t)
	hash, err := auth.Hash("fixture password")
	if err != nil {
		t.Fatal(err)
	}
	friend, err := s.store.CreateUser(t.Context(), "Pending friend", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetInvite(t.Context(), friend.ID, "fixture-invite-hash", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	baseline := time.Duration(math.MaxInt64)
	for range 3 {
		start := time.Now()
		if auth.Check(hash, "wrong password") {
			t.Fatal("wrong password matched")
		}
		baseline = min(baseline, time.Since(start))
	}
	var response string
	for _, name := range []string{"Unknown friend", friend.Name} {
		start := time.Now()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/auth/friend", strings.NewReader(fmt.Sprintf(`{"name":%q,"password":"wrong password"}`, name)))
		s.friendLogin(w, r)
		elapsed := time.Since(start)
		if w.Code != 401 {
			t.Fatalf("%q: %d %s", name, w.Code, w.Body.String())
		}
		if elapsed < baseline/8 {
			t.Errorf("%q skipped password work: %s vs bcrypt baseline %s", name, elapsed, baseline)
		}
		if response != "" && response != w.Body.String() {
			t.Fatal("unknown and pending names have different responses")
		}
		response = w.Body.String()
	}
}

func TestDummyFriendPasswordCanNeverAuthorizeMissingOrPendingFriend(t *testing.T) {
	s, _ := securityServer(t)
	friend, err := s.store.CreateUser(t.Context(), "Pending friend", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetInvite(t.Context(), friend.ID, "fixture", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Unknown friend", friend.Name} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/auth/friend", strings.NewReader(fmt.Sprintf(`{"name":%q,"password":"Arc non-credential dummy comparison"}`, name)))
		s.friendLogin(w, r)
		if w.Code != 401 || len(w.Result().Cookies()) != 0 {
			t.Fatalf("dummy credential authorized %q: %d", name, w.Code)
		}
	}
}
