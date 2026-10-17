// Command api starts the Child Connect server: settings, database, migrations, router and background job.
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
	_ "time/tzdata"

	"childconnect/internal/ai"
	"childconnect/internal/ai/risk"
	"childconnect/internal/audit"
	"childconnect/internal/auth"
	"childconnect/internal/config"
	"childconnect/internal/crypto"
	"childconnect/internal/db"
	"childconnect/internal/handlers"
	"childconnect/internal/jobs"
	"childconnect/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("applying migrations")
	if err := db.Migrate(cfg.DatabaseURLDirect); err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	box, err := crypto.New(cfg.FieldKey)
	if err != nil {
		return err
	}
	store, err := storage.NewLocal(cfg.StorageDir, cfg.JWTSecret, cfg.PublicAPIURL)
	if err != nil {
		return err
	}
	model, err := risk.Load()
	if err != nil {
		return err
	}
	svc := &ai.Service{Pool: pool}
	if cfg.AIEnabled() {
		gem, err := ai.NewGemini(ctx, cfg.GeminiAPIKey, cfg.GeminiModel, cfg.EmbedModel)
		if err != nil {
			slog.Warn("Gemini client could not start; AI features use rules-only fallbacks", "err", err)
		} else {
			svc.Gem = gem
			slog.Info("AI enabled", "model", cfg.GeminiModel, "embed_model", cfg.EmbedModel)
			go jobs.ReembedKnowledge(ctx, pool, svc)
		}
	} else {
		slog.Warn("GEMINI_API_KEY is not set: AI features run in rules-only mode and say so in their output")
	}

	srv := &handlers.Server{
		Pool: pool, Cfg: cfg, Box: box, Audit: audit.New(pool), Store: store, AI: svc, Risk: model,
		Tokens: auth.NewTokens(cfg.JWTSecret, cfg.CookieSecure), Limiter: auth.NewLimiter(),
	}
	(&jobs.Runner{Pool: pool, Risk: model, AI: svc, Every: time.Duration(cfg.JobInterval) * time.Minute}).Start(ctx)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second, // AI calls: 20 s limit, one retry, several steps
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(sctx)
	}()
	slog.Info("Child Connect API listening", "addr", "http://localhost:"+cfg.Port+"/api/v1", "frontend", cfg.FrontendOrigin)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
