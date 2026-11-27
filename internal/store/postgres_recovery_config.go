package store

import (
	"crypto/tls"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func postgresRecoveryConfig(dsn string) (*pgx.ConnConfig, error) {
	dsn = strings.TrimSpace(dsn)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		address, err := url.Parse(dsn)
		if err != nil {
			return nil, errors.New("invalid PostgreSQL recovery connection configuration")
		}
		options, err := url.ParseQuery(address.RawQuery)
		if err != nil {
			return nil, errors.New("invalid PostgreSQL recovery connection configuration")
		}
		if !options.Has("sslmode") {
			options.Set("sslmode", "verify-full")
		}
		address.RawQuery = options.Encode()
		dsn = address.String()
	} else {
		// In pgx keyword syntax later entries override earlier defaults. This also
		// prevents an inherited PGSSLMODE from silently weakening recovery.
		dsn = "sslmode=verify-full " + dsn
	}
	cfg, err := postgresConfig(dsn)
	if err != nil {
		return nil, err
	}

	hasTLS, hasPlainTCP := false, false
	check := func(host string, port uint16, cfg *tls.Config) bool {
		if network, _ := pgconn.NetworkAddress(host, port); network == "unix" {
			return true
		}
		if cfg == nil {
			hasPlainTCP = true
			return true
		}
		hasTLS = true
		return !cfg.InsecureSkipVerify && cfg.ServerName != ""
	}
	valid := check(cfg.Host, cfg.Port, cfg.TLSConfig)
	for _, fallback := range cfg.Fallbacks {
		valid = check(fallback.Host, fallback.Port, fallback.TLSConfig) && valid
	}
	if !valid || hasTLS && hasPlainTCP {
		return nil, errors.New("PostgreSQL recovery requires sslmode=verify-full, or explicit sslmode=disable on a trusted private network")
	}
	return cfg, nil
}
