#!/bin/sh
set -eu

if [ "$#" -gt 0 ]; then
  exec "$@"
fi

secret_file="${EBK_SECURITY_SECRET_KEY_FILE:-/run/secrets/ezbookkeeping-secret-key}"
if [ ! -f "$secret_file" ]; then
  echo "ezBookkeeping security secret file is missing or not a regular file" >&2
  exit 1
fi

mode="$(stat -c '%a' "$secret_file")"
case "$mode" in
  400|600) ;;
  *)
    echo "ezBookkeeping security secret file must use mode 0400 or 0600" >&2
    exit 1
    ;;
esac

size="$(wc -c < "$secret_file" | tr -d '[:space:]')"
if [ -z "$size" ] || [ "$size" -le 0 ] || [ "$size" -gt 4096 ]; then
  echo "ezBookkeeping security secret file has an invalid size" >&2
  exit 1
fi

EBK_SECURITY_SECRET_KEY="$(cat "$secret_file")"
if [ -z "$EBK_SECURITY_SECRET_KEY" ]; then
  echo "ezBookkeeping security secret is empty" >&2
  exit 1
fi
export EBK_SECURITY_SECRET_KEY
unset EBK_SECURITY_SECRET_KEY_FILE

if [ -n "${EBK_CONF_PATH:-}" ]; then
  exec /ezbookkeeping/ezbookkeeping server run --conf-path="$EBK_CONF_PATH"
fi
exec /ezbookkeeping/ezbookkeeping server run
