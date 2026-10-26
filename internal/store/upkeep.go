package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"ams/internal/diskspace"
)

var ErrIntegrity = errors.New("database integrity check failed")

type DatabaseUpkeep struct {
	BeforeBytes    int64  `json:"before_bytes"`
	AfterBytes     int64  `json:"after_bytes"`
	Vacuumed       bool   `json:"vacuumed"`
	VacuumSkipped  string `json:"vacuum_skipped,omitempty"`
	CheckpointBusy bool   `json:"checkpoint_busy"`
}

// OptimizeDatabase joins the existing writer queue; it never replaces the open database.
// WAL readers remain independent. VACUUM needs a dedicated connection, outside a transaction.
func (s *Store) OptimizeDatabase(ctx context.Context, allowed func() bool) (DatabaseUpkeep, error) {
	return s.optimizeDatabase(ctx, allowed, diskspace.Available, 16<<20)
}

func (s *Store) optimizeDatabase(ctx context.Context, allowed func() bool, available func(string) (uint64, error), minimum int64) (out DatabaseUpkeep, err error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return out, err
	}
	defer conn.Close()
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if allowed != nil && !allowed() {
			return context.Canceled
		}
		return nil
	}
	if err = check(); err != nil {
		return out, err
	}
	if err = checkIntegrity(ctx, conn); err != nil {
		return out, err
	}
	var seq int
	var name, path string
	if err = conn.QueryRowContext(ctx, "PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		return out, err
	}
	before, err := databaseBytes(path)
	if err != nil {
		return out, err
	}
	out.BeforeBytes, out.AfterBytes = before, before
	// Modern SQLite bounds analysis and checks all tables, including this fresh connection.
	if _, err = conn.ExecContext(ctx, "PRAGMA optimize=0x10002"); err != nil {
		return out, err
	}
	var pages, free, size int64
	for query, dest := range map[string]*int64{"PRAGMA page_count": &pages, "PRAGMA freelist_count": &free, "PRAGMA page_size": &size} {
		if err = conn.QueryRowContext(ctx, query).Scan(dest); err != nil {
			return out, err
		}
	}
	out.VacuumSkipped = "Compaction not needed"
	if free*size >= minimum && free*5 >= pages {
		space, spaceErr := available(filepath.Dir(path))
		if spaceErr == nil {
			temporary, tempErr := available(os.TempDir())
			if tempErr != nil {
				spaceErr = tempErr
			} else {
				space = min(space, temporary)
			}
		}
		// Allow two whole database copies plus headroom for SQLite's temporary/WAL files.
		required := uint64(max(out.BeforeBytes, pages*size))*2 + (32 << 20)
		if spaceErr != nil {
			out.VacuumSkipped = "Could not verify free disk space; compaction skipped: " + spaceErr.Error()
		} else if space < required {
			out.VacuumSkipped = fmt.Sprintf("Not enough free disk space for compaction (%.1f MiB available; %.1f MiB required)", float64(space)/(1<<20), float64(required)/(1<<20))
		} else {
			if err = check(); err != nil {
				return out, err
			}
			if _, err = conn.ExecContext(ctx, "VACUUM"); err != nil {
				return out, err
			}
			out.Vacuumed, out.VacuumSkipped = true, ""
			var busy, log, checkpointed int
			if err = conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &checkpointed); err != nil {
				return out, err
			}
			out.CheckpointBusy = busy != 0
		}
	}
	if err = checkIntegrity(ctx, conn); err != nil {
		return out, err
	}
	if after, err := databaseBytes(path); err != nil {
		return out, err
	} else {
		out.AfterBytes = after
	}
	return out, nil
}

func checkIntegrity(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrIntegrity, err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("%w: %w", ErrIntegrity, err)
		}
		if result != "ok" {
			return fmt.Errorf("%w: %s", ErrIntegrity, result)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrIntegrity, err)
	}
	if count != 1 {
		return ErrIntegrity
	}
	return nil
}

// MaintenanceRun is independent of per-file background revisions and survives restarts.
type MaintenanceRun struct {
	StartedAt     int64  `json:"started_at"`
	FinishedAt    int64  `json:"finished_at"`
	LastSuccessAt int64  `json:"last_success_at"`
	Detail        string `json:"detail"`
	Error         string `json:"error,omitempty"`
}

func (s *Store) MaintenanceRun(ctx context.Context, kind string) (MaintenanceRun, error) {
	v, err := s.Setting(ctx, "maintenance_run_"+kind)
	var out MaintenanceRun
	if err == nil && v != "" {
		err = json.Unmarshal([]byte(v), &out)
	}
	return out, err
}

func (s *Store) SaveMaintenanceRun(ctx context.Context, kind string, run MaintenanceRun) error {
	b, err := json.Marshal(run)
	if err != nil {
		return err
	}
	return s.SetSetting(ctx, "maintenance_run_"+kind, string(b))
}

// Cached files belonging to missing/offline media remain referenced until records are removed.
func (s *Store) HasMediaFile(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_files WHERE id = ?", id).Scan(&n)
	return n != 0, err
}

// Include uncheckpointed WAL bytes: a live WAL database can be much larger than its main file.
func databaseBytes(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	total := info.Size()
	if wal, err := os.Stat(path + "-wal"); err == nil {
		total += wal.Size()
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	return total, nil
}
