package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourname/jobscheduler/internal/api"
	"github.com/yourname/jobscheduler/internal/api/handlers"
	"github.com/yourname/jobscheduler/internal/config"
	"github.com/yourname/jobscheduler/internal/queue"
	mongorepo "github.com/yourname/jobscheduler/internal/repository/mongo"
	"github.com/yourname/jobscheduler/internal/service"
)

func main() {
	// Structured JSON logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()

	// ── MongoDB ──────────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := mongorepo.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		slog.Error("mongo connect failed", "err", err)
		os.Exit(1)
	}
	slog.Info("mongodb connected")

	// ── NATS ─────────────────────────────────────────────────────────────────
	natsClient, err := queue.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("nats connect failed", "err", err)
		os.Exit(1)
	}
	defer natsClient.Close()
	slog.Info("nats connected")

	// ── Repositories ─────────────────────────────────────────────────────────
	jobRepo := mongorepo.NewJobRepo(db)
	runRepo := mongorepo.NewRunRepo(db)
	workerRepo := mongorepo.NewWorkerRepo(db)

	// ── Services ─────────────────────────────────────────────────────────────
	publisher := queue.NewPublisher(natsClient.JS)
	jobSvc := service.NewJobService(jobRepo)
	runSvc := service.NewRunService(jobRepo, runRepo, publisher)

	// ── Handlers ─────────────────────────────────────────────────────────────
	jobHandler := handlers.NewJobHandler(jobSvc)
	runHandler := handlers.NewRunHandler(runSvc)
	workerHandler := handlers.NewWorkerHandler(workerRepo)

	// ── Router ───────────────────────────────────────────────────────────────
	router := api.NewRouter(jobHandler, runHandler, workerHandler)

	// ── HTTP Server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start in a goroutine so we can listen for shutdown signals.
	go func() {
		slog.Info("api server starting", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down api server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "err", err)
	}
	slog.Info("api server stopped")
}
