// Package perftest creates synthetic catalog rows for opt-in performance investigations.
// It is imported by tests/tools only. Paths and metadata are fictional; no media is opened.
package perftest

import (
	"context"
	"database/sql"
	"fmt"
)

const UserID = "perf-owner"
const sequence = `WITH RECURSIVE numbers(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM numbers WHERE n<?) `

// Seed requires a fresh, migrated disposable store and an explicit transaction. It never opens a
// database path or chooses a server itself. Sizes are bounded to keep accidental fixtures manageable.
func Seed(ctx context.Context, tx *sql.Tx, movies, shows, episodes int) error {
	if movies < 1 || shows < 1 || episodes < 1 || movies > 100000 || shows > 500000/episodes {
		return fmt.Errorf("fixture size out of range")
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO libraries(id,name,kind,created_at) VALUES(1,'Fixture Movies','movies',1),(2,'Fixture Series','shows',1)`, nil},
		{`INSERT INTO library_paths VALUES(1,'/fixture-only/movies'),(2,'/fixture-only/series')`, nil},
		{`INSERT INTO users(id,name,is_admin,created_at) VALUES('perf-owner','Fixture Owner',1,1)`, nil},
		{sequence + `INSERT INTO items(id,library_id,kind,match_key,title,sort_title,year,added_at,updated_at,original_language,rating) SELECT 'm-'||n,1,'movie','m-'||n,'Movie '||n,'movie '||substr('000000',1,6-length(CAST(n AS TEXT)))||CAST(n AS TEXT),2000+n%25,1700000000+n,1700000000,'en',n%10 FROM numbers`, []any{movies}},
		{sequence + `INSERT INTO items(id,library_id,kind,match_key,title,sort_title,year,added_at,updated_at) SELECT 's-'||n,2,'series','s-'||n,'Series '||n,'series '||substr('000000',1,6-length(CAST(n AS TEXT)))||CAST(n AS TEXT),2020,1700000000+n,1700000000 FROM numbers`, []any{shows}},
		{sequence + `INSERT INTO items(id,library_id,kind,parent_id,match_key,title,sort_title,year,added_at,updated_at,release_date) SELECT 'e-'||n,2,'episode','s-'||CAST((n-1)/?+1 AS INTEGER),'e-'||n,'Episode '||n,'episode '||substr('000000',1,6-length(CAST(n AS TEXT)))||CAST(n AS TEXT),2020,1700000000+n,1700000000,'2020-01-01' FROM numbers`, []any{shows * episodes, episodes}},
		{`INSERT INTO media_files(id,library_id,path,size,mtime,last_seen_at,container,duration_ms,bitrate,probed_at,format_probe_version) SELECT rowid,library_id,'/fixture-only/'||id||'.mkv',1024,1,1,'matroska',3600000,8000000,1,2 FROM items WHERE kind<>'series'`, nil},
		{`INSERT INTO item_files(item_id,file_id,part) SELECT id,rowid,0 FROM items WHERE kind<>'series'`, nil},
		{`INSERT INTO media_streams(file_id,idx,kind,codec,width,height) SELECT id,0,'video','h264',1920,1080 FROM media_files`, nil},
		{`INSERT INTO media_streams(file_id,idx,kind,codec,channels) SELECT id,1,'audio','aac',6 FROM media_files`, nil},
		{`INSERT INTO item_arrivals(item_id,added_at,sequence) SELECT id,added_at,rowid FROM items`, nil},
		{`INSERT INTO item_genres(item_id,name) SELECT id,CASE WHEN rowid%2=0 THEN 'Drama' ELSE 'Action' END FROM items WHERE kind<>'episode'`, nil},
		{`INSERT INTO user_item_data(user_id,item_id,played,position_ms,last_played_at) SELECT 'perf-owner',id,CASE WHEN rowid%100=0 THEN 0 ELSE 1 END,CASE WHEN rowid%100=0 THEN 120000 ELSE 0 END,1700000000+rowid FROM items WHERE kind<>'series' AND rowid%10=0`, nil},
		{`INSERT INTO orderings(id,series_id,source,source_ref,name,updated_at) SELECT CAST(substr(id,3) AS INTEGER),id,'tmdb_aired','','Aired',1 FROM items WHERE kind='series'`, nil},
		{`INSERT INTO ordering_seasons(ordering_id,season_number,name) SELECT id,1,'Season 1' FROM orderings`, nil},
		{`INSERT INTO ordering_entries(ordering_id,episode_id,season_number,episode_number) SELECT CAST(substr(parent_id,3) AS INTEGER),id,1,(CAST(substr(id,3) AS INTEGER)-1)%?+1 FROM items WHERE kind='episode'`, []any{episodes}},
	}
	for i, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
			return fmt.Errorf("fixture statement %d: %w", i+1, err)
		}
	}
	return nil
}
