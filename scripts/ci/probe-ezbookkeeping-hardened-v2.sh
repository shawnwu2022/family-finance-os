#!/usr/bin/env bash
set -euo pipefail

TRIVY_IMAGE='ghcr.io/aquasecurity/trivy:0.73.0@sha256:7cced7cae583819fc7806d4cbc0dbbc7cad18b99f7d3e235192e6da8c091045c'
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

cat >"$workdir/Dockerfile" <<'EOF'
FROM alpine:3.24 AS source
RUN apk add --no-cache git
WORKDIR /src
RUN git init \
 && git remote add origin https://github.com/mayswind/ezbookkeeping.git \
 && git fetch --depth=1 origin 6ccd0c462100828c78e203792a5b2feb8d569039 \
 && git checkout --detach FETCH_HEAD \
 && test "$(git rev-parse HEAD)" = "6ccd0c462100828c78e203792a5b2feb8d569039"

FROM golang:1.26.6-alpine AS be-builder
WORKDIR /src
COPY --from=source /src /src
RUN apk add --no-cache git gcc g++ libc-dev jq
RUN git fetch --depth=2 origin 8b1b2a02d59a5901b7eac6491df0c2a82127bc43 \
 && git diff '8b1b2a02d59a5901b7eac6491df0c2a82127bc43^' 8b1b2a02d59a5901b7eac6491df0c2a82127bc43 -- go.mod go.sum > /tmp/dependency-upgrade.patch \
 && test -s /tmp/dependency-upgrade.patch \
 && git apply --check /tmp/dependency-upgrade.patch \
 && git apply /tmp/dependency-upgrade.patch \
 && grep -Fq 'github.com/xuri/excelize/v2 v2.11.0' go.mod \
 && grep -Fq 'golang.org/x/text v0.40.0' go.mod
RUN docker/backend-build-pre-setup.sh
RUN RELEASE_BUILD=1 ./build.sh backend --release --no-lint --no-test
RUN set +e; \
    go test -json ./... -count=1 > /tmp/go-test.json 2>&1; rc=$?; \
    set -e; \
    if [ "$rc" -ne 0 ]; then \
      echo 'EZBOOKKEEPING_TEST_STATUS=FAIL'; \
      echo '=== FAILING PACKAGES ==='; \
      jq -r 'select(.Action == "fail" and .Package != null) | .Package' /tmp/go-test.json | sort -u; \
      echo '=== FAILING TESTS ==='; \
      jq -r 'select(.Action == "fail" and .Package != null and .Test != null) | (.Package + "::" + .Test)' /tmp/go-test.json | sort -u; \
      echo '=== LAST TEST OUTPUT ==='; \
      jq -r 'select(.Output != null) | .Output' /tmp/go-test.json | tail -n 180; \
      exit "$rc"; \
    fi; \
    echo 'EZBOOKKEEPING_TEST_STATUS=PASS'

FROM node:24.19.0-bookworm-slim AS fe-builder
WORKDIR /src
COPY --from=source /src /src
RUN apt-get update \
 && apt-get install -y --no-install-recommends git \
 && rm -rf /var/lib/apt/lists/*
RUN docker/frontend-build-pre-setup.sh
RUN RELEASE_BUILD=1 ./build.sh frontend --release --no-lint --no-test

FROM alpine:3.24
RUN apk upgrade --no-cache \
 && addgroup -S -g 1000 ezbookkeeping \
 && adduser -S -G ezbookkeeping -u 1000 ezbookkeeping \
 && apk --no-cache add tzdata
COPY --from=source /src/docker/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh \
 && mkdir -p /ezbookkeeping/data /ezbookkeeping/log /ezbookkeeping/storage \
 && chown -R 1000:1000 /ezbookkeeping
WORKDIR /ezbookkeeping
COPY --from=be-builder --chown=1000:1000 /src/ezbookkeeping /ezbookkeeping/ezbookkeeping
COPY --from=fe-builder --chown=1000:1000 /src/dist /ezbookkeeping/public
COPY --from=source --chown=1000:1000 /src/conf /ezbookkeeping/conf
COPY --from=source --chown=1000:1000 /src/templates /ezbookkeeping/templates
COPY --from=source --chown=1000:1000 /src/LICENSE /ezbookkeeping/LICENSE
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
EOF

docker build -f "$workdir/Dockerfile" -t family-finance/ezbookkeeping-hardened:probe "$workdir"
docker run --rm --entrypoint /ezbookkeeping/ezbookkeeping family-finance/ezbookkeeping-hardened:probe --version

docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  "$TRIVY_IMAGE" image --quiet --format json --scanners vuln \
  --severity HIGH,CRITICAL family-finance/ezbookkeeping-hardened:probe >"$workdir/scan.json"

total="$(jq '[.Results[]?.Vulnerabilities[]?] | length' "$workdir/scan.json")"
fixable="$(jq '[.Results[]?.Vulnerabilities[]? | select((.FixedVersion // "") != "")] | length' "$workdir/scan.json")"
echo 'COMPONENT=ezbookkeeping'
echo "SCAN_TOTAL=$total"
echo "SCAN_FIXABLE=$fixable"

if (( fixable > 0 )); then
  jq -r '.Results[]?.Vulnerabilities[]? | select((.FixedVersion // "") != "") | [.VulnerabilityID,.PkgName,.InstalledVersion,.FixedVersion,.Severity] | @tsv' "$workdir/scan.json" | head -n 80
  exit 1
fi
