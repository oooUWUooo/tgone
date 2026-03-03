package main

import (
	"habr-rss-bot/internal/delivery/http"
	"habr-rss-bot/internal/delivery/telegram"
	"habr-rss-bot/internal/repository"
	"habr-rss-bot/internal/usecase"
	"habr-rss-bot/pkg/config"
	"habr-rss-bot/pkg/logger"
	"log"
	"time"

	_ "habr-rss-bot/docs/swagger"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Habr RSS Bot API
// @version 1.0
// @description This is a professional RSS aggregator API.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
func main() {
	// 1. Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 2. Initialize logger
	l, err := logger.NewLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Error initializing logger: %v", err)
	}
	defer l.Sync()

	// 3. Database connection (PostgreSQL)
	db, err := sqlx.Connect("postgres", cfg.DBURL)
	if err != nil {
		l.Error("Failed to connect to database: " + err.Error())
	} else {
		// Run migrations
		m, err := migrate.New(
			"file://scripts/migrations",
			cfg.DBURL,
		)
		if err != nil {
			l.Error("Migration failed: " + err.Error())
		} else {
			if err := m.Up(); err != nil && err != migrate.ErrNoChange {
				l.Error("Migration UP failed: " + err.Error())
			}
		}
	}

	// 4. Initialize Repositories
	if db == nil {
		l.Fatal("Database connection is required")
	}
	articleRepo := repository.NewPostgresArticleRepository(db)
	sourceRepo := repository.NewPostgresSourceRepository(db)

	// 5. Initialize Usecases
	articleUsecase := usecase.NewArticleUsecase(articleRepo, sourceRepo, 30*time.Second)

	// 6. Background worker for fetching RSS feeds
	go func() {
		ticker := time.NewTicker(cfg.FetchInterval)
		defer ticker.Stop()

		// Initial fetch
		if err := articleUsecase.FetchAndSave(); err != nil {
			l.Error("Initial fetch failed: " + err.Error())
		}

		for range ticker.C {
			if err := articleUsecase.FetchAndSave(); err != nil {
				l.Error("Periodic fetch failed: " + err.Error())
			}
		}
	}()

	// 7. Start Telegram Bot
	if cfg.TelegramToken != "" {
		bot, err := telegram.NewTelegramBot(cfg.TelegramToken, articleUsecase)
		if err != nil {
			l.Error("Failed to initialize telegram bot: " + err.Error())
		} else {
			go bot.Start()
			l.Info("Telegram bot started")
		}
	}

	// 8. Start HTTP Server (Gin)
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API routes
	http.NewArticleHandler(r, articleUsecase)

	// Static files for frontend
	r.NoRoute(func(c *gin.Context) {
		c.File("./docs/index.html")
	})

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	l.Info("Starting HTTP server on port " + cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		l.Fatal("HTTP server failed: " + err.Error())
	}
}
