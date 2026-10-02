#!/bin/sh
set -eu

INSTALL_DIR=/volume1/docker/santak-nut-bridge
DROPIN_FILE=/etc/systemd/system/ups-usb.service.d/override.conf
BACKUP_FILE=$INSTALL_DIR/override.conf.before-santak

if [ "$(id -u)" -ne 0 ]; then
	echo "Run this uninstaller as root" >&2
	exit 1
fi

systemctl stop ups-usb.service || true
rm -f "$DROPIN_FILE"
if [ -f "$BACKUP_FILE" ]; then
	cp "$BACKUP_FILE" "$DROPIN_FILE"
fi
systemctl daemon-reload
systemctl start ups-usb.service
echo "DSM UPS service restored"
