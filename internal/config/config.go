package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	TelegramBotToken string
	Port             string
	WebOnlyMode      bool
	
	// RSS Feed settings
	HabrRSSURL string
	
	// Article settings
	MaxArticles     int
	ArticleExpiry   time.Duration
	CleanupInterval time.Duration
	
	// Rate limiting
	RateLimitEvery  time.Duration
	RateLimitBurst  int
	
	// HTTP Client
	HTTPTimeout time.Duration
	
	// Logging
	LogLevel string
}

// Load reads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		Port:             getEnvOrDefault("PORT", "8080"),
		HabrRSSURL:       getEnvOrDefault("HABR_RSS_URL", "https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru"),
		MaxArticles:      getIntEnvOrDefault("MAX_ARTICLES", 10),
		ArticleExpiry:    getDurationEnvOrDefault("ARTICLE_EXPIRY", 24*time.Hour),
		CleanupInterval:  getDurationEnvOrDefault("CLEANUP_INTERVAL", 1*time.Hour),
		RateLimitEvery:   getDurationEnvOrDefault("RATE_LIMIT_EVERY", 1*time.Second),
		RateLimitBurst:   getIntEnvOrDefault("RATE_LIMIT_BURST", 1),
		HTTPTimeout:      getDurationEnvOrDefault("HTTP_TIMEOUT", 30*time.Second),
		LogLevel:         getEnvOrDefault("LOG_LEVEL", "info"),
	}
	
	// Determine if running in web-only mode
	cfg.WebOnlyMode = cfg.TelegramBotToken == "" || cfg.TelegramBotToken == "dummy_token_for_testing"
	
	return cfg
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnvOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnvOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
