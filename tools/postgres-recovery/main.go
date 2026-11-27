// Command postgres-recovery restores an Arc safety archive to an empty PostgreSQL fixture or replacement database.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ams/internal/store"
)

func main() {
	dsn := os.Getenv("ARC_POSTGRES_RECOVERY_DSN")
	if len(os.Args) != 2 || dsn == "" {
		log.Fatal("set ARC_POSTGRES_RECOVERY_DSN for the empty destination and pass one Arc PostgreSQL archive path")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := store.RestorePostgresBackup(ctx, dsn, os.Args[1]); err != nil {
		log.Fatal(err)
	}
	log.Print("PostgreSQL backup restored")
}
