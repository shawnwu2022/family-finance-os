.PHONY: test config preflight up down logs backup

test:
	cd apps/finance-core && PYTHONPATH=src pytest -q

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
