#!/bin/sh
set -eu

if [ ! -c /dev/ttyACM0 ]; then
	echo "UPS serial device /dev/ttyACM0 is missing" >&2
	exit 1
fi

# Remove only the DSM NUT processes that conflict with the bridge. upsmon is
# started again by start-upsmon.sh after the bridge is listening.
for process in usbhid-ups upsd upsmon upssched; do
	/usr/bin/killall "$process" 2>/dev/null || true
done

rm -f /var/run/upsmon.pid /run/upsmon.pid

attempt=0
while netstat -lnt 2>/dev/null | grep -q ':3493[[:space:]]'; do
	attempt=$((attempt + 1))
	if [ "$attempt" -ge 10 ]; then
		echo "TCP port 3493 is still occupied" >&2
		exit 1
	fi
	sleep 1
done
