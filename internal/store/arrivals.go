package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type arrival struct{ at, sequence int64 }

func nextArrival(ctx context.Context, tx *sql.Tx) (arrival, error) {
	a := arrival{at: time.Now().Unix()}
	err := tx.QueryRowContext(ctx, `UPDATE arrival_clock SET sequence=sequence+1 WHERE id=1 RETURNING sequence`).Scan(&a.sequence)
	return a, err
}

// Only first availability changes added_at. Refreshing metadata, adding another
// version or returning from an offline drive must not turn old content into new.
func recordArrival(ctx context.Context, tx *sql.Tx, id string, a arrival) error {
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_arrivals(item_id,added_at,sequence) VALUES(?,?,?)`, id, a.at, a.sequence)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		if _, err = tx.ExecContext(ctx, `UPDATE items SET added_at=? WHERE id=?`, a.at, id); err != nil {
			return err
		}
	}
	var parent, kind string
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(parent_id,''),kind FROM items WHERE id=?`, id).Scan(&parent, &kind); err != nil {
		return err
	}
	if kind == "episode" && parent != "" {
		return recordArrival(ctx, tx, parent, a)
	}
	return nil
}

// Reidentifying/merging/splitting existing files preserves their earlier arrival.
func copyArrival(ctx context.Context, tx *sql.Tx, target, source string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO item_arrivals(item_id,added_at,sequence)
 SELECT ?1,added_at,sequence FROM item_arrivals WHERE item_id=?2
 ON CONFLICT(item_id) DO UPDATE SET added_at=excluded.added_at,sequence=excluded.sequence
 WHERE excluded.added_at<item_arrivals.added_at OR (excluded.added_at=item_arrivals.added_at AND excluded.sequence<item_arrivals.sequence)`, target, source)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE items SET added_at=(SELECT added_at FROM item_arrivals WHERE item_id=?1) WHERE id=?1 AND EXISTS(SELECT 1 FROM item_arrivals WHERE item_id=?1)`, target)
	return err
}

func linkedArrival(ctx context.Context, tx *sql.Tx, fileID int64) (arrival, error) {
	var a arrival
	err := tx.QueryRowContext(ctx, `SELECT a.added_at,a.sequence FROM item_arrivals a JOIN item_files f ON f.item_id=a.item_id WHERE f.file_id=? ORDER BY a.added_at,a.sequence LIMIT 1`, fileID).Scan(&a.at, &a.sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return nextArrival(ctx, tx)
	}
	return a, err
}

// eligibleArrivalEpisode is shared by both keys for a show's Added order.
func eligibleArrivalEpisode(access Access) (string, []any) {
	cond, args := access.Conds("c")
	visible := ""
	if len(cond) > 0 {
		visible = " AND " + strings.Join(cond, " AND ")
	}
	return `c.parent_id=items.id AND c.kind='episode' AND EXISTS(SELECT 1 FROM item_files f
		JOIN media_files m ON m.id=f.file_id WHERE f.item_id=c.id AND m.missing_since IS NULL)` + visible, args
}

// A show's Added order follows newly available visible episodes. Its own added_at
// remains the first arrival, for distinguishing a new show from a new season.
func recentDateSQL(access Access) (string, []any) {
	where, args := eligibleArrivalEpisode(access)
	return `CASE WHEN items.kind='series' THEN COALESCE((SELECT c.added_at FROM items c
		WHERE ` + where + ` ORDER BY c.added_at DESC LIMIT 1),items.added_at) ELSE items.added_at END`, args
}

func recentSequenceSQL(access Access) (string, []any) {
	where, args := eligibleArrivalEpisode(access)
	const arrived = `FROM items c JOIN item_arrivals a ON a.item_id=c.id WHERE `
	// Seek to the latest eligible date in items_parent_added. Only episodes on that date
	// can win the sequence tie-break, and an episode without an arrival row cannot win it.
	return `COALESCE(CASE WHEN items.kind='series' THEN (SELECT MAX(a.sequence) ` + arrived + where + `
		AND c.added_at=(SELECT c.added_at ` + arrived + where + ` ORDER BY c.added_at DESC LIMIT 1))
		ELSE (SELECT sequence FROM item_arrivals WHERE item_id=items.id) END,0)`,
		append(args, args...)
}
