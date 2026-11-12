# The container runs Arc as PUID:PGID, handing its own config over on start

## What changed

- The image now starts a small script as root (`docker/arc-start.sh`), as linuxserver.io's Plex
  and Jellyfin images do. It:
  - gives Arc's own `/config` to `PUID:PGID` (default `1000:1000`), including the root-owned files
    of an install from before the non-root change. It doesn't follow symlinks, cross into another
    mounted filesystem or touch files already owned. It logs how many entries it changed;
  - adds the groups that own any passed-through `/dev/dri` devices;
  - runs Arc as `PUID:PGID` through `setpriv`, with no inherited capabilities.
- `PUID=0` runs Arc as root, for anyone who asks for it. Docker's `--user` still works: the script
  then runs Arc straight away, and the config must already be writable by that user.
- Compose, the README and `docs/CONTAINER-PERMISSIONS.md` say this; `group_add` is no longer needed
  for Intel/AMD.
- The container smoke test checks four cases: the default, Unraid-style `PUID=99 PGID=100` over a
  root-owned config, `PUID=0`, and Docker's `--user 12345`. It checks Arc's own process user
  (`/proc/1`) rather than `docker exec`'s.

## Why

#382 (merged in #418) made the image run as a fixed UID 10001 and left ownership to the person
installing it. That keeps Arc off root, but an existing install (root-owned config) failed to start
until someone changed ownership by hand, and `/dev/dri` needed `group_add`. This keeps the same
protection (Arc itself never runs as root) and makes installs and upgrades work with no steps.

## Decisions

- **Default 1000:1000:** the first account on most Linux machines, so a Compose bind mount ends up
  owned by the person who ran it. Unraid users can set 99:100 to match their other containers.
- **Only Arc's own config is changed,** never media or any other mount.

## Logging

One line on start when the script hands config entries over, with the count; nothing otherwise.
A bad `PUID`/`PGID` stops the container with a message saying so.
