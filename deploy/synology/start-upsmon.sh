#!/bin/sh
set -eu

# The bridge binds immediately; allow the first serial Q1 sample to arrive
# before DSM's native monitor performs LOGIN/MASTER.
sleep 2
exec /usr/sbin/upsmon
