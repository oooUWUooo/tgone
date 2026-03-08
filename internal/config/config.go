package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for a single bot instance.
// All fields are populated from environment variables.
type Config struct {
	TelegramToken string
	Port          string
	BotName       string
	FeedURL       string
	MaxArticles   int
	ArticleExpiry time.Duration
	WebOnly       bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	token := getEnv("TELEGRAM_BOT_TOKEN", "")
	webOnly := token == "" || token == "dummy_token_for_testing"

	maxArticles, _ := strconv.Atoi(getEnv("MAX_ARTICLES", "10"))
	if maxArticles <= 0 {
		maxArticles = 10
	}

	expiryHours, _ := strconv.Atoi(getEnv("ARTICLE_EXPIRY_HOURS", "24"))
	if expiryHours <= 0 {
		expiryHours = 24
	}

	return &Config{
		TelegramToken: token,
		Port:          getEnv("PORT", "8080"),
		BotName:       getEnv("BOT_NAME", "HabrInfoSecBot"),
		FeedURL:       getEnv("FEED_URL", "https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru"),
		MaxArticles:   maxArticles,
		ArticleExpiry: time.Duration(expiryHours) * time.Hour,
		WebOnly:       webOnly,
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
