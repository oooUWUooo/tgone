package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/time/rate"

	"habr-rss-bot/internal/config"
	"habr-rss-bot/internal/handlers"
	"habr-rss-bot/internal/services"
)

func main() {
	log.Println("🚀 Starting Habr InfoSec RSS Bot v2.0...")

	// Load configuration
	cfg := config.Load()

	// Create RSS service
	rssService := services.NewRSSService(cfg)

	// Start cleanup routine
	done := make(chan struct{})
	go rssService.StartCleanupRoutine(done)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Shutting down gracefully...")
		close(done)
		os.Exit(0)
	}()

	// Initialize Telegram bot if not in web-only mode
	var bot *tgbotapi.BotAPI
	var telegramHandler *handlers.TelegramHandler

	if !cfg.WebOnlyMode {
		var err error
		bot, err = tgbotapi.NewBotAPI(cfg.TelegramBotToken)
		if err != nil {
			log.Fatalf("❌ Failed to initialize Telegram bot: %v", err)
		}

		log.Printf("✅ Telegram bot authorized on account @%s", bot.Self.UserName)

		// Create rate limiter
		limiter := rate.NewLimiter(rate.Every(cfg.RateLimitEvery), cfg.RateLimitBurst)

		// Create Telegram handler
		telegramHandler = handlers.NewTelegramHandler(bot, rssService, cfg)

		// Start listening for updates
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60

		updates, err := bot.GetUpdatesChan(u)
		if err != nil {
			log.Fatalf("❌ Failed to get updates channel: %v", err)
		}

		go func() {
			for update := range updates {
				if update.Message != nil {
					// Apply rate limiting
					if !limiter.Allow() {
						log.Println("⚠️ Rate limit exceeded, skipping message")
						continue
					}

					go telegramHandler.HandleMessage(update.Message)
				}
			}
		}()
	} else {
		log.Println("ℹ️ Running in web-only mode - Telegram bot disabled")
	}

	// Create API handler
	apiHandler := handlers.NewAPIHandler(rssService, cfg)

	// Setup HTTP routes
	http.HandleFunc("/api/articles", apiHandler.HandleArticles)
	http.HandleFunc("/api/health", apiHandler.HandleHealth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve static files from docs directory
		http.FileServer(http.Dir("./docs")).ServeHTTP(w, r)
	})

	// Start web server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🌐 Web server starting on port %s", cfg.Port)
		log.Printf("📱 Web interface: http://localhost:%s", cfg.Port)
		log.Printf("🔌 API endpoint: http://localhost:%s/api/articles", cfg.Port)
		log.Printf("💚 Health check: http://localhost:%s/api/health", cfg.Port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Web server error: %v", err)
		}
	}()

	// Keep the application running
	select {}
}
