#!/bin/sh
set -eu

INSTALL_DIR=/volume1/docker/santak-nut-bridge
DROPIN_DIR=/etc/systemd/system/ups-usb.service.d
DROPIN_FILE=$DROPIN_DIR/override.conf
BACKUP_FILE=$INSTALL_DIR/override.conf.before-santak

if [ "$(id -u)" -ne 0 ]; then
	echo "Run this installer as root" >&2
	exit 1
fi

for file in santak-nut-bridge prepare.sh run.sh override.conf; do
	if [ ! -f "$INSTALL_DIR/$file" ]; then
		echo "Missing $INSTALL_DIR/$file" >&2
		exit 1
	fi
done

chmod 755 "$INSTALL_DIR/santak-nut-bridge" "$INSTALL_DIR/prepare.sh" "$INSTALL_DIR/run.sh"

# Stop with the vendor unit definition still active so DSM cleans up its
# usbhid-ups, upsd, and upsmon processes normally.
systemctl stop ups-usb.service || true
mkdir -p "$DROPIN_DIR"
if [ -f "$DROPIN_FILE" ] && ! grep -q '^# Managed by santak-nut-bridge$' "$DROPIN_FILE"; then
	cp -p "$DROPIN_FILE" "$BACKUP_FILE"
fi
cp "$INSTALL_DIR/override.conf" "$DROPIN_FILE"
systemctl daemon-reload

if ! systemctl start ups-usb.service; then
	echo "Bridge failed to start; restoring the DSM unit" >&2
	rm -f "$DROPIN_FILE"
	if [ -f "$BACKUP_FILE" ]; then
		cp "$BACKUP_FILE" "$DROPIN_FILE"
	fi
	systemctl daemon-reload
	systemctl start ups-usb.service || true
	exit 1
fi

systemctl is-active --quiet ups-usb.service
echo "santak-nut-bridge installed and active"
