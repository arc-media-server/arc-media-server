// Package databaseconfig owns the protected local database selector and journaled cutover.
// Credentials cannot live only in the database they are needed to open.
package databaseconfig

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Connection struct {
	Host     string `json:"host" api:"required"`
	Port     int    `json:"port"`
	Database string `json:"database" api:"required"`
	Username string `json:"username" api:"required"`
	Password string `json:"password"`
	TLSMode  string `json:"tls_mode"`
}

func (c Connection) DSN() (string, error) {
	if strings.TrimSpace(c.Host) == "" || strings.TrimSpace(c.Database) == "" || strings.TrimSpace(c.Username) == "" || strings.ContainsAny(c.Host, "/\x00") || len(c.Host) > 255 || len(c.Database) > 255 || len(c.Username) > 255 || len(c.Password) > 4096 {
		return "", errors.New("provide a PostgreSQL host, database and account")
	}
	port := c.Port
	if port == 0 {
		port = 5432
	}
	if port < 1 || port > 65535 {
		return "", errors.New("invalid PostgreSQL port")
	}
	tls := c.TLSMode
	if tls == "" {
		tls = "verify-full"
	}
	if tls != "verify-full" && tls != "disable" {
		return "", errors.New("TLS mode must be verify-full or explicitly disabled for a trusted network")
	}
	u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(c.Host, strconv.Itoa(port)), Path: "/" + c.Database, User: url.UserPassword(c.Username, c.Password)}
	q := url.Values{"sslmode": {tls}, "connect_timeout": {"5"}, "application_name": {"arc-media-server"}, "search_path": {"public"}}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
