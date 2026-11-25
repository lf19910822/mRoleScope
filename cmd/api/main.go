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

	"github.com/jackc/pgx/v5/pgxpool"

	"mrolescope/internal/access"
	"mrolescope/internal/aitrend"
	"mrolescope/internal/config"
	"mrolescope/internal/httpserver"
	"mrolescope/internal/jdserving"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	if err := cfg.ValidateAccess(); err != nil {
		slog.Error("invalid access configuration", "error", err)
		os.Exit(1)
	}

	appContext, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(appContext, cfg.DatabaseURL)
	if err != nil {
		slog.Error("create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(appContext); err != nil {
		slog.Error("database is not ready", "error", err)
		os.Exit(1)
	}

	accessService, err := access.New(access.Config{
		InviteBcryptHash:     cfg.InviteBcryptHash,
		SessionSigningSecret: cfg.SessionSigningSecret,
	})
	if err != nil {
		slog.Error("invalid access configuration", "error", err)
		os.Exit(1)
	}

	repository := jdserving.NewRepositoryFromPool(pool)
	var trendRunner httpserver.AITrendStreamRunner
	var debugRunner httpserver.AITrendDebugRunner
	if cfg.TrendAIAPIKey != "" && cfg.TrendAIWorkspaceID != "" {
		provider, providerErr := aitrend.NewQwenIntentProvider(cfg.TrendAIAPIKey, cfg.TrendAIWorkspaceID, &http.Client{Timeout: 75 * time.Second})
		if providerErr != nil {
			slog.Warn("trend AI disabled", "reason", "provider configuration is invalid")
		} else {
			trendRunner, providerErr = aitrend.NewTrendRunner(provider, repository, repository, repository)
			if providerErr != nil {
				slog.Warn("trend AI disabled", "reason", "trend runner configuration is invalid")
			} else if concreteRunner, ok := trendRunner.(*aitrend.TrendRunner); ok {
				concreteRunner.Audit = aitrend.NewAuditStore(pool)
				concreteRunner.Langfuse = aitrend.LangfuseDebugConfig{BaseURL: cfg.LangfuseBaseURL, ProjectID: cfg.LangfuseProjectID, PublicKey: cfg.LangfusePublicKey, SecretKey: cfg.LangfuseSecretKey, FlushTimeout: 20 * time.Second, MaxBytes: cfg.AIDebugMaxBytes}
				debugRunner = concreteRunner
			}
		}
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpserver.NewWithAccessAndAIAndDebug(pool, accessService, trendRunner, debugRunner, httpserver.AIDebugConfig{Enabled: cfg.AIDebugEnabled, MaxEvents: cfg.AIDebugMaxEvents, MaxBytes: cfg.AIDebugMaxBytes, Timeout: cfg.AIDebugTimeout}, repository),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("API listening", "address", cfg.HTTPAddress, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("API server stopped", "error", err)
			cancel()
		}
	}()

	<-appContext.Done()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
