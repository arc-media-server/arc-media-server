// Command pgcatalog regenerates release catalogs in a disposable PostgreSQL database.
package main

import (
	"ams/internal/store"
	"context"
	"log"
	"os"
)

func main() {
	dsn := os.Getenv("ARC_TEST_POSTGRES")
	if dsn == "" {
		log.Fatal("ARC_TEST_POSTGRES must select a disposable PostgreSQL test database")
	}
	if err := store.GeneratePostgresReleaseCatalogs(context.Background(), dsn, "internal/store/postgres/releases"); err != nil {
		log.Fatal(err)
	}
}
