package api

import (
	"context"
	"sync"
	"time"

	"ams/internal/search"
	"ams/internal/store"
)

// peopleIndex keeps everyone in the cast and crew in memory with their names already
// normalized, so search doesn't reload and re-normalize tens of thousands of names on every
// keystroke. It reloads when people are added or removed, and every few minutes for renames.
type peopleIndex struct {
	mu      sync.Mutex
	version string
	loaded  time.Time
	people  []store.Person
	names   []string
}

const peopleIndexMaxAge = 5 * time.Minute

func (x *peopleIndex) get(ctx context.Context, st *store.Store) ([]store.Person, []string, error) {
	version, err := st.PeopleVersion(ctx)
	if err != nil {
		return nil, nil, err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.people != nil && version == x.version && time.Since(x.loaded) < peopleIndexMaxAge {
		return x.people, x.names, nil
	}
	people, err := st.PeopleNames(ctx)
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, len(people))
	for i, p := range people {
		names[i] = search.Normalize(p.Name)
	}
	x.version, x.loaded, x.people, x.names = version, time.Now(), people, names
	return people, names, nil
}
