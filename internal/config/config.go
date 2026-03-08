// Package config loads bot configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for one bot instance.
type Config struct {
	TelegramToken string
	Port          string
	BotName       string
	MaxArticles   int
	ArticleExpiry time.Duration // how long until a sent article is "forgotten"
	CacheTTL      time.Duration // how long feed results are cached
	PollInterval  time.Duration // how often the background poller runs
	DataFile      string        // path for subscription persistence; "" = in-memory only
	WebOnly       bool
}

// Load reads configuration from environment variables with safe defaults.
func Load() *Config {
	token := getEnv("TELEGRAM_BOT_TOKEN", "")
	webOnly := token == "" || token == "dummy_token_for_testing"

	return &Config{
		TelegramToken: token,
		Port:          getEnv("PORT", "8080"),
		BotName:       getEnv("BOT_NAME", "HabrInfoSecBot"),
		MaxArticles:   parseInt(getEnv("MAX_ARTICLES", "20"), 20),
		ArticleExpiry: time.Duration(parseInt(getEnv("ARTICLE_EXPIRY_HOURS", "24"), 24)) * time.Hour,
		CacheTTL:      time.Duration(parseInt(getEnv("CACHE_TTL_MINUTES", "5"), 5)) * time.Minute,
		PollInterval:  time.Duration(parseInt(getEnv("POLL_INTERVAL_MINUTES", "15"), 15)) * time.Minute,
		DataFile:      getEnv("DATA_FILE", "subscriptions.json"),
		WebOnly:       webOnly,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseInt(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}
