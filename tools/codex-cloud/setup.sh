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
  if [[ "${EUID}" -eq 0 ]] || { command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; }; then
    npx playwright install --with-deps chromium
  else
    # Some cloud bases already supply the OS libraries but do not expose sudo.
    # Download the exact pinned binaries; the launch checks below must still pass.
    npx playwright install chromium
  fi
  node --input-type=module <<'JS'
import { chromium } from '@playwright/test';
for (const options of [{}, { channel: 'chromium' }]) {
  const browser = await chromium.launch(options);
  try {
    const page = await browser.newPage();
    await page.setContent('<title>Arc cloud browser check</title>');
    if (await page.title() !== 'Arc cloud browser check') throw new Error('Browser content check failed');
    console.log(`Pinned ${options.channel ?? 'headless shell'} launch passed: ${browser.version()}`);
  } finally {
    await browser.close();
  }
}
JS
  npm run build
  npx tsc -p e2e/tsconfig.json
)

echo 'Cloud dependencies prepared. Run bash tools/codex-cloud/verify.sh before publishing.'
