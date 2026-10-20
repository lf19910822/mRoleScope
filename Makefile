.DEFAULT_GOAL := help

# The development image has legacy docker-compose; newer systems can override
# this with: make COMPOSE='docker compose' <target>
COMPOSE ?= docker-compose

.PHONY: help fmt test test-go test-python collector-fixture db-up db-down migrate-up api

help:
	@printf '%s\n' 'Targets: fmt test collector-fixture db-up db-down migrate-up api'

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
