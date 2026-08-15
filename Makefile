.PHONY: test test-race fmt fmt-check vet build config preflight up down logs backup

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
	./scripts/preflight.sh

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

backup:
	./scripts/backup.sh
