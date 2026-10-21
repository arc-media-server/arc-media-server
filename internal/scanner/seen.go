package scanner

import (
	"ams/internal/store"
	"context"
)

type seenKey struct{}
type seenFiles struct {
	ids []int64
	st  *store.Store
	at  int64
}

func (s *seenFiles) flush(ctx context.Context) error {
	if err := s.st.TouchFiles(ctx, s.ids, s.at); err != nil {
		return err
	}
	s.ids = s.ids[:0]
	return nil
}
