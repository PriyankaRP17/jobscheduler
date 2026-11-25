# Distributed Job Scheduler

A reliable, self-hostable background job scheduler built with Go, NATS JetStream, MongoDB and Next.js.

> **Status:** Week 1 — core pipeline (API + worker + NATS queue). See [docs/](docs/) for the full requirements and design specification.

## Architecture

```
Next.js Dashboard
      │
  API Service (Gin) ◄──► MongoDB
      │                     ▲
      ▼                     │
  NATS JetStream ◄── Scheduler (Week 2)
      │
      ▼
  Workers (N replicas) ──► Target webhooks
```

## Tech Stack

| Layer | Technology |
|---|---|
| API | Go + Gin |
| Queue | NATS JetStream |
| Database | MongoDB 7 |
| Dashboard | Next.js + React (Week 4) |
| Gateway | Kong (Week 5) |
| Metrics | Prometheus + Grafana (Week 5) |

## Prerequisites

- Go 1.22+
- Docker and Docker Compose
- `nats` CLI (optional, for debugging): `brew install nats-io/nats-tools/nats`
- MongoDB Compass (optional, for inspecting data)

## Quick Start

```bash
# 1. Clone and enter
git clone https://github.com/yourname/jobscheduler
cd jobscheduler

# 2. Copy env
cp .env.example .env

# 3. Start infrastructure (Mongo + NATS)
make up

# 4. In terminal 1 — start the API
make run-api

# 5. In terminal 2 — start a worker
make run-worker

# 6. In terminal 3 — create a job and trigger it
make demo-create-job
# copy the returned id, then:
make demo-trigger JOB_ID=<id>
```

## API Reference

### Create a job
```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Daily Sales Report",
    "type": "http_webhook",
    "payload": {
      "url": "https://httpbin.org/post",
      "method": "POST",
      "body": "{\"report\":\"daily\"}"
    },
    "schedule": {
      "kind": "delay",
      "run_at": "2099-01-01T00:00:00Z"
    },
    "policy": {
      "max_retries": 3,
      "timeout_sec": 30
    }
  }'
```

### Trigger a job immediately
```bash
curl -X POST http://localhost:8080/api/v1/jobs/<id>/trigger
```

### Check run status
```bash
curl http://localhost:8080/api/v1/runs/<run_id>
```

### List all runs for a job
```bash
curl http://localhost:8080/api/v1/jobs/<id>/runs
```

### Pause / Resume a job
```bash
curl -X PATCH http://localhost:8080/api/v1/jobs/<id> \
  -H "Content-Type: application/json" \
  -d '{"status": "paused"}'
```

## Running Tests

```bash
# Unit tests
make test

# With coverage
go test ./internal/... ./pkg/... -coverprofile=cover.out
go tool cover -html=cover.out
```

## Run Multiple Workers (load balancing demo)

```bash
# Terminal 2
WORKER_ID=worker-1 make run-worker

# Terminal 3
WORKER_ID=worker-2 make run-worker

# Terminal 4
WORKER_ID=worker-3 make run-worker

# Trigger 20 jobs and watch them spread across workers
for i in $(seq 1 20); do curl -s -X POST http://localhost:8080/api/v1/jobs/<id>/trigger; done
```

## NATS Monitoring

Open http://localhost:8222 in your browser to see:
- Streams and message counts
- Consumer lag
- Server info

Or use the CLI:
```bash
nats stream info JOBS
nats consumer info JOBS workers
```

## Project Structure

```
cmd/           # one main.go per service
internal/
  config/      # env-based config
  domain/      # core types (Job, Run, Worker)
  api/         # Gin handlers, middleware, router
  service/     # business logic
  repository/  # interfaces + MongoDB implementations
  queue/       # NATS client, publisher, consumer
  worker/      # pull loop, executor, handlers
pkg/
  backoff/     # exponential backoff with jitter
deploy/        # Docker Compose, Dockerfile
docs/          # requirements, design, ADRs
test/          # integration, chaos, load tests
web/           # Next.js dashboard (Week 4)
```

## Weekly Progress

- [x] Week 1: API, MongoDB, NATS queue, worker, graceful shutdown
- [ ] Week 2: Scheduler, cron/delay, retries, dead-letter
- [ ] Week 3: Heartbeats, reaper, crash recovery, idempotency
- [ ] Week 4: Next.js dashboard, SSE live updates, auth
- [ ] Week 5: Metrics, chaos tests, k6 load tests, Kong, CI gate
- [ ] Week 6: Deploy, demo, README polish

## Key Design Decisions

See [docs/decisions.md](docs/decisions.md) for the full ADRs.

- **At-least-once delivery** — workers ack only after saving the result to MongoDB (NFR-01)
- **No duplicate scheduling** — atomic MongoDB claim with compare-and-swap (NFR-02)
- **SSRF protection** — private/loopback IPs blocked at validation and at execution time (NFR-08)
- **Small queue messages** — workers load job details from MongoDB, not from the queue (ADR-03)
- **Pluggable handlers** — add a new job type by implementing one interface and one registry line
