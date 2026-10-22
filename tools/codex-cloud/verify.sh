#!/usr/bin/env bash
# No persistent Arc server, owner database, live provider keys or media mounts needed.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

if [[ "$(uname -s)" != Linux ]]; then
  echo 'Run cloud verification in a disposable Linux environment.' >&2
  exit 1
fi

# Avoid inherited deployment overrides; the browser harness also sanitizes these.
for name in ${!AMS_@}; do unset "$name"; done

(
  cd web
  npm run build
  npx tsc -p e2e/tsconfig.json
)
go vet ./...
go test ./... -timeout 10m
# The race detector needs CGO and a C compiler; do not silently skip this gate.
CGO_ENABLED=1 go test -race ./... -timeout 15m
go build -o /dev/null ./cmd/ams
(
  cd web
  npm run test:e2e:only
)
git diff --check
echo 'Cloud Go, race, embedded-build and browser checks passed.'
