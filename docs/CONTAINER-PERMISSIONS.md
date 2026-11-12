# Container permissions

Arc never runs as root in its container. It runs as the account in `PUID` and `PGID`, the way
linuxserver.io's Plex and Jellyfin images work:

- Default `PUID=1000`, `PGID=1000`. On Unraid use `PUID=99`, `PGID=100` (nobody:users), as for
  your other containers. On a Linux desktop, `id -u` and `id -g` give your own account's numbers.
- The container's start script (`docker/arc-start.sh`) runs as root only for a moment. It:
  1. gives Arc's own config folder (`/config`) and the files in it to `PUID:PGID`, including the
     root-owned files of an install from before 13 November 2026. It never follows symlinks,
     never crosses into another mounted filesystem, and leaves files already owned alone;
  2. adds the groups that own any passed-through `/dev/dri` devices, so Intel/AMD hardware
     conversion works without `group_add`;
  3. then runs Arc as `PUID:PGID` with no extra privileges.
- Media and any other mounts are never changed. Mount media read-only.
- `PUID=0` runs Arc as root, for anyone who really wants it. It isn't recommended.

## Docker's own `--user`

Starting the container with `--user UID:GID` (Compose `user:`) still works: the container is then
unprivileged from the first instruction, so the start script can't change ownership. The config
folder must already be writable by that user, and `group_add` is needed for `/dev/dri`.

## Upgrading an install that ran as root

Nothing to do by hand: set `PUID`/`PGID` and start the new image. The first start hands the
existing config files over and logs how many it changed. Keeping a config backup first is still
wise. No database conversion or rescan is needed.

## Hardware conversion

Passing `/dev/dri` is enough for Intel/AMD with `PUID`/`PGID`. NVIDIA still needs its container
runtime. No physical encoder is guaranteed by merely exposing a device.

The container smoke test (`tools/docker-smoke.mjs`) checks the default, an Unraid-style
`PUID=99`/`PGID=100` over a root-owned config, `PUID=0` and Docker's `--user`. Physical GPU
access, NAS permissions and owner deployments remain owner checks.
