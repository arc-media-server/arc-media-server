#!/bin/sh
# Arc's container start, the way linuxserver.io's Plex and Jellyfin images work: the container
# starts as root only long enough to give Arc's own config folder to PUID:PGID (Unraid: 99:100),
# let that user reach passed-through graphics devices, and then run Arc as that user. Arc never
# runs as root unless PUID=0 is asked for. Media and anything else mounted are never touched.
# Started with Docker's --user (or Compose user:), it is already unprivileged and just runs Arc.
set -eu
arc=/usr/local/bin/ams

if [ "$(id -u)" != 0 ]; then
	exec "$arc" "$@"
fi

PUID=${PUID:-1000}
PGID=${PGID:-1000}
case "$PUID:$PGID" in
	*[!0-9:]* | :* | *:) echo "arc: PUID and PGID must be numbers (got PUID=$PUID PGID=$PGID)" >&2; exit 1 ;;
esac
if [ "$PUID" = 0 ]; then
	echo "arc: PUID=0, running as root" >&2
	exec "$arc" "$@"
fi

config=${AMS_DATA:-/config}
mkdir -p "$config"
# Only Arc's own folder, never across into another filesystem (a media mount placed inside it),
# and never following symlinks. Files already owned by PUID:PGID are left alone, so after the
# first start this only reads the folder.
changed=$(find "$config" -xdev \( ! -user "$PUID" -o ! -group "$PGID" \) -exec chown -h "$PUID:$PGID" {} + -print | wc -l)
if [ "$changed" -gt 0 ]; then
	echo "arc: gave $changed config entries to $PUID:$PGID" >&2
fi
chmod u+rwx "$config"

# Graphics devices for hardware conversion (/dev/dri): add the groups that own them.
groups=$PGID
for device in /dev/dri/*; do
	[ -e "$device" ] || continue
	group=$(stat -c %g "$device")
	case ",$groups," in *",$group,"*) ;; *) groups="$groups,$group" ;; esac
done

exec setpriv --reuid="$PUID" --regid="$PGID" --groups="$groups" --inh-caps=-all "$arc" "$@"
