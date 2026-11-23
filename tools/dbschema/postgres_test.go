package main

import (
	"strings"
	"testing"
)

func TestGenerateCurrentSchema(t *testing.T) {
	first, err := generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatal("schema output varied across fresh SQLite databases")
	}
	for _, part := range []string{
		`CREATE TABLE "downloads"`,
		`"source_cache_key" TEXT COLLATE "C" NOT NULL DEFAULT ''`,
		`"is_tool" BIGINT NOT NULL DEFAULT 0`,
		`CHECK ( is_tool IN ( 0 , 1 ) )`,
		`CREATE UNIQUE INDEX item_labels_folded ON item_labels(item_id,arc_lower(label))`,
		`FOREIGN KEY ("user_id") REFERENCES "users"("id")`,
	} {
		if !strings.Contains(first, part) {
			t.Errorf("generated schema missing %s", part)
		}
	}
}
