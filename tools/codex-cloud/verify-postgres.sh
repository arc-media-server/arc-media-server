#!/usr/bin/env bash
# Point only at a disposable PostgreSQL instance with a fixture-admin account.
# Tests create/drop their schemas, databases and roles; production runtime needs only its DB owner.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
: "${ARC_TEST_POSTGRES:?Set ARC_TEST_POSTGRES to an isolated disposable PostgreSQL instance}"
go test ./internal/store ./internal/databaseconfig ./cmd/ams -count=1 -timeout 10m
CGO_ENABLED=1 go test -race ./internal/store ./internal/databaseconfig ./internal/datalock ./internal/api ./cmd/ams -timeout 15m
go vet ./...
