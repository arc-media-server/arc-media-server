package databaseconfig

import (
	"ams/internal/store"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestVerifiedTLSCutoverAndCertificateFailures(t *testing.T) {
	dsn := os.Getenv("ARC_TEST_POSTGRES_TLS")
	if dsn == "" {
		t.Skip("separate TLS-enabled isolated PostgreSQL fixture not configured")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARC_TEST_POSTGRES", dsn)
	t.Setenv("PGSSLROOTCERT", parsed.Query().Get("sslrootcert"))
	c := freshTarget(t)
	c.TLSMode = "verify-full"
	m, _ := sqliteInstance(t)
	if err = m.Request(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	if err = m.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	pg, err := m.Open(t.Context(), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	pg.Close()
	wrongHost := c
	wrongHost.Host = "127.0.0.1"
	if _, err = m.Test(t.Context(), wrongHost); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatal("certificate hostname was not checked", err)
	}
	t.Setenv("PGSSLROOTCERT", "")
	if _, err = m.Test(t.Context(), c); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatal("untrusted CA was accepted", err)
	}
	if strings.Contains(err.Error(), c.Password) || strings.Contains(err.Error(), dsn) {
		t.Fatal("certificate failure exposed connection secrets")
	}
}
