.PHONY: test test-race fmt fmt-check vet build config preflight up down logs backup verify verify-contract verify-go verify-web verify-mcp-security verify-edge-security verify-container

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f)

fmt-check:
	@test -z "$$(gofmt -l $$(find cmd internal -name '*.go' -type f))" || (echo 'gofmt required' && gofmt -d $$(gofmt -l $$(find cmd internal -name '*.go' -type f)) && exit 1)

vet:
	go vet ./...

build:
	mkdir -p build
	CGO_ENABLED=0 go build -trimpath -o build/finance-core ./cmd/finance-core

config:
	docker compose config

preflight:
	bash scripts/preflight.sh

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

backup:
	bash scripts/backup.sh

verify:
	bash scripts/ci/verify.sh

verify-contract:
	bash scripts/ci/contract-test.sh

verify-go:
	bash scripts/ci/go-stack-verify.sh

verify-web:
	@set -eu; \
	project="family-finance-ci-web-$$(id -u)-$$$$"; \
	cleanup() { docker compose -p "$$project" -f compose.ci.yaml down -v --remove-orphans >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT INT TERM; \
	docker compose -p "$$project" -f compose.ci.yaml run --rm --no-deps web bash /src/scripts/ci/web-verify.sh

verify-mcp-security:
	@set -eu; \
	project="family-finance-ci-mcp-$$(id -u)-$$$$"; \
	cleanup() { docker compose -p "$$project" -f compose.ci.yaml down -v --remove-orphans >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT INT TERM; \
	docker compose -p "$$project" -f compose.ci.yaml build go; \
	docker compose -p "$$project" -f compose.ci.yaml run --rm --no-deps go bash /src/scripts/ci/mcp-security.sh

verify-edge-security:
	bash scripts/ci/edge-security.sh

verify-container:
	bash scripts/ci/container-verify.sh
