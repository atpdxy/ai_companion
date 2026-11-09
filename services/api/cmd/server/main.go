package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"companion/api/internal/handler"
	"companion/api/internal/infrastructure"
	"companion/api/internal/service"
	"github.com/redis/go-redis/v9"
)

func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		slog.Error("missing_config", "key", key)
		os.Exit(1)
	}
	return value
}
func fallback(key, value string) string {
	if configured := os.Getenv(key); configured != "" {
		return configured
	}
	return value
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With("service", "api"))
	secret, token := required("JWT_SECRET"), required("SERVICE_TOKEN")
	if len(secret) < 32 || len(token) < 32 || secret == token {
		slog.Error("invalid_secret_configuration")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	store, err := infrastructure.NewPostgres(startup, required("DATABASE_URL"))
	if err != nil {
		slog.Error("database_connect_failed")
		os.Exit(1)
	}
	defer store.Pool.Close()
	if err = store.Migrate(startup); err != nil {
		slog.Error("migration_failed", "error", err)
		os.Exit(1)
	}
	options, err := redis.ParseURL(required("REDIS_URL"))
	if err != nil {
		slog.Error("invalid_redis_url")
		os.Exit(1)
	}
	cache := redis.NewClient(options)
	defer func() {
		if closeErr := cache.Close(); closeErr != nil {
			slog.Warn("redis_close_failed")
		}
	}()
	if err = cache.Ping(startup).Err(); err != nil {
		slog.Error("redis_connect_failed")
		os.Exit(1)
	}
	auth := &service.Auth{Store: store, Secret: []byte(secret)}
	agent := infrastructure.NewAgentClient(fallback("AI_URL", "http://127.0.0.1:8000"), token)
	origin := fallback("WEB_ORIGIN", "http://localhost:5173")
	mode := fallback("LLM_MODE", "demo")
	api := &handler.API{Tools: &service.Tools{Store: store, Gateway: agent}, Voice: &service.Voice{Store: store, Gateway: agent}, ASREnabled: os.Getenv("ASR_ENABLED") == "true", TTSEnabled: os.Getenv("TTS_ENABLED") == "true", Mode: mode, Store: store, Auth: auth, Chat: &service.Chat{Store: store, Agent: agent}, Limiter: &infrastructure.RedisLimiter{Client: cache}, Origin: origin, SecureCookies: strings.HasPrefix(origin, "https://"), Ready: func(ctx context.Context) error {
		if err := store.Pool.Ping(ctx); err != nil {
			return err
		}
		return cache.Ping(ctx).Err()
	}}
	server := &http.Server{Addr: fallback("API_ADDR", "127.0.0.1:8080"), Handler: api.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	done := make(chan struct{})
	schedulerDone := make(chan struct{})
	go func() {
		defer close(schedulerDone)
		store.RunScheduler(ctx, func(error) { slog.Error("scheduler_failed") })
	}()
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Error("shutdown_failed")
		}
	}()
	slog.Info("server_started", "address", server.Addr)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server_failed")
		stop()
	}
	stop()
	<-done
	<-schedulerDone
}
