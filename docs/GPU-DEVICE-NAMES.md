# Graphics-card model names

Settings → Playback & transcoding → Graphics card uses the detected adapter model when
available, for example **Intel UHD Graphics 630**, rather than treating **Intel Quick Sync**
as the physical GPU. The hardware acceleration description retains the encoding method.
Identical model names include the render-device suffix. Missing/ambiguous metadata is shown
as **Model unavailable** with the existing encoding/device detail; Arc never invents a model.

Only devices that already pass Arc's generated-frame encode test are named. Names do not
change device order, Auto, saved selection IDs, working-driver/low-power settings, admission,
existing playback sessions or hardware support. Restart the updated server/container for its
normal hardware detection; no library scan or media analysis is involved.

## Discovery and limits

- Linux maps each render node to its own sysfs PCI vendor/device IDs and resolves the offline
  distro `pci.ids` database (`/usr/share/misc/pci.ids` or `/usr/share/hwdata/pci.ids`). Docker
  includes Debian's small **pci.ids data package**, without needing lspci, online lookups or
  added privileges. Driver product names are used when supplied. PCI data can describe a
  model family/shared device ID rather than an exact retail SKU.
- A unique NVIDIA proc entry can supply the model when DRM metadata is unavailable. Only
  Model is read, not UUID/serial fields. Several unbound GPUs remain ambiguous.
- Windows uses Win32_VideoController names and PCI vendor identity via built-in PowerShell;
  macOS uses built-in system_profiler display metadata. An unbound encoder gets a model only
  with one matching adapter; multiple OS-selected adapters remain unnamed.
- Metadata lookup is cosmetic and follows the existing encoding/filter capability tests.
  OS inventory commands have a two-second deadline, bounded output and a short pipe wait;
  Linux reads bounded local metadata/database files. No per-frame/request/poll lookup or
  network request is added; results live in the existing cached capabilities.
- No physical GPU/native OS/owner Docker acceptance is established by cloud fixtures. Hosts
  hiding sysfs/proc, omitting the database/system utilities or reporting ambiguous models
  retain working encoders with the explicit model-unavailable label.

## Additive API handoff

`GET/PATCH /api/v1/settings` → `transcoder.devices[].name` is an optional model string.
Prefer it for the GPU selector; use `label` as method/device detail and distinguish identical
names with the device/ID. The existing `id`, `label`, `encoder`, `device` and active `hw_label`
remain unchanged; persist **id**, never name or label. Old clients can keep using label.
No new endpoint, setting or migration. Generated OpenAPI/v1 contracts retain existing entries.

No new logs: absent model metadata is an expected display fallback. Existing bounded detection
start/outcome/probe diagnostics remain, and cosmetic metadata errors do not become server
failures. Model/UUID/serial inventory and command output are not logged.
