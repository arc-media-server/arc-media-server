package store

import (
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"ams/internal/perftest"
)

func queueStoreFixture(t *testing.T) (*Store, string, string, []string) {
	t.Helper()
	st := openTest(t)
	ctx := t.Context()
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	session := "fixture-session"
	if err = st.CreateSession(ctx, session, u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, name := range []string{"A", "B", "C", "D"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: name}, name)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return st, u.ID, session, ids
}

func TestPlayQueueOperationsAndConflictBoundaries(t *testing.T) {
	st, user, session, ids := queueStoreFixture(t)
	ctx := t.Context()
	q, err := st.CreatePlayQueue(ctx, user, session, []string{ids[0], ids[1], ids[0], ids[2]}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if q.Entries[0].ID == q.Entries[2].ID || q.Entries[0].ItemID != q.Entries[2].ItemID {
		t.Fatal("duplicate entries lost identity")
	}
	old := q
	q, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "next", ItemIDs: []string{ids[3]}})
	if err != nil {
		t.Fatal(err)
	}
	if q.Entries[1].ItemID != ids[3] || q.Entries[0].ID != old.Entries[0].ID {
		t.Fatal("Play Next moved current")
	}
	if _, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: old.Revision, Operation: "clear"}); !errors.Is(err, ErrQueueConflict) {
		t.Fatalf("stale mutation %v", err)
	}
	original := slices.Clone(q.Entries)
	q, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "shuffle"})
	if err != nil {
		t.Fatal(err)
	}
	if q.CurrentEntryID != original[0].ID || q.Entries[0].ID != original[0].ID {
		t.Fatal("shuffle moved current")
	}
	q, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "unshuffle"})
	if err != nil {
		t.Fatal(err)
	}
	// Play Next while unshuffled should become part of the baseline before a shuffle cycle.
	if !slices.Equal(q.Entries, original) {
		t.Fatal("unshuffle did not restore the pre-shuffle queue")
	}
	if len(q.Entries) != 5 {
		t.Fatal("shuffle changed membership")
	}
	if _, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "remove", EntryIDs: []string{q.CurrentEntryID}}); !errors.Is(err, ErrQueueInput) {
		t.Fatal("removed current entry")
	}
	if _, err = st.CreatePlayQueue(ctx, user, session, ids, "", 0); !errors.Is(err, ErrQueueConflict) {
		t.Fatal("overwrote active queue without revision")
	}
	// Invalid FK input rolls back replacement, preserving the original queue.
	if _, err = st.CreatePlayQueue(ctx, user, session, []string{"does-not-exist"}, q.ID, q.Revision); err == nil {
		t.Fatal("accepted missing item")
	}
	if _, err = st.PlayQueue(ctx, user, session, q.ID); err != nil {
		t.Fatal("failed replacement lost queue", err)
	}
	q, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "advance"})
	if err != nil {
		t.Fatal(err)
	}
	q, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "clear"})
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Entries) != 2 {
		t.Fatal("clear removed history/current or left upcoming", len(q.Entries))
	}
}

func TestPlayQueueConcurrentEditsPersistenceAndRevocation(t *testing.T) {
	st, user, session, ids := queueStoreFixture(t)
	ctx := t.Context()
	q, err := st.CreatePlayQueue(ctx, user, session, ids, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "append", ItemIDs: ids[:1]})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	pass, conflict := 0, 0
	for err := range results {
		if err == nil {
			pass++
		} else if errors.Is(err, ErrQueueConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if pass != 1 || conflict != 1 {
		t.Fatal("lost conflict protection", pass, conflict)
	}
	backup := filepath.Join(t.TempDir(), "queue.db")
	if err = st.BackupTo(ctx, backup); err != nil {
		t.Fatal(err)
	}
	var restored *Store
	if dsn, ok := perftest.PostgresDatabase(t); ok {
		if err := RestorePostgresBackup(ctx, dsn, backup); err != nil {
			t.Fatal(err)
		}
		restored, err = OpenPostgres(ctx, dsn, "")
	} else {
		restored, err = Open(ctx, backup, Options{})
	}
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got, err := restored.PlayQueue(ctx, user, session, q.ID)
	if err != nil || got.Revision != 2 || len(got.Entries) != 5 {
		t.Fatalf("persistent queue %+v %v", got, err)
	}
	if _, err = st.PlayQueue(ctx, user, "other-device", q.ID); !errors.Is(err, ErrQueueNotFound) {
		t.Fatal("other device accessed queue")
	}
	if _, err = st.PlayQueue(ctx, "other-profile", session, q.ID); !errors.Is(err, ErrQueueNotFound) {
		t.Fatal("other profile accessed queue")
	}
	if err = st.DeleteSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err = st.PlayQueue(ctx, user, session, q.ID); !errors.Is(err, ErrQueueNotFound) {
		t.Fatal("revoked session retained queue")
	}
}

func TestPlayQueueExpiryLimitsAndExactReorder(t *testing.T) {
	st, user, session, ids := queueStoreFixture(t)
	ctx := t.Context()
	if _, err := st.CreatePlayQueue(ctx, user, session, make([]string, MaxQueueEntries+1), "", 0); !errors.Is(err, ErrQueueInput) {
		t.Fatal("queue cap ignored")
	}
	q, err := st.CreatePlayQueue(ctx, user, session, ids, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	reordered := []string{q.Entries[0].ID, q.Entries[3].ID, q.Entries[2].ID, q.Entries[1].ID}
	q, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "reorder", EntryIDs: reordered})
	if err != nil {
		t.Fatal(err)
	}
	if q.Entries[1].ItemID != ids[3] {
		t.Fatal("reorder not applied")
	}
	bad := slices.Clone(reordered)
	bad[0], bad[1] = bad[1], bad[0]
	if _, err = st.MutatePlayQueue(ctx, user, session, q.ID, QueueMutation{Revision: q.Revision, Operation: "reorder", EntryIDs: bad}); !errors.Is(err, ErrQueueInput) {
		t.Fatal("reorder moved current")
	}
	if _, err = st.db.Exec("UPDATE play_queues SET expires_at=1 WHERE id=?", q.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = st.PlayQueue(ctx, user, session, q.ID); !errors.Is(err, ErrQueueNotFound) {
		t.Fatal("expired queue still visible")
	}
	if _, err = st.CreatePlayQueue(ctx, user, session, ids, "", 0); err != nil {
		t.Fatal("expired queue prevented creation", err)
	}
}

func TestPlayQueueReferencesSurviveEpisodeCleanupAndUnmatch(t *testing.T) {
	st, user, session, _ := queueStoreFixture(t)
	ctx := t.Context()
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	show, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Known"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	ep, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: show, Title: "Saved"}, "episode")
	if err != nil {
		t.Fatal(err)
	}
	playlistOnly, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: show, Title: "Playlist only"}, "playlist-only")
	if err != nil {
		t.Fatal(err)
	}
	playlist, err := st.CreatePlaylist(ctx, user, "Retained")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.AddToPlaylist(ctx, playlist.ID, []string{playlistOnly}); err != nil {
		t.Fatal(err)
	}
	q, err := st.CreatePlayQueue(ctx, user, session, []string{ep}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.DeleteOrphanEpisodes(ctx, show); err != nil {
		t.Fatal(err)
	}
	kept, e := st.GetItem(ctx, playlistOnly)
	if e != nil || kept == nil {
		t.Fatal("cleanup deleted a playlist-only episode", e)
	}
	if err = st.Unmatch(ctx, show, "Filename", 0, nil); err != nil {
		t.Fatal(err)
	}
	got, err := st.PlayQueue(ctx, user, session, q.ID)
	if err != nil || len(got.Entries) != 1 || got.Entries[0].ItemID != ep {
		t.Fatal("cleanup lost queue reference", err)
	}
}
