.PHONY: up down run-api run-worker run-scheduler run-reaper \
        test test-unit test-integration lint fmt vet tidy \
        build-api build-worker build-all tag

up:
	docker compose -f deploy/docker-compose.yml up -d
	@echo "Mongo  → localhost:27017"
	@echo "NATS   → localhost:4222  (monitoring: http://localhost:8222)"

down:
	docker compose -f deploy/docker-compose.yml down

down-v:
	docker compose -f deploy/docker-compose.yml down -v

run-api:
	go run ./cmd/api

run-worker:
	WORKER_ID=worker-local go run ./cmd/worker

run-scheduler:
	go run ./cmd/scheduler

run-reaper:
	go run ./cmd/reaper

test: test-unit

test-unit:
	go test ./internal/... ./pkg/... -v -count=1 -timeout 60s

test-integration:
	go test ./test/integration/... -v -count=1 -timeout 120s -tags integration

test-chaos:
	go test ./test/chaos/... -v -count=1 -timeout 300s -tags chaos

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

build-api:
	go build -o bin/api ./cmd/api

build-worker:
	go build -o bin/worker ./cmd/worker

build-all:
	go build -o bin/api       ./cmd/api
	go build -o bin/worker    ./cmd/worker
	go build -o bin/scheduler ./cmd/scheduler
	go build -o bin/reaper    ./cmd/reaper

tag:
	@read -p "Version (e.g. v0.1.0): " v; \
	git tag -a $$v -m "Release $$v" && git push origin $$v

demo-create-job:
	curl -s -X POST http://localhost:8080/api/v1/jobs \
	  -H "Content-Type: application/json" \
	  -d '{ \
	    "name": "Test Webhook", \
	    "type": "http_webhook", \
	    "payload": {"url": "https://httpbin.org/post", "method": "POST"}, \
	    "schedule": {"kind": "delay", "run_at": "2099-01-01T00:00:00Z"}, \
	    "policy": {"max_retries": 2, "timeout_sec": 10} \
	  }' | jq .

demo-trigger:
	@echo "Usage: make demo-trigger JOB_ID=<id>"
	curl -s -X POST http://localhost:8080/api/v1/jobs/$(JOB_ID)/trigger | jq .
