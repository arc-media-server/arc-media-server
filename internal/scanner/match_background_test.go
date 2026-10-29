package scanner

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/metadata"
	"ams/internal/store"
)

// matchSetup scans the fixture show's files with no providers (so it is unmatched, with
// file-only episodes), then points the scanner at the stand-in providers.
func matchSetup(t *testing.T, delay time.Duration) (*store.Store, *Scanner, int64, *store.Item, *matchFixture) {
	t.Helper()
	ctx := context.Background()
	st, s, lib, _ := libraryFixture(t, "shows", showFiles())
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 1 {
		t.Fatalf("shows=%v err=%v", shows, err)
	}
	f := newMatchFixture(t, delay)
	s.tmdb = metadata.NewTMDB("fixture-only")
	s.tvdb = metadata.NewTVDB("fixture-only")
	s.Activity = activity.New()
	it, err := st.GetItem(ctx, shows[0].ID)
	if err != nil || it == nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Wait)
	return st, s, lib, it, f
}

func findTask(tasks []activity.Task, id int64) *activity.Task {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

// checkEpisodes asserts the show has exactly the provider's episodes, each file linked to one.
func checkEpisodes(t *testing.T, st *store.Store, lib int64, show string) []store.Item {
	t.Helper()
	ctx := context.Background()
	kids, err := st.Children(ctx, show)
	if err != nil {
		t.Fatal(err)
	}
	if want := fixtureSeasons * fixtureEpisodes; len(kids) != want {
		t.Fatalf("episodes = %d, want %d", len(kids), want)
	}
	for _, k := range kids {
		if k.MetadataState != "matched" || !strings.HasPrefix(k.Title, "Episode ") {
			t.Fatalf("episode not filled from the provider: %+v", k)
		}
		files, err := st.ItemFiles(ctx, k.ID)
		if err != nil || len(files) != 1 {
			t.Fatalf("episode %s has files %v (%v)", k.Title, files, err)
		}
	}
	return kids
}

func TestSetMatchReturnsBeforeEpisodesAreFetched(t *testing.T) {
	ctx := context.Background()
	st, s, lib, it, f := matchSetup(t, 0)
	release := f.hold()
	defer release()

	done := make(chan struct{})
	var taskID int64
	var err error
	go func() { taskID, err = s.SetMatch(ctx, it, "tvdb", 404); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("SetMatch waited for the episode requests")
	}
	if err != nil || taskID == 0 {
		t.Fatalf("task=%d err=%v", taskID, err)
	}
	got, _ := st.GetItem(ctx, it.ID)
	if got.Title != "Right Show" || got.Year != 2020 || got.TVDBID != 404 || got.TMDBID != 303 ||
		got.MetadataState != "matched" || !strings.Contains(got.PosterPath, "right.jpg") {
		t.Fatalf("quick part didn't identify the show: %+v", got)
	}
	running, _ := s.Activity.Snapshot()
	task := findTask(running, taskID)
	if task == nil || task.Title != "Matching Right Show (2020)" {
		t.Fatalf("no Activity task: %+v", running)
	}
	if !s.Matching(it.ID) {
		t.Fatal("Matching = false while the background part runs")
	}

	release()
	s.Wait()
	_, recent := s.Activity.Snapshot()
	if task := findTask(recent, taskID); task == nil || task.Error != "" {
		t.Fatalf("background task didn't finish cleanly: %+v", task)
	}
	if s.Matching(it.ID) {
		t.Fatal("still matching after the task finished")
	}
	checkEpisodes(t, st, lib, it.ID)
	orders, err := st.ListOrderings(ctx, it.ID)
	if err != nil || len(orders) < 4 {
		t.Fatalf("orderings = %d (%v), want both providers' orders", len(orders), err)
	}
	info, err := s.MatchInfo(ctx, it.ID)
	if err != nil || info == nil || info.Steps[0].Kind != "chosen" || info.Steps[0].Picked != 404 {
		t.Fatalf("match info lost the choice: %+v %v", info, err)
	}
}

func TestSetMatchKeepsWatchHistory(t *testing.T) {
	ctx := context.Background()
	st, s, lib, it, _ := matchSetup(t, 0)
	before, err := st.GetItemByKey(ctx, lib, fileEpisodeKey(it.ID, false, 1, 1))
	if err != nil || before == nil {
		t.Fatalf("file-only episode missing: %v", err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, before.ID, 600000, 2400000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetMatch(ctx, it, "tmdb", 303); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	checkEpisodes(t, st, lib, it.ID)
	// The library numbers files by TMDB: S01E01 is TMDB episode 100101, adopted with its id.
	after, err := st.GetItemByKey(ctx, lib, episodeKey(it.ID, "tmdb", 100101))
	if err != nil || after == nil || after.ID != before.ID {
		t.Fatalf("episode re-keyed under a new id: before=%s after=%+v (%v)", before.ID, after, err)
	}
	data, err := st.UserDataFor(ctx, u.ID, []string{before.ID})
	if err != nil || data[before.ID].PositionMS != 600000 {
		t.Fatalf("watch history lost: %+v %v", data, err)
	}
}

func TestConcurrentMatchesDontCorrupt(t *testing.T) {
	ctx := context.Background()
	st, s, lib, it, f := matchSetup(t, 0)
	release := f.hold()
	first, err := s.SetMatch(ctx, it, "tmdb", 303)
	if err != nil {
		t.Fatal(err)
	}
	// While the first is still fetching: a second match, a refresh and a scan of the library.
	cur, _ := st.GetItem(ctx, it.ID)
	var wg sync.WaitGroup
	var second int64
	var secondErr, refreshErr error
	wg.Add(2)
	go func() { defer wg.Done(); second, secondErr = s.SetMatch(ctx, cur, "tvdb", 404) }()
	go func() {
		defer wg.Done()
		l, _ := st.GetLibrary(ctx, lib)
		refreshErr = s.RefreshSeries(ctx, l, it.ID, false, nil)
	}()
	s.Request(lib)
	time.Sleep(50 * time.Millisecond)
	release()
	wg.Wait()
	s.Wait()
	if secondErr != nil || second == 0 || second == first {
		t.Fatalf("second match: task=%d err=%v", second, secondErr)
	}
	if refreshErr != nil && !errors.Is(refreshErr, ErrLibraryBusy) {
		t.Fatalf("refresh during a match: %v", refreshErr)
	}
	got, _ := st.GetItem(ctx, it.ID)
	if got.TMDBID != 303 || got.TVDBID != 404 || got.Title != "Right Show" || got.MetadataState != "matched" {
		t.Fatalf("show after concurrent matches: %+v", got)
	}
	checkEpisodes(t, st, lib, it.ID)
	_, recent := s.Activity.Snapshot()
	if task := findTask(recent, second); task == nil || task.Error != "" {
		t.Fatalf("latest match didn't finish cleanly: %+v", task)
	}
	if s.Matching(it.ID) {
		t.Fatal("still matching")
	}
}

func TestBackgroundMatchFailureIsVisible(t *testing.T) {
	ctx := context.Background()
	st, s, _, it, f := matchSetup(t, 0)
	f.fail.Store(true)
	taskID, err := s.SetMatch(ctx, it, "tvdb", 404)
	if err != nil {
		t.Fatalf("quick part failed: %v", err)
	}
	s.Wait()
	_, recent := s.Activity.Snapshot()
	task := findTask(recent, taskID)
	if task == nil || task.Error == "" {
		t.Fatalf("failure not in Activity: %+v", task)
	}
	info, err := s.MatchInfo(ctx, it.ID)
	if err != nil || info == nil {
		t.Fatal(err)
	}
	last := info.Steps[len(info.Steps)-1]
	if !strings.Contains(last.Error, "Refresh metadata") {
		t.Fatalf("failure not in the match explanation: %+v", info.Steps)
	}
	// The chosen show is kept (it exists), and still needs a full fetch.
	got, _ := st.GetItem(ctx, it.ID)
	if got.TVDBID != 404 || got.Title != "Right Show" {
		t.Fatalf("match not kept: %+v", got)
	}
	if synced, _ := st.SyncedAt(ctx, it.ID); synced != 0 {
		t.Fatal("a failed match was marked as fully fetched")
	}
}

func TestSetMatchUnknownIDChangesNothing(t *testing.T) {
	ctx := context.Background()
	st, s, _, it, _ := matchSetup(t, 0)
	if _, err := s.SetMatch(ctx, it, "tvdb", 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	got, _ := st.GetItem(ctx, it.ID)
	if got.Title != it.Title || got.TVDBID != 0 || got.MetadataState != it.MetadataState {
		t.Fatalf("a failed match changed the show: %+v", got)
	}
	if running, recent := s.Activity.Snapshot(); len(running)+len(recent) != 0 {
		t.Fatalf("a failed match started work: %+v %+v", running, recent)
	}
}

func TestMovieMatchStaysQuickAndSynchronous(t *testing.T) {
	ctx := context.Background()
	st, s, lib, _ := libraryFixture(t, "movies", map[string]string{"Wrong (1999)/Wrong (1999).mkv": "movie"})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	movies, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(movies) != 1 {
		t.Fatalf("movies=%v err=%v", movies, err)
	}
	f := newMatchFixture(t, 200*time.Millisecond)
	s.tmdb = metadata.NewTMDB("fixture-only")
	start := time.Now()
	taskID, err := s.SetMatch(ctx, &movies[0], "tmdb", 603)
	took := time.Since(start)
	if err != nil || taskID != 0 {
		t.Fatalf("task=%d err=%v", taskID, err)
	}
	got, _ := st.GetItem(ctx, movies[0].ID)
	if got.Title != "The Matrix" || got.TMDBID != 603 {
		t.Fatalf("movie not matched before returning: %+v", got)
	}
	t.Logf("movie match took %v with %d provider requests at 200ms each", took, f.requests.Load())
}
