#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "runtime image hardening contract failed: $*" >&2
  exit 1
}

required=(
  .dockerignore
  Dockerfile.postgres
  Dockerfile.caddy
  Dockerfile.ezbookkeeping
  infra/caddy/main.go.tmpl
  infra/caddy/docker-entrypoint.sh
  infra/ezbookkeeping/patches/frontend-security.patch
  scripts/ci/runtime-images-security.sh
  .github/workflows/runtime-images-security.yml
)
for path in "${required[@]}"; do
  [[ -f "$path" ]] || fail "missing hardened runtime artifact: $path"
done

bash -n scripts/ci/runtime-images-security.sh || fail "runtime image verifier has invalid shell syntax"
sh -n infra/caddy/docker-entrypoint.sh || fail "Caddy entrypoint has invalid shell syntax"

# The repository root is the Docker build context. Keep local credentials, finance data,
# backups and VCS metadata out of the context before anything is sent to the daemon.
for ignored in '.git' '.env' '.env.*' 'secrets' '*.pem' '*.key' '*.sqlite' '*.db' 'backups'; do
  grep -Fxq "$ignored" .dockerignore \
    || fail ".dockerignore must exclude $ignored from the production build context"
done

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

# Preserve the exact PostgreSQL hardening mechanics.
grep -Fq 'apk add --no-cache su-exec' Dockerfile.postgres \
  || fail "PostgreSQL hardening must install su-exec"
grep -Fq "sed -i 's/exec gosu postgres/exec su-exec postgres/g'" Dockerfile.postgres \
  || fail "PostgreSQL entrypoint must replace gosu with su-exec"
grep -Fq 'rm -f /usr/local/bin/gosu' Dockerfile.postgres \
  || fail "PostgreSQL hardened image must remove gosu"

# Caddy is a separate build module. Store the custom main as a non-Go template so the
# repository's root `go mod tidy` never absorbs Caddy's large transitive dependency graph.
if find infra/caddy -maxdepth 1 -type f -name '*.go' -print -quit | grep -q .; then
  fail "Caddy custom build sources must not participate in the root Go module"
fi
grep -Fq 'COPY infra/caddy/main.go.tmpl ./main.go' Dockerfile.caddy \
  || fail "Caddy must materialize the repository custom main only inside the Docker build"
grep -Fq 'github.com/caddyserver/caddy/v2/modules/standard' infra/caddy/main.go.tmpl \
  || fail "Caddy custom main must include the standard module set"
grep -Fq 'ENV GOPROXY=https://proxy.golang.org' Dockerfile.caddy \
  || fail "Caddy module downloads must use the Go module proxy without direct fallback"
grep -Fq 'ENV GOSUMDB=sum.golang.org' Dockerfile.caddy \
  || fail "Caddy module downloads must use the Go checksum database"
grep -Fq 'github.com/caddyserver/caddy/v2@v2.11.4' Dockerfile.caddy \
  || fail "Caddy v2.11.4 module version pin is missing"
grep -Fq 'h1:XKxkMTgNSizEvKG6QHue6cAsFOteU2qA61w2tKkCWi0=' Dockerfile.caddy \
  || fail "Caddy v2.11.4 module zip checksum pin is missing"
grep -Fq 'h1:zXCl032uTaF5/TpgU38axqFD41jqzxomTDNqK7BzMeI=' Dockerfile.caddy \
  || fail "Caddy v2.11.4 go.mod checksum pin is missing"
grep -Fq "go list -m -f '{{.Version}}' github.com/caddyserver/caddy/v2" Dockerfile.caddy \
  || fail "Caddy build must assert the resolved release version"
grep -Fq 'golang.org/x/crypto@v0.54.0' Dockerfile.caddy \
  || fail "Caddy crypto dependency hardening is missing"
grep -Fq 'golang.org/x/net@v0.57.0' Dockerfile.caddy \
  || fail "Caddy network dependency hardening is missing"
grep -Fq 'golang.org/x/text@v0.40.0' Dockerfile.caddy \
  || fail "Caddy text dependency hardening is missing"
grep -Fq 'google.golang.org/grpc@v1.82.1' Dockerfile.caddy \
  || fail "Caddy gRPC dependency hardening is missing"
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
grep -Fq 'ENV GOFLAGS=-mod=readonly' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping backend dependency graph must be module-readonly"
grep -Fq 'RUN go mod download' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping backend must download the locked module graph before building"
grep -Fq 'CGO_ENABLED=1 go build -a -trimpath' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping backend must use the release-equivalent direct Go build"
grep -Fq -- '-X main.Version=1.6.1 -X main.CommitHash=6ccd0c4' Dockerfile.ezbookkeeping \
  || fail "ezBookkeeping backend build metadata must remain anchored to v1.6.1"
if grep -Fq './build.sh backend' Dockerfile.ezbookkeeping || grep -Fq 'go get .' Dockerfile.ezbookkeeping; then
  fail "ezBookkeeping backend build must not mutate the locked module graph"
fi
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
# upstream runtime images directly.
for dockerfile in Dockerfile.postgres Dockerfile.ezbookkeeping Dockerfile.caddy; do
  grep -Fq "dockerfile: $dockerfile" compose.yaml \
    || fail "compose.yaml must build $dockerfile"
done
if grep -Eq '^[[:space:]]*image:[[:space:]]*(postgres:18\.6|mayswind/ezbookkeeping:1\.6\.1|caddy:2\.11\.4-alpine)' compose.yaml; then
  fail "compose.yaml still references an upstream runtime image directly"
fi

# Public/third-party runtime image CVE scanning is intentionally not a PR/release gate.
# We retain immutable inputs, dependency audits, full builds and real-container smoke tests.
if grep -Eqi 'trivy|scan hardened runtime images|runtime-images-security\.sh scan' \
    scripts/ci/runtime-images-security.sh .github/workflows/runtime-images-security.yml; then
  fail "public runtime image vulnerability scanning must not be a blocking repository gate"
fi

grep -Fq 'SUFFIX="${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}"' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier phases must share stable image and volume names"
grep -Fq '$ROOT_DIR/Caddyfile:/etc/caddy/Caddyfile:ro' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must validate the repository Caddyfile with the custom binary"
grep -Fq 'Verifying hardened Caddy release identity' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must check Caddy release identity"
grep -Fq "grep -Eq '^v2\\.11\\.4([[:space:]]|$)' <<<\"\$caddy_version\"" scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must require Caddy v2.11.4 build metadata"
grep -Fq 'legacy-root-state' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must exercise legacy root-owned Caddy state migration"
grep -Fq '/^Uid:/ {print $2}' scripts/ci/runtime-images-security.sh \
  || fail "runtime verifier must assert the running Caddy process UID"
for phase in build smoke cleanup; do
  grep -Fq "bash scripts/ci/runtime-images-security.sh ${phase}" .github/workflows/runtime-images-security.yml \
    || fail "runtime security workflow must expose the ${phase} phase"
done
grep -Fq 'if: always()' .github/workflows/runtime-images-security.yml \
  || fail "runtime security cleanup must run even when an earlier phase fails"
for trigger in pull_request push workflow_dispatch; do
  grep -Eq "^[[:space:]]*${trigger}:" .github/workflows/runtime-images-security.yml \
    || fail "runtime security workflow must support ${trigger}"
done
if grep -Eq '^[[:space:]]*schedule:' .github/workflows/runtime-images-security.yml; then
  fail "digest-pinned public runtime verification must not consume scheduled Actions capacity"
fi
if (( $(grep -Fc '      - Caddyfile' .github/workflows/runtime-images-security.yml) < 2 )); then
  fail "runtime security workflow must run when Caddyfile changes on PRs and main pushes"
fi
if (( $(grep -Fc '      - infra/caddy/**' .github/workflows/runtime-images-security.yml) < 2 )); then
  fail "runtime security workflow must run when Caddy implementation changes"
fi

# Do not reintroduce mutable action tags in the workflow.
if grep -Eq 'uses:[[:space:]]+[^#[:space:]]+@(v[0-9]+|main|master)([[:space:]]|$)' .github/workflows/runtime-images-security.yml; then
  fail "runtime security workflow contains a mutable action reference"
fi

echo "Runtime image hardening contract OK"
