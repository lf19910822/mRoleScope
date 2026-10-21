.DEFAULT_GOAL := help

# The development image has legacy docker-compose; newer systems can override
# this with: make COMPOSE='docker compose' <target>
COMPOSE ?= docker-compose
DEPLOY_COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && printf 'docker compose' || printf 'docker-compose')

.PHONY: help fmt test test-go test-python collector-fixture db-up db-down migrate-up api deploy-config-check

help:
	@printf '%s\n' 'Targets: fmt test collector-fixture db-up db-down migrate-up api deploy-config-check'

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f)

test: test-go test-python

test-go:
	go test ./cmd/... ./internal/...

test-python:
	python3 -m unittest collector.test_collector

collector-fixture:
	@task_tmp="$$(mktemp -d)"; \
	python3 collector/mihoyo_collector.py --run-id local-fixture --fixture collector/fixtures/complete_collection.json --output-dir "$$task_tmp/run"; \
	printf '%s\n' "fixture written to $$task_tmp/run"

db-up:
	$(COMPOSE) up -d postgres

db-down:
	$(COMPOSE) down

migrate-up:
	$(COMPOSE) run --rm migrate

api:
	go run ./cmd/api

deploy-config-check:
	MROLESCOPE_API_IMAGE=docker.io/mrolescopeverify/mrolescope-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
	MROLESCOPE_MIGRATE_IMAGE=docker.io/mrolescopeverify/mrolescope-migrate@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
	POSTGRES_DB=mrolescope_verify \
	POSTGRES_USER=mrolescope_verify \
	POSTGRES_PASSWORD=not-a-real-password \
	DATABASE_URL='postgres://mrolescope_verify:not-a-real-password@postgres:5432/mrolescope_verify?sslmode=disable' \
	$(DEPLOY_COMPOSE) -f deploy/remote-development/compose.yaml config --quiet
