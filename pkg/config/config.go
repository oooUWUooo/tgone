package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	TelegramToken string `mapstructure:"TELEGRAM_BOT_TOKEN"`
	Port          string `mapstructure:"PORT"`
	DBURL         string `mapstructure:"DATABASE_URL"`
	LogLevel      string `mapstructure:"LOG_LEVEL"`
	FetchInterval time.Duration `mapstructure:"FETCH_INTERVAL"`
	ArticleExpiry time.Duration `mapstructure:"ARTICLE_EXPIRY"`
}

func LoadConfig() (*Config, error) {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("FETCH_INTERVAL", 15 * time.Minute)
	viper.SetDefault("ARTICLE_EXPIRY", 24 * time.Hour)

	// Try to load from .env file
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables and defaults")
	}

	config := &Config{
		TelegramToken: viper.GetString("TELEGRAM_BOT_TOKEN"),
		Port:          viper.GetString("PORT"),
		DBURL:         viper.GetString("DATABASE_URL"),
		LogLevel:      viper.GetString("LOG_LEVEL"),
		FetchInterval: viper.GetDuration("FETCH_INTERVAL"),
		ArticleExpiry: viper.GetDuration("ARTICLE_EXPIRY"),
	}

	return config, nil
}
