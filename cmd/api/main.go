package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/cache"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/config"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/events"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/httpapi"
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

	redisCache, err := cache.NewRedis(ctx, cfg)
	if err != nil {
		log.Error("failed to connect redis", "error", err)
		os.Exit(1)
	}
	defer redisCache.Close()

	publisher := events.NewKafkaPublisher(cfg)
	defer publisher.Close()

	api := httpapi.New(cfg, db, redisCache, publisher, log)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("api server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownPeriod)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("api shutdown failed", "error", err)
	}
}
