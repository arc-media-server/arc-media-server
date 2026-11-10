package store

import (
	"errors"
	"testing"
)

func TestMovieMergePreservesWatchedStateAndQueueIdentity(t *testing.T) {
	st, user, session, items := queueStoreFixture(t)
	keep, source := items[0], items[1]
	if err := st.SetUserRating(t.Context(), user, keep, 7); err != nil {
		t.Fatal(err)
	} // Target already has a row.
	if err := st.SetPlayed(t.Context(), user, []string{source}, true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetWatchlist(t.Context(), user, source, true); err != nil {
		t.Fatal(err)
	}
	queue, err := st.CreatePlayQueue(t.Context(), user, session, []string{source, keep}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MergeItems(t.Context(), keep, []string{source}); err != nil {
		t.Fatal(err)
	}
	data, err := st.UserDataFor(t.Context(), user, []string{keep})
	if err != nil {
		t.Fatal(err)
	}
	state := data[keep]
	if !state.Played || state.PlayCount < 1 || state.Rating != 7 || !state.Watchlisted {
		t.Errorf("merged state lost: %+v", state)
	}
	after, err := st.PlayQueue(t.Context(), user, session, queue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Entries) != 2 || after.CurrentEntryID != queue.CurrentEntryID {
		t.Errorf("queue membership/current entry lost: %+v", after)
	}
	for _, entry := range after.Entries {
		if entry.ItemID != keep {
			t.Errorf("queue entry still targets source: %+v", entry)
		}
	}
	if after.Revision <= queue.Revision {
		t.Error("queue identity changed without advancing revision")
	}
	if _, err := st.MutatePlayQueue(t.Context(), user, session, queue.ID, QueueMutation{Revision: queue.Revision, Operation: "shuffle"}); !errors.Is(err, ErrQueueConflict) {
		t.Errorf("stale pre-merge mutation not rejected: %v", err)
	}
}
