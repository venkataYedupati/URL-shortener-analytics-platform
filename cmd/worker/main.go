package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/analytics"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/config"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/events"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := store.NewPostgres(ctx, cfg)
	if err != nil {
		log.Error("failed to connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.ApplyMigrationFile(ctx, "migrations/001_init.sql"); err != nil {
		log.Error("failed to apply migrations", "error", err)
		os.Exit(1)
	}

	reader := events.NewKafkaConsumer(cfg)
	defer reader.Close()

	consumer := analytics.NewConsumer(reader, db, log)
	if err := consumer.Run(ctx); err != nil {
		log.Error("analytics worker stopped", "error", err)
		os.Exit(1)
	}
}
