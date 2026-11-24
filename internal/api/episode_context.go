package api

import (
	"ams/internal/store"
	"context"
)

type episodeContext struct {
	series    *store.Item
	positions map[string][2]int
	entries   []store.OrderingEntry
	source    string
}

// episodeContexts is request-local: reuse one parent/order lookup per series without
// retaining another profile's display order or hiding database failures as missing metadata.
// A missing parent remains a valid context; each caller decides whether to show or group it.
type episodeContexts struct {
	server *Server
	ctx    context.Context
	cached map[string]episodeContext
}

func (s *Server) episodeContexts(ctx context.Context) *episodeContexts {
	return &episodeContexts{server: s, ctx: ctx, cached: map[string]episodeContext{}}
}

func (c *episodeContexts) get(parent string) (episodeContext, error) {
	if found, ok := c.cached[parent]; ok {
		return found, nil
	}
	series, err := c.server.store.GetItem(c.ctx, parent)
	if err != nil {
		return episodeContext{}, err
	}
	found := episodeContext{series: series}
	if series != nil {
		found.positions, found.entries, err = c.server.orderPositions(c.ctx, series)
		if err != nil {
			return episodeContext{}, err
		}
	}
	c.cached[parent] = found
	return found, nil
}

// preload shares series and display-order reads across the episodes on Home.
func (c *episodeContexts) preload(ids []string) error {
	var missing []string
	seen := map[string]bool{}
	for _, id := range ids {
		if _, ok := c.cached[id]; !ok && !seen[id] {
			missing = append(missing, id)
			seen[id] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}
	series, err := c.server.store.ItemsByID(c.ctx, missing)
	if err != nil {
		return err
	}
	orders, err := c.server.store.SeriesDisplayOrders(c.ctx, missing)
	if err != nil {
		return err
	}
	for _, id := range missing {
		found := episodeContext{}
		if item, ok := series[id]; ok {
			order := orders[id]
			found = episodeContext{series: &item, positions: entryPositions(order.Entries), entries: order.Entries, source: order.Source}
		}
		c.cached[id] = found
	}
	return nil
}
