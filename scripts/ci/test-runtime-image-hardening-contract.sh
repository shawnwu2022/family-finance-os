#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "runtime image hardening contract failed: $*" >&2
  exit 1
}

required=(
  Dockerfile.postgres
  Dockerfile.caddy
  Dockerfile.ezbookkeeping
  infra/caddy/docker-entrypoint.sh
  infra/ezbookkeeping/patches/frontend-security.patch
  scripts/ci/runtime-images-security.sh
  .github/workflows/runtime-images-security.yml
)
for path in "${required[@]}"; do
  [[ -f "$path" ]] || fail "missing hardened runtime artifact: $path"
done

bash -n scripts/ci/runtime-images-security.sh || fail "runtime image security verifier has invalid shell syntax"
sh -n infra/caddy/docker-entrypoint.sh || fail "Caddy entrypoint has invalid shell syntax"

# Every external base image used by the hardened images must be immutable.
grep -Eq '^FROM postgres:18\.6-alpine@sha256:[0-9a-f]{64}$' Dockerfile.postgres \
  || fail "PostgreSQL base image must be pinned by digest"
grep -Eq '^FROM golang:1\.26\.6-alpine@sha256:[0-9a-f]{64} AS build$' Dockerfile.caddy \
  || fail "Caddy Go builder must be pinned by digest"
grep -Eq '^FROM alpine:3\.24@sha256:[0-9a-f]{64}$' Dockerfile.caddy \
  || fail "Caddy runtime Alpine image must be pinned by digest"
grep -Eq '^FROM alpine:3\.24@sha256:[0-9a-f]{64} AS source$' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping source stage must be pinned by digest"
grep -Eq '^FROM golang:1\.26\.6-alpine@sha256:[0-9a-f]{64} AS be-builder$' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping Go builder must be pinned by digest"
grep -Eq '^FROM node:24\.19\.0-bookworm-slim@sha256:[0-9a-f]{64} AS fe-builder$' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping Node builder must be pinned by digest"
grep -Eq '^FROM alpine:3\.24@sha256:[0-9a-f]{64}$' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping runtime Alpine image must be pinned by digest"

# Preserve the exact hardening mechanics that were independently scanned to zero
# HIGH/CRITICAL findings.
grep -Fq 'apk add --no-cache su-exec' Dockerfile.postgres \
  || fail "PostgreSQL hardening must install su-exec"
grep -Fq "sed -i 's/exec gosu postgres/exec su-exec postgres/g'" Dockerfile.postgres \
  || fail "PostgreSQL entrypoint must replace gosu with su-exec"
grep -Fq 'rm -f /usr/local/bin/gosu' Dockerfile.postgres \
  || fail "PostgreSQL hardened image must remove gosu"
grep -Fq 'e2eee6a7fce366321294c9c2a79f3146891dcbdf' Dockerfile.caddy \
  || fail "Caddy v2.11.4 source commit pin is missing"
grep -Fq 'golang.org/x/crypto@v0.54.0' Dockerfile.caddy \
  || fail "Caddy crypto dependency hardening is missing"
grep -Fq 'setcap cap_net_bind_service=+ep /usr/bin/caddy' Dockerfile.caddy \
  || fail "Caddy binary must retain low-port bind capability after privilege drop"
grep -Fq 'COPY infra/caddy/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh' Dockerfile.caddy \
  || fail "Caddy runtime must use the repository migration entrypoint"
grep -Fq 'find "$state_dir" -xdev -exec chown -h 1000:1000 {} +' infra/caddy/docker-entrypoint.sh \
  || fail "Caddy entrypoint must migrate legacy volume ownership without following symlinks or filesystems"
grep -Fq 'exec su-exec 1000:1000 /usr/bin/caddy "$@"' infra/caddy/docker-entrypoint.sh \
  || fail "Caddy entrypoint must drop privileges before starting the server"

# The customized ezBookkeeping build must remain anchored to the exact v1.6.1 source,
# the audited upstream Go dependency update, and the exact frontend patch that passed
# npm audit, 38,456 frontend tests, and a production build.
grep -Fq '6ccd0c462100828c78e203792a5b2feb8d569039' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping v1.6.1 source commit pin is missing"
grep -Fq '8b1b2a02d59a5901b7eac6491df0c2a82127bc43' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping upstream Go dependency patch pin is missing"
grep -Fq 'infra/ezbookkeeping/patches/frontend-security.patch' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping frontend security patch must be applied from the repository"
expected_patch_sha='7a4be217daeec79591c08ea08bc1d733ff5538157ceb87ee58bf86f941ac5444'
actual_patch_sha="$(sha256sum infra/ezbookkeeping/patches/frontend-security.patch | awk '{print $1}')"
[[ "$actual_patch_sha" == "$expected_patch_sha" ]] \
  || fail "ezBookkeeping frontend security patch differs from the independently verified patch"
if grep -E '^diff --git a/' infra/ezbookkeeping/patches/frontend-security.patch \
    | grep -Ev '^diff --git a/package(-lock)?\.json b/package(-lock)?\.json$' >/dev/null; then
  fail "ezBookkeeping frontend security patch must modify only package.json and package-lock.json"
fi
grep -Fq 'BUILD_PIPELINE=1 CHECK_3RD_API=0 go test' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping backend tests must isolate real third-party exchange-rate APIs"
grep -Fq 'npm ci' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping frontend install must be lockfile-driven"
grep -Fq 'npm audit --audit-level=high' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping frontend build must fail on HIGH/CRITICAL npm findings"
grep -Fq 'npm test' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping frontend unit tests must run in the hardened build"
grep -Fq 'npm run build' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping production frontend build must run in the hardened build"
grep -Fq 'USER 1000:1000' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping runtime must run as a non-root user"

# Compose must build the hardened third-party runtimes locally instead of pulling the
# vulnerable upstream runtime images directly.
for dockerfile in Dockerfile.postgres Dockerfile.ezbookkeeping Dockerfile.caddy; do
  grep -Fq "dockerfile: $dockerfile" compose.yaml \
    || fail "compose.yaml must build $dockerfile"
done
if grep -Eq '^[[:space:]]*image:[[:space:]]*(postgres:18\.6|mayswind/ezbookkeeping:1\.6\.1|caddy:2\.11\.4-alpine)' compose.yaml; then
  fail "compose.yaml still references a vulnerable upstream runtime image directly"
fi

# Runtime scanning is a separate release-boundary gate so pinned images are rescanned
# as vulnerability intelligence changes. The workflow deliberately exposes build,
# smoke, and scan as separate steps so a failed gate is diagnosable without raw logs.
grep -Fq "TRIVY_IMAGE='ghcr.io/aquasecurity/trivy:0.73.0@sha256:7cced7cae583819fc7806d4cbc0dbbc7cad18b99f7d3e235192e6da8c091045c'" scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must pin the Trivy scanner image"
grep -Fq 'SUFFIX="${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}"' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier phases must share stable image and volume names"
grep -Fq -- '--severity HIGH,CRITICAL' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must scan HIGH and CRITICAL vulnerabilities"
grep -Fq '$ROOT_DIR/Caddyfile:/etc/caddy/Caddyfile:ro' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must validate the repository Caddyfile with the custom binary"
grep -Fq 'legacy-root-state' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must exercise legacy root-owned Caddy state migration"
grep -Fq '/^Uid:/ {print $2}' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must assert the running Caddy process UID"
for phase in build smoke scan cleanup; do
  grep -Fq "bash scripts/ci/runtime-images-security.sh ${phase}" .github/workflows/runtime-images-security.yml \
    || fail "runtime security workflow must expose the ${phase} phase"
done
grep -Fq 'if: always()' .github/workflows/runtime-images-security.yml \
  || fail "runtime security cleanup must run even when an earlier phase fails"
for trigger in pull_request push workflow_dispatch schedule; do
  grep -Eq "^[[:space:]]*${trigger}:" .github/workflows/runtime-images-security.yml \
    || fail "runtime security workflow must support ${trigger}"
done
if (( $(grep -Fc '      - Caddyfile' .github/workflows/runtime-images-security.yml) < 2 )); then
  fail "runtime security workflow must run when Caddyfile changes on PRs and main pushes"
fi
if (( $(grep -Fc '      - infra/caddy/**' .github/workflows/runtime-images-security.yml) < 2 )); then
  fail "runtime security workflow must run when the Caddy migration entrypoint changes"
fi

# Do not reintroduce mutable action tags in the new workflow.
if grep -Eq 'uses:[[:space:]]+[^#[:space:]]+@(v[0-9]+|main|master)([[:space:]]|$)' .github/workflows/runtime-images-security.yml; then
  fail "runtime security workflow contains a mutable action reference"
fi

echo "Runtime image hardening contract OK"
