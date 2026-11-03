// Command dbschema regenerates the PostgreSQL baseline from Arc's SQLite migration history.
package main

import (
	"ams/internal/store"
	"context"
	"log"
)

func main() {
	if err := store.GeneratePostgresBaseline(context.Background(), "internal/store/postgres"); err != nil {
		log.Fatal(err)
	}
}
