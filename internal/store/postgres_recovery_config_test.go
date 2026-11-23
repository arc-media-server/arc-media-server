package store

import (
	"strings"
	"testing"
)

func TestPostgresRecoveryVerifiesTLSByDefault(t *testing.T) {
	t.Setenv("PGSSLMODE", "prefer")
	t.Setenv("PGSSLROOTCERT", "")
	for _, dsn := range []string{
		"postgres://arc:fixture@database.example/arc",
		"host=database.example user=arc password='fixture secret' dbname=arc",
	} {
		cfg, err := postgresRecoveryConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify || cfg.TLSConfig.ServerName != "database.example" || len(cfg.Fallbacks) != 0 {
			t.Fatal("recovery must verify the certificate and hostname without a plaintext fallback")
		}
	}
	cfg, err := postgresRecoveryConfig("postgres://arc:fixture@database.example/arc?sslmode=disable")
	if err != nil || cfg.TLSConfig != nil {
		t.Fatal("explicit private-network override failed", err)
	}
	for _, mode := range []string{"prefer", "allow", "require", "verify-ca"} {
		_, err := postgresRecoveryConfig("postgres://arc:fixture@database.example/arc?sslmode=" + mode)
		if err == nil || strings.Contains(err.Error(), "fixture") {
			t.Fatalf("recovery accepted or exposed the credentials for %s", mode)
		}
	}
}
