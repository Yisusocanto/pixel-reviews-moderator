package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Yisusocanto/pixel-reviews-moderator/internal/apiclient"
	"github.com/Yisusocanto/pixel-reviews-moderator/internal/application"
	"github.com/Yisusocanto/pixel-reviews-moderator/internal/broker"
	"github.com/Yisusocanto/pixel-reviews-moderator/internal/config"
	"github.com/Yisusocanto/pixel-reviews-moderator/internal/judge"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rulesJudge := judge.NewRulesJudge()
	geminiJudge := judge.NewGeminiJudge(cfg.GeminiApiKey, cfg.GeminiApiUrl, cfg.GeminiModel, logger)
	pipeline := judge.NewPipeline(rulesJudge, geminiJudge)

	apiClient := apiclient.NewClient(cfg.ApiBaseUrl, cfg.ModerationSecretKey, logger)
	moderationHandler := application.NewModerationHandler(pipeline, apiClient, logger)

	rabbitClient, err := broker.Connect(ctx, cfg.RabbitMQUrl, logger)
	if err != nil {
		logger.Error("failed to initialize RabbitMQ client", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rabbitClient.Close(closeCtx); err != nil {
			logger.Error("failed to close RabbitMQ client", slog.Any("error", err))
		}
	}()

	consumer := broker.NewConsumer(
		moderationHandler,
		"moderation.review.check",
		rabbitClient.Connection(),
		logger,
	)
	if err := consumer.Run(ctx); err != nil {
		logger.Error("consumer exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}
