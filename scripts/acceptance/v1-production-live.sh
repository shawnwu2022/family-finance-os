#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "V1 production live acceptance failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

for command_name in curl docker git sha256sum stat readlink date grep awk find sort xargs; do
  command -v "$command_name" >/dev/null || fail "$command_name is required"
done

docker compose version >/dev/null

evidence_root="${V1_PRODUCTION_EVIDENCE_DIR:-}"
[[ -n "$evidence_root" ]] || fail "V1_PRODUCTION_EVIDENCE_DIR is required"
[[ "$evidence_root" == /* ]] || fail "V1_PRODUCTION_EVIDENCE_DIR must be an absolute path"

resolved_evidence_root="$(readlink -m -- "$evidence_root")"
case "$resolved_evidence_root" in
  "$ROOT_DIR"|"$ROOT_DIR"/*)
    fail "V1_PRODUCTION_EVIDENCE_DIR must live outside the repository"
    ;;
esac

finance_url="${FINANCE_PUBLIC_URL:-}"
ebk_url="${EBK_PUBLIC_URL:-}"
for spec in "FINANCE_PUBLIC_URL:$finance_url" "EBK_PUBLIC_URL:$ebk_url"; do
  label="${spec%%:*}"
  value="${spec#*:}"
  [[ "$value" =~ ^https://[^/?#@]+/?$ ]] \
    || fail "$label must be an HTTPS origin without path, query, fragment, or credentials"
done
finance_url="${finance_url%/}"
ebk_url="${ebk_url%/}"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
run_id="${V1_PRODUCTION_RUN_ID:-$timestamp}"
[[ "$run_id" =~ ^[A-Za-z0-9._-]+$ ]] || fail "V1_PRODUCTION_RUN_ID contains unsupported characters"

mkdir -p "$resolved_evidence_root"
chmod 0700 "$resolved_evidence_root"
run_dir="$resolved_evidence_root/$run_id"
[[ ! -e "$run_dir" ]] || fail "evidence run directory already exists: $run_dir"
mkdir -m 0700 "$run_dir"
raw_dir="$run_dir/raw"
mkdir -m 0700 "$raw_dir"
status_file="$run_dir/status.tsv"
metadata_file="$run_dir/metadata.tsv"
: >"$status_file"
: >"$metadata_file"
chmod 0600 "$status_file" "$metadata_file"

record() {
  local gate="$1"
  local status="$2"
  local detail="$3"
  detail="${detail//$'\n'/ }"
  detail="${detail//$'\t'/ }"
  printf '%s\t%s\t%s\n' "$gate" "$status" "$detail" >>"$status_file"
}

commit_sha="$(git rev-parse HEAD)"
printf 'run_id\t%s\n' "$run_id" >>"$metadata_file"
printf 'executed_at_utc\t%s\n' "$timestamp" >>"$metadata_file"
printf 'commit\t%s\n' "$commit_sha" >>"$metadata_file"
printf 'finance_origin\t%s\n' "$finance_url" >>"$metadata_file"
printf 'ledger_origin\t%s\n' "$ebk_url" >>"$metadata_file"

if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  fail "tracked repository files are modified on the production checkout"
fi
record "production_checkout_clean" "PASS" "$commit_sha"

if bash scripts/preflight.sh >"$raw_dir/preflight.txt" 2>&1; then
  record "production_preflight" "PASS" "repository-defined preflight passed"
else
  cat "$raw_dir/preflight.txt" >&2
  fail "repository-defined preflight failed"
fi

if bash scripts/check-edge-security.sh >"$raw_dir/edge-static.txt" 2>&1; then
  record "edge_static_contract" "PASS" "repository edge contract passed"
else
  cat "$raw_dir/edge-static.txt" >&2
  fail "repository edge contract failed"
fi

docker compose ps >"$raw_dir/compose-ps.txt"
record "compose_runtime_visible" "PASS" "docker compose ps captured"

: >"$raw_dir/runtime-ports.tsv"
caddy_seen=0
caddy_80=0
caddy_443_tcp=0
caddy_443_udp=0
while IFS= read -r container_id; do
  [[ -n "$container_id" ]] || continue
  service="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.service" }}' "$container_id")"
  bindings="$(docker inspect --format '{{json .HostConfig.PortBindings}}' "$container_id")"
  printf '%s\t%s\n' "$service" "$bindings" >>"$raw_dir/runtime-ports.tsv"

  if [[ "$service" == "caddy" ]]; then
    caddy_seen=1
    [[ "$bindings" == *'"80/tcp"'* ]] && caddy_80=1
    [[ "$bindings" == *'"443/tcp"'* ]] && caddy_443_tcp=1
    [[ "$bindings" == *'"443/udp"'* ]] && caddy_443_udp=1
    continue
  fi

  if [[ "$bindings" != "{}" && "$bindings" != "null" ]]; then
    fail "non-Caddy service publishes host ports: $service"
  fi
done < <(docker compose ps -q)

[[ "$caddy_seen" == "1" ]] || fail "running Caddy service was not found"
[[ "$caddy_80" == "1" && "$caddy_443_tcp" == "1" && "$caddy_443_udp" == "1" ]] \
  || fail "Caddy must publish 80/tcp, 443/tcp and 443/udp"
record "runtime_host_port_exposure" "PASS" "only Caddy publishes the required host ports"

curl_https() {
  curl --silent --show-error --fail \
    --proto '=https' --proto-redir '=https' --tlsv1.2 \
    --connect-timeout 5 --max-time 20 "$@"
}

curl_https "$finance_url/healthz" >"$raw_dir/finance-healthz.txt"
[[ -s "$raw_dir/finance-healthz.txt" ]] || fail "Finance /healthz returned an empty response"
record "finance_healthz_https" "PASS" "HTTPS health endpoint succeeded"

curl_https "$finance_url/readyz" >"$raw_dir/finance-readyz.txt"
[[ -s "$raw_dir/finance-readyz.txt" ]] || fail "Finance /readyz returned an empty response"
record "finance_readyz_https" "PASS" "HTTPS readiness endpoint succeeded"

curl_https "$ebk_url/" >"$raw_dir/ezbookkeeping-root.html"
[[ -s "$raw_dir/ezbookkeeping-root.html" ]] || fail "ezBookkeeping HTTPS root returned an empty response"
record "ezbookkeeping_https" "PASS" "ledger HTTPS endpoint succeeded"

curl_https "$finance_url/manifest.webmanifest" >"$raw_dir/manifest.webmanifest"
[[ -s "$raw_dir/manifest.webmanifest" ]] || fail "PWA manifest is empty"
grep -Fq '"display"' "$raw_dir/manifest.webmanifest" || fail "PWA manifest has no display field"
record "finance_pwa_manifest_https" "PASS" "PWA manifest fetched over HTTPS"

curl_https "$finance_url/sw.js" >"$raw_dir/sw.js"
[[ -s "$raw_dir/sw.js" ]] || fail "PWA service worker is empty"
record "finance_pwa_service_worker_https" "PASS" "service worker fetched over HTTPS"

period="$(TZ=Asia/Shanghai date +%Y-%m)"
unauth_status="$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
  --proto '=https' --proto-redir '=https' --tlsv1.2 \
  --connect-timeout 5 --max-time 20 \
  "$finance_url/api/v1/dashboard?period=$period")"
[[ "$unauth_status" == "401" ]] || fail "unauthenticated Finance dashboard status=$unauth_status, want 401"
record "finance_unauthenticated_dashboard" "PASS" "protected dashboard rejects unauthenticated requests with 401"

# These gates require real human credentials, financial facts, a physical phone,
# or an independent recovery environment. Keeping them explicit prevents an
# automated host script from promoting them to PASS without real evidence.
record "finance_password_totp_production_login" "NOT_RUN" "requires real owner login and TOTP enrollment"
record "finance_logout_revocation_csrf_household" "NOT_RUN" "requires authenticated browser acceptance"
record "browser_mcp_credential_separation" "NOT_RUN" "requires real browser session and optional MCP bearer"
record "ezbookkeeping_owner_2fa_enrollment" "NOT_RUN" "requires owner-side confirmation"
record "real_chinese_statement_import" "NOT_RUN" "requires a real statement outside Git"
record "complete_natural_month_reconciliation" "NOT_RUN" "requires independent financial reconciliation"
record "safe_to_spend_debt_scenario_crosscheck" "NOT_RUN" "requires independent calculator or spreadsheet evidence"
record "real_external_advisor_provider" "NOT_RUN" "requires the production provider and exact model"
record "real_backup_append_only_offhost_restore" "NOT_RUN" "requires backup and independent recovery hosts"
record "scheduler_production_restart_recovery" "NOT_RUN" "requires controlled production-like restart exercise"
record "real_phone_pwa_install" "NOT_RUN" "requires a physical phone over production HTTPS"
record "final_secret_hygiene_review" "NOT_RUN" "requires operator review of host and evidence logs"

(
  cd "$run_dir"
  find . -type f ! -name SHA256SUMS -print0 \
    | LC_ALL=C sort -z \
    | xargs -0 sha256sum >SHA256SUMS
)
chmod 0600 "$run_dir/SHA256SUMS"
index_sha256="$(sha256sum "$run_dir/SHA256SUMS" | awk '{print $1}')"

printf 'v1_production_live automated=PASS evidence_dir=%s index_sha256=%s manual_gates=NOT_RUN\n' \
  "$run_dir" "$index_sha256"
