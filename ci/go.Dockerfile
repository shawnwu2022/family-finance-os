FROM golang:1.26.6-bookworm

ARG SQLC_VERSION=1.31.1
ARG SQLC_SHA256=497ae4fcdfa64c5b0c311ffe4c2bd991e43991e82e5367792ed78bc2dca27354
ARG GOOSE_VERSION=3.27.3
ARG GOOSE_SHA256=ca18112e2438b3ad608af9a5938beafd01fa36a4a19a3edbe4f29226ca5c8533
ARG GOVULNCHECK_VERSION=v1.7.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git postgresql-client tar \
    && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    curl -fsSL -o /tmp/sqlc.tar.gz \
      "https://github.com/sqlc-dev/sqlc/releases/download/v${SQLC_VERSION}/sqlc_${SQLC_VERSION}_linux_amd64.tar.gz"; \
    echo "${SQLC_SHA256}  /tmp/sqlc.tar.gz" | sha256sum -c -; \
    tar -xzf /tmp/sqlc.tar.gz -C /usr/local/bin sqlc; \
    rm -f /tmp/sqlc.tar.gz; \
    sqlc version

RUN set -eux; \
    curl -fsSL -o /usr/local/bin/goose \
      "https://github.com/pressly/goose/releases/download/v${GOOSE_VERSION}/goose_linux_x86_64"; \
    echo "${GOOSE_SHA256}  /usr/local/bin/goose" | sha256sum -c -; \
    chmod 0755 /usr/local/bin/goose; \
    goose -version

RUN GOBIN=/usr/local/bin go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" \
    && govulncheck -version

WORKDIR /src
