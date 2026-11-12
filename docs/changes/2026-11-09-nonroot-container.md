# #350: unprivileged container runtime

The production Dockerfile had no USER and therefore ran Arc/FFmpeg as root. Make its default
runtime UID/GID 10001, own only the image's empty /config directory, and set HOME there for
runtime caches. No privileged entrypoint, host chown, root fallback or media writes are added.

Compatibility matters: existing root-owned bind-mounted config needs an explicit owner-managed
permission migration or a suitable Compose user override. New managed volumes inherit the
image directory's ownership. README, Compose comments and the permissions guide explain this,
including numeric GPU supplementary groups. No host accounts, owner config or devices are
changed by this PR. The owner reviews/merges it before any deployment change.

Runtime validation must use isolated config and generated read-only media. Physical GPU/NAS
permissions remain unverified. No new logging: existing startup permission errors and job/scan
outcomes cover this boundary; no secret/body/URL or per-file log output is added.

Extend the existing isolated Docker smoke tool rather than adding another: assert image USER,
exercise the unmodified image-default user with a fresh managed volume, retain explicit root
and arbitrary numeric-user overrides, and run fixture-only ownership/copy helpers explicitly
as root. With an optional previous image, provision the same chosen UID for both versions;
this does not claim automatic migration of root-owned host files.

Validation: the production runtime stage built on Debian trixie with pinned jellyfin-ffmpeg8
8.1.3-2 (binary 8.1.3-Jellyfin), using a locally compiled current-source binary and prepared UI.
This was a runtime-stage build, not the full Node/Go multistage build. Cloud trust was mounted
only during downloads in the external test Dockerfile. Isolated runtime checks passed setup,
generated read-only video scanning, backup writing, unchanged source hash, graceful stop,
unwritable-config refusal without chown and numeric-user override. The corrected existing smoke
passed default UID 10001, explicit UID 0 and UID 12345, including backup/restore, restart and
volume persistence. Its first override fixture exposed Docker's empty-volume copy behavior;
retain that failure evidence and use volume-nocopy for pre-provisioned override volumes.
Go vet and the full Go suite passed; no Go packages changed. Physical GPU/NAS access, existing
owner data and earlier-image upgrade/rollback were not tested in this run.
