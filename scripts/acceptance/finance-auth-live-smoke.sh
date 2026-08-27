#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "Finance auth live smoke failed: $*" >&2
  exit 1
}

for command_name in go curl openssl sha256sum sed awk; do
  command -v "$command_name" >/dev/null || fail "$command_name is required"
done

for key in TEST_POSTGRES_HOST TEST_POSTGRES_PORT TEST_POSTGRES_DB TEST_POSTGRES_USER TEST_POSTGRES_PASSWORD; do
  [[ -n "${!key:-}" ]] || fail "$key is required"
done

workdir="$(mktemp -d /tmp/family-finance-auth-smoke.XXXXXX)"
chmod 0700 "$workdir"
finance_pid=""
ledger_pid=""
cleanup() {
  set +e
  if [[ -n "$finance_pid" ]]; then
    kill "$finance_pid" >/dev/null 2>&1 || true
    wait "$finance_pid" >/dev/null 2>&1 || true
  fi
  if [[ -n "$ledger_pid" ]]; then
    kill "$ledger_pid" >/dev/null 2>&1 || true
    wait "$ledger_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT INT TERM

finance_bin="$workdir/finance-core"
helper_bin="$workdir/auth-smoke-helper"
auth_key_file="$workdir/finance-auth-key"
admin_password_file="$workdir/finance-admin-password"
ledger_token_file="$workdir/ezbookkeeping-api-token"

CGO_ENABLED=0 go build -buildvcs=false -trimpath -o "$finance_bin" ./cmd/finance-core

cat >"$workdir/auth-smoke-helper.go" <<'EOF_GO'
package main

import (
  "crypto/hmac"
  "crypto/sha1"
  "encoding/base32"
  "encoding/binary"
  "encoding/json"
  "fmt"
  "os"
  "strings"
  "time"
)

func die(err error) {
  if err != nil {
    fmt.Fprintln(os.Stderr, "auth smoke helper failed")
    os.Exit(1)
  }
}

func load(path string) map[string]any {
  data, err := os.ReadFile(path)
  die(err)
  var payload map[string]any
  die(json.Unmarshal(data, &payload))
  return payload
}

func field(path, name string) {
  payload := load(path)
  value, ok := payload[name].(string)
  if !ok || strings.TrimSpace(value) == "" {
    os.Exit(1)
  }
  fmt.Print(value)
}

func totpFromLogin(path string) {
  payload := load(path)
  secret, ok := payload["totp_secret"].(string)
  if !ok || strings.TrimSpace(secret) == "" {
    os.Exit(1)
  }
  normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
  key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
  die(err)
  counter := uint64(time.Now().UTC().Unix() / 30)
  var message [8]byte
  binary.BigEndian.PutUint64(message[:], counter)
  mac := hmac.New(sha1.New, key)
  _, _ = mac.Write(message[:])
  sum := mac.Sum(nil)
  offset := sum[len(sum)-1] & 0x0f
  binaryCode := (uint32(sum[offset])&0x7f)<<24 |
    uint32(sum[offset+1])<<16 |
    uint32(sum[offset+2])<<8 |
    uint32(sum[offset+3])
  fmt.Printf("%06d", binaryCode%1000000)
}

func recoveryCount(path string) {
  payload := load(path)
  values, ok := payload["recovery_codes"].([]any)
  if !ok {
    os.Exit(1)
  }
  fmt.Print(len(values))
}

func main() {
  if len(os.Args) < 3 {
    os.Exit(2)
  }
  switch os.Args[1] {
  case "field":
    if len(os.Args) != 4 {
      os.Exit(2)
    }
    field(os.Args[2], os.Args[3])
  case "totp":
    if len(os.Args) != 3 {
      os.Exit(2)
    }
    totpFromLogin(os.Args[2])
  case "recovery-count":
    if len(os.Args) != 3 {
      os.Exit(2)
    }
    recoveryCount(os.Args[2])
  default:
    os.Exit(2)
  }
}
EOF_GO
go build -buildvcs=false -trimpath -o "$helper_bin" "$workdir/auth-smoke-helper.go"

ledger_token="smoke-ledger-$(openssl rand -hex 12)"
printf '%s' "$ledger_token" >"$ledger_token_file"
printf '%s' "$(openssl rand -hex 16)" >"$auth_key_file"
admin_password="$(openssl rand -hex 18)"
printf '%s' "$admin_password" >"$admin_password_file"
chmod 0600 "$ledger_token_file" "$auth_key_file" "$admin_password_file"

cat >"$workdir/fake-ledger.go" <<'EOF_GO'
package main

import (
  "encoding/json"
  "net/http"
  "os"
  "strings"
)

func main() {
  tokenBytes, err := os.ReadFile(os.Getenv("SMOKE_LEDGER_TOKEN_FILE"))
  if err != nil {
    panic(err)
  }
  wantAuth := "Bearer " + strings.TrimSpace(string(tokenBytes))
  handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Header.Get("Authorization") != wantAuth || r.Header.Get("X-Timezone-Name") != "Asia/Shanghai" {
      http.Error(w, "unauthorized", http.StatusUnauthorized)
      return
    }
    w.Header().Set("Content-Type", "application/json")
    switch r.URL.Path {
    case "/api/v1/accounts/list.json":
      _ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []any{}})
    case "/api/v1/transactions/list.json":
      _ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{"items": []any{}, "nextTimeSequenceId": 0, "totalCount": 0}})
    default:
      http.NotFound(w, r)
    }
  })
  if err := http.ListenAndServe("127.0.0.1:18081", handler); err != nil {
    panic(err)
  }
}
EOF_GO
SMOKE_LEDGER_TOKEN_FILE="$ledger_token_file" go run "$workdir/fake-ledger.go" >"$workdir/ledger.log" 2>&1 &
ledger_pid=$!

ledger_ready=0
for _ in $(seq 1 30); do
  if curl --silent --output /dev/null --connect-timeout 1 --max-time 2 \
    --header "Authorization: Bearer $ledger_token" \
    --header 'X-Timezone-Name: Asia/Shanghai' \
    http://127.0.0.1:18081/api/v1/accounts/list.json; then
    ledger_ready=1
    break
  fi
  sleep 1
done
[[ "$ledger_ready" == "1" ]] || fail "fake ledger did not become ready"

export DB_HOST="$TEST_POSTGRES_HOST"
export DB_PORT="$TEST_POSTGRES_PORT"
export DB_NAME="$TEST_POSTGRES_DB"
export DB_USER="$TEST_POSTGRES_USER"
export DB_PASSWORD="$TEST_POSTGRES_PASSWORD"
export DB_SSLMODE=disable
export APP_TIMEZONE=Asia/Shanghai
export EBK_BASE_URL=http://127.0.0.1:18081/api/v1
export EBK_API_TOKEN_FILE="$ledger_token_file"
export FINANCE_AUTH_KEY_FILE="$auth_key_file"
export FINANCE_ADMIN_PASSWORD_FILE="$admin_password_file"
export FINANCE_ADMIN_USERNAME="auth-smoke-$(date +%s)-$$"
export FINANCE_LISTEN_ADDR=127.0.0.1:18080
export FINANCE_HEALTHCHECK_URL=http://127.0.0.1:18080/healthz
export MCP_ENABLED=false
unset LLM_BASE_URL LLM_API_KEY LLM_FAST_MODEL LLM_PLANNER_MODEL LLM_REVIEWER_MODEL FINANCE_TRUSTED_PROXY_CIDR

period="$(TZ=Asia/Shanghai date +%Y-%m)"
"$finance_bin" bootstrap \
  --name "auth-smoke-$$" \
  --currency CNY \
  --timezone Asia/Shanghai \
  --period "$period" \
  --liquidity-floor-minor 0 >"$workdir/bootstrap.out"
household_id="$(sed -n 's/.*household_id=\([0-9][0-9]*\).*/\1/p' "$workdir/bootstrap.out" | tail -n 1)"
[[ "$household_id" =~ ^[1-9][0-9]*$ ]] || fail "bootstrap did not return a household id"

"$finance_bin" serve >"$workdir/finance.log" 2>&1 &
finance_pid=$!
finance_ready=0
for _ in $(seq 1 30); do
  if curl --silent --show-error --fail --connect-timeout 1 --max-time 2 http://127.0.0.1:18080/healthz >/dev/null 2>&1; then
    finance_ready=1
    break
  fi
  if ! kill -0 "$finance_pid" >/dev/null 2>&1; then
    fail "Finance Core exited before readiness"
  fi
  sleep 1
done
[[ "$finance_ready" == "1" ]] || fail "Finance Core did not become ready"

base_url=http://127.0.0.1:18080
status_unauth="$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/dashboard?period=$period")"
[[ "$status_unauth" == "401" ]] || fail "unauthenticated dashboard status=$status_unauth"

cat >"$workdir/login.json" <<EOF_JSON
{"username":"$FINANCE_ADMIN_USERNAME","password":"$admin_password"}
EOF_JSON
chmod 0600 "$workdir/login.json"
status_login="$(curl --silent --show-error \
  --header 'Content-Type: application/json' \
  --data-binary @"$workdir/login.json" \
  --output "$workdir/login-response.json" \
  --write-out '%{http_code}' \
  "$base_url/api/v1/auth/login")"
[[ "$status_login" == "200" ]] || fail "password login status=$status_login"
step="$("$helper_bin" field "$workdir/login-response.json" step)"
[[ "$step" == "enroll_totp" ]] || fail "first login did not require TOTP enrollment"
challenge="$("$helper_bin" field "$workdir/login-response.json" challenge)"

status_pre_2fa="$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/dashboard?period=$period")"
[[ "$status_pre_2fa" == "401" ]] || fail "pre-TOTP dashboard status=$status_pre_2fa"

totp_code="$("$helper_bin" totp "$workdir/login-response.json")"
cat >"$workdir/enroll.json" <<EOF_JSON
{"challenge":"$challenge","code":"$totp_code"}
EOF_JSON
chmod 0600 "$workdir/enroll.json"
status_enroll="$(curl --silent --show-error \
  --header 'Content-Type: application/json' \
  --data-binary @"$workdir/enroll.json" \
  --dump-header "$workdir/enroll.headers" \
  --output "$workdir/enroll-response.json" \
  --write-out '%{http_code}' \
  "$base_url/api/v1/auth/totp/enroll/confirm")"
[[ "$status_enroll" == "200" ]] || fail "TOTP enrollment status=$status_enroll"
recovery_count="$("$helper_bin" recovery-count "$workdir/enroll-response.json")"
[[ "$recovery_count" == "10" ]] || fail "TOTP enrollment did not issue 10 recovery codes"
csrf_token="$("$helper_bin" field "$workdir/enroll-response.json" csrf_token)"
session_token="$(sed -n 's/^Set-Cookie: __Host-finance_session=\([^;]*\).*/\1/p' "$workdir/enroll.headers" | tr -d '\r' | tail -n 1)"
[[ -n "$session_token" ]] || fail "TOTP enrollment did not issue the Finance session cookie"
session_sha256="$(printf '%s' "$session_token" | sha256sum | awk '{print $1}')"

cat >"$workdir/session.curl" <<EOF_CURL
header = "Cookie: __Host-finance_session=$session_token"
EOF_CURL
cat >"$workdir/session-csrf.curl" <<EOF_CURL
header = "Cookie: __Host-finance_session=$session_token"
header = "X-CSRF-Token: $csrf_token"
EOF_CURL
chmod 0600 "$workdir/session.curl" "$workdir/session-csrf.curl"

status_dashboard="$(curl --config "$workdir/session.curl" --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/dashboard?period=$period")"
[[ "$status_dashboard" == "200" ]] || fail "authenticated dashboard status=$status_dashboard"

status_override="$(curl --config "$workdir/session.curl" --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/dashboard?period=$period&household_id=$((household_id + 1))")"
[[ "$status_override" == "400" ]] || fail "household override status=$status_override"

cat >"$workdir/advisor.json" <<'EOF_JSON'
{"question":"auth smoke"}
EOF_JSON
status_bad_csrf="$(curl --config "$workdir/session.curl" --silent --show-error \
  --header 'Content-Type: application/json' \
  --header 'X-CSRF-Token: invalid' \
  --data-binary @"$workdir/advisor.json" \
  --output /dev/null --write-out '%{http_code}' \
  "$base_url/api/v1/advisor")"
[[ "$status_bad_csrf" == "403" ]] || fail "invalid CSRF status=$status_bad_csrf"

status_logout="$(curl --config "$workdir/session-csrf.curl" --silent --show-error \
  --request POST --output /dev/null --write-out '%{http_code}' \
  "$base_url/api/v1/auth/logout")"
[[ "$status_logout" == "204" ]] || fail "logout status=$status_logout"

status_revoked="$(curl --config "$workdir/session.curl" --silent --output /dev/null --write-out '%{http_code}' "$base_url/api/v1/dashboard?period=$period")"
[[ "$status_revoked" == "401" ]] || fail "revoked session dashboard status=$status_revoked"

printf 'finance_auth_smoke unauth=%s password_only=%s enroll=%s dashboard=%s invalid_csrf=%s household_override=%s logout=%s revoked=%s\n' \
  "$status_unauth" "$status_pre_2fa" "$status_enroll" "$status_dashboard" "$status_bad_csrf" "$status_override" "$status_logout" "$status_revoked"
printf 'finance_auth_smoke session_sha256=%s recovery_count=%s\n' "$session_sha256" "$recovery_count"
printf 'finance_auth_smoke=PASS\n'
