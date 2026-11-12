package store

import (
	"errors"
	"sync"
	"testing"
)

func TestCreateProfilePublishesRestrictionsAndInviteTogether(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	_, err := s.db.ExecContext(ctx, `CREATE TRIGGER require_complete_profile BEFORE INSERT ON users
 WHEN NEW.name='Restricted friend' AND (NEW.max_age<>7 OR NEW.library_ids<>'0' OR NEW.exclude_labels<>'["private"]' OR NEW.is_friend<>1 OR NEW.invite_hash<>'fixture-hash' OR NEW.invite_expires<>123 OR NEW.allow_downloads<>1)
 BEGIN SELECT RAISE(ABORT,'partial profile observed'); END`)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.CreateProfile(ctx, User{Name: "Restricted friend", Color: "blue", MaxAge: 7, LibraryIDs: []int64{}, ExcludeLabels: []string{"private"}, IsFriend: true, AllowDownloads: true}, "fixture-hash", 123)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.MaxAge != 7 || saved.LibraryIDs == nil || len(saved.LibraryIDs) != 0 || !saved.IsFriend || !saved.AllowDownloads || len(saved.ExcludeLabels) != 1 {
		t.Fatalf("incomplete saved profile: %+v", saved)
	}
}
func TestCreateProfileFailurePublishesNothing(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER refuse_profile BEFORE INSERT ON users WHEN NEW.name='Refused' BEGIN SELECT RAISE(ABORT,'fixture write refused'); END`); err != nil {
		t.Fatal(err)
	}
	u, err := s.CreateProfile(ctx, User{Name: "Refused", IsFriend: true, MaxAge: 7}, "fixture-invite", 100)
	if err == nil || u != nil {
		t.Fatal("failed insert published a profile")
	}
	count, err := s.CountUsers(ctx)
	if err != nil || count != 0 {
		t.Fatalf("partial row retained: %d %v", count, err)
	}
}
func TestConcurrentProfileNameCreatesOnlyOneCompleteAccount(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreateProfile(ctx, User{Name: "Same", MaxAge: 7, LibraryIDs: []int64{}}, "", 0)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrNameTaken) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflict=%d", successes, conflicts)
	}
}
