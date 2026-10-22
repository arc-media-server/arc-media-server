#!/usr/bin/env bash
# Run in a disposable Linux Codex Cloud environment, not on the media server.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

if [[ "$(uname -s)" != Linux ]]; then
  echo 'This setup is for a disposable Linux cloud environment.' >&2
  exit 1
fi

for tool in go node npm ffmpeg ffprobe cc; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Missing $tool. Ask environment setup to install Go from go.mod, Node 24, FFmpeg and a C compiler before retrying." >&2
    exit 1
  fi
done
node -e 'if (Number(process.versions.node.split(".")[0]) < 24) { console.error("Use Node 24 or newer to match CI."); process.exit(1); }'

# Go selects the version declared by go.mod when toolchain auto-selection is enabled.
go version
node --version
go mod download
(
  cd web
  npm ci
  # This installs Chromium and its Linux system dependencies. Setup needs package access
  # and the normal cloud-environment package-install privileges.
  npx playwright install --with-deps chromium
  npm run build
  npx tsc -p e2e/tsconfig.json
)

echo 'Cloud dependencies prepared. Run bash tools/codex-cloud/verify.sh before publishing.'
