#!/usr/bin/env bash
set -euo pipefail

SOURCE_ROOT="${CI_SOURCE_ROOT:-/src}"
[[ -f "$SOURCE_ROOT/web/package.json" ]] || {
  echo "web verification requires repository mounted at $SOURCE_ROOT" >&2
  exit 1
}

WORK_ROOT="$(mktemp -d /tmp/family-finance-web-verify.XXXXXX)"
cleanup() {
  rm -rf "$WORK_ROOT"
}
trap cleanup EXIT

cp -a "$SOURCE_ROOT/web" "$WORK_ROOT/web"
cd "$WORK_ROOT/web"

[[ "$(node --version)" == "v24.19.0" ]] || {
  echo "expected Node v24.19.0, got $(node --version)" >&2
  exit 1
}

npm ci --ignore-scripts
npm audit --audit-level=high
npm test
npm run check:pwa
npm run typecheck
npm run build

echo "Web verification OK"
