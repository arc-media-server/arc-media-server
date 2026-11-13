# Name the GPU in the graphics-card selector

Owner report: Intel Quick Sync identifies an encoding technology, not the graphics device.
The selector now prefers an optional detected adapter model (e.g. Intel UHD Graphics 630),
keeps method information separately, distinguishes duplicate models and explicitly describes
unavailable names. Existing saved device IDs, API labels, selection order, fallback and actual
hardware encode tests remain.

Linux resolves each node's sysfs PCI IDs through offline pci.ids data; Docker adds the distro
text-data package. Windows/macOS use bounded built-in inventory commands and only name
unambiguous adapters. No new Go/npm dependency, internet lookup, permission, library/media
work or per-playback discovery. Small local metadata reads run only during existing detection.
Additive API/client details and limits: [GPU-DEVICE-NAMES](../GPU-DEVICE-NAMES.md).

Logging: existing detection outcome/failure diagnostics remain; unavailable model metadata is
an expected UI fallback. No new warnings/info loops or raw inventory/serial/UUID output logged.

Validation and native/container limits are recorded in the PR and task evidence under
/workspace/arc-cloud-evidence/gpu-device-names/. Only temporary stores/generated frames/local
provider/device/sysfs fixtures; no owner media/settings/keys, Unraid or persistent Arc server.


Actual final checks: Go vet/full suite (37 tested packages), complete transcoder race suite
(13.257s), app/production/e2e TypeScript and build pass. All6 affected pinned-browser journeys
pass (56.1s,0failures/skips/retries); desktop/390px captures inspected. Windows amd64/macOS
arm64 transcoder test binaries compile; native inventory execution remains unverified. Official
Debian Trixie pci.ids0.0~2025.06.09-1 was downloaded, SHA256/size checked against the package
index, and its actual database resolved fixture8086:3e92 to Intel UHD Graphics630 using the
new helper. The data was extracted only, not installed. No clean Docker image or physical GPU
encode/name acceptance run. OpenAPI/v1 regeneration adds2name entries and removes0.

Final visual correction restores the unrelated profile selector width and widens only the GPU
selector. Two final hardware journeys pass (21.8s); the last copy-only clarification preserves
the hardware placeholder and explains Quick Sync as the encoding method. Final copy/layout
capture results are recorded in the PR. Earlier six-journey/core Go checks remain preserved.

Final GPU width/copy rechecks:2/2passed after width correction (21.8s), then1/1passed after
the copy-only clarification (18.6s). These repeat earlier cases, not nine distinct journeys.
Final screenshots show full model names within the desktop/phone control. No unrelated
selector width change remains. All runtime Go/transcoder code is unchanged since its green
full/race/cross-compile checks; the final production build/typechecks also pass.
