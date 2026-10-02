#!/bin/sh
set -eu

INSTALL_DIR=/volume1/docker/santak-nut-bridge
UPSMON_PID=/var/run/upsmon.pid

cleanup() {
	if [ -n "${bridge_pid:-}" ] && kill -0 "$bridge_pid" 2>/dev/null; then
		kill "$bridge_pid" 2>/dev/null || true
		wait "$bridge_pid" 2>/dev/null || true
	fi
}

shutdown() {
	trap - EXIT INT TERM
	cleanup
	exit 0
}

trap cleanup EXIT
trap shutdown INT TERM

"$INSTALL_DIR/santak-nut-bridge" \
	-serial /dev/ttyACM0 \
	-listen 0.0.0.0:3493 \
	-name ups \
	-manufacturer SANTAK \
	-model "Megatec/Q1 UPS" &
bridge_pid=$!

sleep 2
/usr/sbin/upsmon

attempt=0
while [ ! -s "$UPSMON_PID" ]; do
	attempt=$((attempt + 1))
	if [ "$attempt" -ge 10 ]; then
		echo "DSM upsmon did not create $UPSMON_PID" >&2
		exit 1
	fi
	sleep 1
done

while kill -0 "$bridge_pid" 2>/dev/null; do
	upsmon_pid=$(cat "$UPSMON_PID" 2>/dev/null || true)
	if [ -z "$upsmon_pid" ] || ! kill -0 "$upsmon_pid" 2>/dev/null; then
		echo "DSM upsmon stopped unexpectedly" >&2
		exit 1
	fi
	sleep 5
done

wait "$bridge_pid"
