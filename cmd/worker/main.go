package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourname/jobscheduler/internal/config"
	"github.com/yourname/jobscheduler/internal/queue"
	mongorepo "github.com/yourname/jobscheduler/internal/repository/mongo"
	"github.com/yourname/jobscheduler/internal/worker"
	"github.com/yourname/jobscheduler/internal/worker/handlers"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()

	// ── MongoDB ───────────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := mongorepo.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		slog.Error("mongo connect failed", "err", err)
		os.Exit(1)
	}
	slog.Info("mongodb connected", "worker_id", cfg.WorkerID)

	// ── NATS ──────────────────────────────────────────────────────────────────
	natsClient, err := queue.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("nats connect failed", "err", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	// ── Consumer ──────────────────────────────────────────────────────────────
	// AckWait must be greater than the max job timeout (300s) plus buffer.
	consumer, err := queue.NewConsumer(natsClient.JS, 330*time.Second)
	if err != nil {
		slog.Error("consumer setup failed", "err", err)
		os.Exit(1)
	}

	// ── Repositories ──────────────────────────────────────────────────────────
	jobRepo := mongorepo.NewJobRepo(db)
	runRepo := mongorepo.NewRunRepo(db)

	// ── Handler registry ──────────────────────────────────────────────────────
	registry := handlers.NewRegistry()
	registry.Register("http_webhook", handlers.NewHTTPWebhookHandler())
	// Add more handlers here as the project grows.

	// ── Executor and publisher ────────────────────────────────────────────────
	publisher := queue.NewPublisher(natsClient.JS)
	executor := worker.NewExecutor(registry)

	// ── Worker ────────────────────────────────────────────────────────────────
	w := worker.New(
		cfg.WorkerID,
		consumer,
		publisher,
		jobRepo,
		runRepo,
		executor,
		slog.Default(),
	)

	// Context cancelled on SIGTERM/SIGINT triggers graceful shutdown.
	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("worker starting", "worker_id", cfg.WorkerID)
	w.Run(runCtx)
	slog.Info("worker stopped", "worker_id", cfg.WorkerID)
}
