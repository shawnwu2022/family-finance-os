#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

TRIVY_IMAGE='ghcr.io/aquasecurity/trivy:0.73.0@sha256:7cced7cae583819fc7806d4cbc0dbbc7cad18b99f7d3e235192e6da8c091045c'
SUFFIX="${GITHUB_RUN_ID:-local}-$$"
POSTGRES_IMAGE="family-finance/postgres-hardened:${SUFFIX}"
EZBOOKKEEPING_IMAGE="family-finance/ezbookkeeping-hardened:${SUFFIX}"
CADDY_IMAGE="family-finance/caddy-hardened:${SUFFIX}"
POSTGRES_CONTAINER="ffos-postgres-security-${SUFFIX}"
CADDY_CONTAINER="ffos-caddy-security-${SUFFIX}"
WORK_DIR="$(mktemp -d)"
TRIVY_CACHE="$WORK_DIR/trivy-cache"
mkdir -p "$TRIVY_CACHE"

cleanup() {
  docker rm -f "$POSTGRES_CONTAINER" "$CADDY_CONTAINER" >/dev/null 2>&1 || true
  docker image rm -f "$POSTGRES_IMAGE" "$EZBOOKKEEPING_IMAGE" "$CADDY_IMAGE" >/dev/null 2>&1 || true
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

wait_for_postgres() {
  local attempt
  for attempt in $(seq 1 45); do
    if docker exec "$POSTGRES_CONTAINER" pg_isready -U probe -d probe >/dev/null 2>&1; then
      return 0
    fi
    if ! docker inspect -f '{{.State.Running}}' "$POSTGRES_CONTAINER" 2>/dev/null | grep -qx true; then
      docker logs "$POSTGRES_CONTAINER" >&2 || true
      return 1
    fi
    sleep 2
  done
  docker logs "$POSTGRES_CONTAINER" >&2 || true
  return 1
}

wait_for_http() {
  local url="$1"
  local attempt
  for attempt in $(seq 1 30); do
    if curl -fsS "$url" >/dev/null; then
      return 0
    fi
    if ! docker inspect -f '{{.State.Running}}' "$CADDY_CONTAINER" 2>/dev/null | grep -qx true; then
      docker logs "$CADDY_CONTAINER" >&2 || true
      return 1
    fi
    sleep 1
  done
  docker logs "$CADDY_CONTAINER" >&2 || true
  return 1
}

scan_image() {
  local component="$1"
  local image="$2"
  local report="$WORK_DIR/${component}-trivy.json"

  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$TRIVY_CACHE:/root/.cache/" \
    "$TRIVY_IMAGE" image \
      --quiet \
      --format json \
      --scanners vuln \
      --severity HIGH,CRITICAL \
      "$image" >"$report"

  local total
  total="$(jq '[.Results[]?.Vulnerabilities[]?] | length' "$report")"
  echo "COMPONENT=$component"
  echo "SCAN_TOTAL=$total"

  if (( total > 0 )); then
    jq -r '.Results[]?.Vulnerabilities[]? | [.VulnerabilityID,.PkgName,.InstalledVersion,(.FixedVersion // ""),.Severity] | @tsv' "$report" | head -n 100 >&2
    return 1
  fi
}

echo 'Building hardened runtime images...'
docker build --pull -f Dockerfile.postgres -t "$POSTGRES_IMAGE" .
docker build --pull -f Dockerfile.ezbookkeeping -t "$EZBOOKKEEPING_IMAGE" .
docker build --pull -f Dockerfile.caddy -t "$CADDY_IMAGE" .

echo 'Smoke-testing hardened PostgreSQL...'
docker run -d --rm \
  --name "$POSTGRES_CONTAINER" \
  -e POSTGRES_USER=probe \
  -e POSTGRES_PASSWORD=probe-password \
  -e POSTGRES_DB=probe \
  "$POSTGRES_IMAGE" >/dev/null
wait_for_postgres
test "$(docker exec "$POSTGRES_CONTAINER" psql -U probe -d probe -Atqc 'SELECT 1')" = '1'
docker stop "$POSTGRES_CONTAINER" >/dev/null

echo 'Smoke-testing hardened ezBookkeeping...'
ez_version="$(docker run --rm --entrypoint /ezbookkeeping/ezbookkeeping "$EZBOOKKEEPING_IMAGE" --version)"
echo "$ez_version"
grep -Fq 'ezBookkeeping version 1.6.1' <<<"$ez_version"
grep -Fq 'commit 6ccd0c4' <<<"$ez_version"

echo 'Validating repository Caddy configuration...'
auth_hash="$(docker run --rm "$CADDY_IMAGE" hash-password --plaintext runtime-security-probe)"
docker run --rm \
  -e EBK_DOMAIN=ebk.example.test \
  -e FINANCE_DOMAIN=finance.example.test \
  -e FINANCE_AUTH_USER=probe \
  -e FINANCE_AUTH_HASH="$auth_hash" \
  -v "$ROOT_DIR/Caddyfile:/etc/caddy/Caddyfile:ro" \
  "$CADDY_IMAGE" validate --config /etc/caddy/Caddyfile --adapter caddyfile

cat >"$WORK_DIR/Caddyfile.smoke" <<'EOF'
:80 {
    respond "runtime-security-ok" 200
}
EOF

echo 'Smoke-testing non-root Caddy on the production HTTP port...'
docker run -d --rm \
  --name "$CADDY_CONTAINER" \
  -p 127.0.0.1:18080:80 \
  -v "$WORK_DIR/Caddyfile.smoke:/etc/caddy/Caddyfile:ro" \
  "$CADDY_IMAGE" >/dev/null
wait_for_http 'http://127.0.0.1:18080/'
test "$(curl -fsS http://127.0.0.1:18080/)" = 'runtime-security-ok'
docker stop "$CADDY_CONTAINER" >/dev/null

echo 'Scanning hardened runtime images for HIGH/CRITICAL vulnerabilities...'
scan_image postgres "$POSTGRES_IMAGE"
scan_image ezbookkeeping "$EZBOOKKEEPING_IMAGE"
scan_image caddy "$CADDY_IMAGE"

echo 'Runtime image security verification OK'
