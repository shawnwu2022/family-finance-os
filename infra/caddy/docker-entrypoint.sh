#!/bin/sh
set -eu

for state_dir in /data/caddy /config/caddy; do
  mkdir -p "$state_dir"
  find "$state_dir" -xdev -exec chown -h 1000:1000 {} +
done

exec su-exec 1000:1000 /usr/bin/caddy "$@"
