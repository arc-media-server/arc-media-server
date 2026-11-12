# Container permissions

Arc's image runs as UID/GID `10001:10001`, without root privileges. Port 8099 does not need
root. The image owns `/config`, and new Docker-managed volumes initialized from it inherit
that ownership. The container never changes ownership of mounted host files.

For a bind mount, create the config folder before starting. Make that folder and its existing
Arc files writable by the account that will run Arc. Compose's `user: "UID:GID"` can select a
host account instead of `10001:10001`; environment variables `PUID` and `PGID` have no effect.
Use numeric IDs to avoid assuming host/container account names match. Keep config local; media
can be mounted separately, read-only. Avoid making config world-writable.

## Existing installations that ran as root

1. Stop Arc and keep a config backup.
2. Identify the actual Arc config bind mount and the desired UID/GID. On Unraid or another NAS,
   use its account/permission tools or Compose's explicit user override; do not assume a Linux
   desktop's account IDs apply.
3. Grant that account access to **Arc's config directory and its existing files only**. A recursive
   ownership change may be necessary for old root-owned files; inspect the exact target first.
   Never apply it to media, a shared parent, or another application's data.
4. Start the new image. Check health and confirm settings, libraries and a fresh backup work.
   No database conversion/rescan is required just to change the process user.

If config is not writable, startup fails with a permission error. Arc does not silently chown
host mounts, make data public, or rerun itself as root. The owner decides how their host account
and storage permissions should be configured.

## Hardware conversion

`devices: ["/dev/dri:/dev/dri"]` exposes Intel/AMD device nodes but does not change their Unix
permissions. Check their numeric owners/groups on the host, then grant only the needed render
and/or video groups with Compose `group_add`. Add the numeric GIDs; container group names need
not match the host. Device IDs differ between servers. NVIDIA still needs its container runtime
and device permissions. No physical encoder is guaranteed by merely exposing a device.

The cloud validation uses generated read-only media and temporary config volumes. Physical
GPU access, NAS permissions, existing owner config and remote deployment remain owner checks.

When pre-provisioning a *new, empty named volume* for a different UID, use Docker's
`volume-nocopy` mount option for that override after provisioning. Otherwise a later container
can populate the still-empty volume from the image again and reset its directory ownership.
Ordinary fresh volumes with the default image user need no override or pre-provisioning.
