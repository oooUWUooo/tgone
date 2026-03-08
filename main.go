package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"habr-rss-bot/internal/bot"
	"habr-rss-bot/internal/config"
	"habr-rss-bot/internal/feed"
	"habr-rss-bot/internal/server"
)

// docs/ is embedded directly into the binary — no external files needed at runtime.
//
//go:embed docs
var docsFS embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()

	staticFS, err := fs.Sub(docsFS, "docs")
	if err != nil {
		logger.Error("Failed to prepare embedded FS", "error", err)
		os.Exit(1)
	}

	fetcher := feed.New(cfg.MaxArticles)
	srv := server.New(cfg, fetcher, staticFS, logger)

	// ctx is cancelled on SIGINT / SIGTERM → triggers graceful shutdown everywhere.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if !cfg.WebOnly {
		b, err := bot.New(cfg, fetcher, logger)
		if err != nil {
			logger.Error("Failed to start Telegram bot", "error", err)
			os.Exit(1)
		}
		go func() {
			if err := b.Run(ctx); err != nil {
				logger.Error("Bot stopped unexpectedly", "error", err)
				cancel() // propagate failure → shut down the web server too
			}
		}()
	} else {
		logger.Info("Web-only mode active (set TELEGRAM_BOT_TOKEN to enable the Telegram bot)")
	}

	if err := srv.Run(ctx); err != nil {
		logger.Error("HTTP server error", "error", err)
		os.Exit(1)
	}

	logger.Info("Shutdown complete")
}
