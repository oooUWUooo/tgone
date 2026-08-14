package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// SecretStore provides secure storage for sensitive configuration
type SecretStore struct {
	mux          sync.RWMutex
	secrets      map[string]string
	encryptionKey []byte
	configPath   string
}

// SecretConfig represents the YAML structure for secrets
type SecretConfig struct {
	Telegram struct {
		BotToken       string `yaml:"bot_token"`
		AdminUserIDs   []int64 `yaml:"admin_user_ids"`
		AllowedChatIDs []int64 `yaml:"allowed_chat_ids"`
	} `yaml:"telegram"`

	Database struct {
		SQLitePath    string `yaml:"sqlite_path"`
		PostgresURL   string `yaml:"postgres_url"`
		RedisURL      string `yaml:"redis_url"`
		MaxConnections int    `yaml:"max_connections"`
	} `yaml:"database"`

	API struct {
		HabrRSSURLs        []string `yaml:"habr_rss_urls"`
		GitHubToken        string   `yaml:"github_token"`
		PrometheusUsername string   `yaml:"prometheus_username"`
		PrometheusPassword string   `yaml:"prometheus_password"`
		JWTSecret          string   `yaml:"jwt_secret"`
	} `yaml:"api"`

	Scheduler struct {
		FetchIntervalMinutes  int `yaml:"fetch_interval_minutes"`
		CleanupIntervalHours  int `yaml:"cleanup_interval_hours"`
		BackoffMultiplier     float64 `yaml:"backoff_multiplier"`
		MaxRetries            int `yaml:"max_retries"`
		RateLimitPerMinute    int `yaml:"rate_limit_per_minute"`
	} `yaml:"scheduler"`

	Encryption struct {
		MasterKey string `yaml:"master_key"`
	} `yaml:"encryption"`

	Notifications struct {
		WebhookURLs        []string `yaml:"webhook_urls"`
		EmailSMTPHost      string   `yaml:"email_smtp_host"`
		EmailSMTPPort      int      `yaml:"email_smtp_port"`
		EmailUsername      string   `yaml:"email_username"`
		EmailPassword      string   `yaml:"email_password"`
		SlackWebhookURL    string   `yaml:"slack_webhook_url"`
		DiscordWebhookURL  string   `yaml:"discord_webhook_url"`
	} `yaml:"notifications"`

	FeatureFlags struct {
		EnableTopics         bool `yaml:"enable_topics"`
		EnableSubscriptions  bool `yaml:"enable_subscriptions"`
		EnableAnalytics      bool `yaml:"enable_analytics"`
		EnableAutoModeration bool `yaml:"enable_auto_moderation"`
		EnableBackup         bool `yaml:"enable_backup"`
	} `yaml:"feature_flags"`
}

var (
	defaultStore *SecretStore
	storeOnce    sync.Once
)

// GetStore returns the singleton secret store instance
func GetStore() (*SecretStore, error) {
	var err error
	storeOnce.Do(func() {
		defaultStore, err = NewSecretStore("configs/secrets.yaml")
	})
	return defaultStore, err
}

// NewSecretStore creates a new secret store from config file
func NewSecretStore(configPath string) (*SecretStore, error) {
	store := &SecretStore{
		secrets:    make(map[string]string),
		configPath: configPath,
	}

	// Try to load encryption key from environment
	if key := os.Getenv("SECRET_ENCRYPTION_KEY"); key != "" {
		store.encryptionKey = []byte(key)
	}

	// Load secrets from file
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}

	return store, nil
}

// Load reads secrets from the configuration file
func (s *SecretStore) Load() error {
	s.mux.Lock()
	defer s.mux.Unlock()

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config if it doesn't exist
			return s.createDefaultConfig()
		}
		return err
	}

	var config SecretConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse secrets config: %w", err)
	}

	// Populate secrets map
	s.populateSecrets(config)

	return nil
}

// Save writes secrets to the configuration file
func (s *SecretStore) Save() error {
	s.mux.RLock()
	defer s.mux.RUnlock()

	config := s.buildConfigFromSecrets()

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	// Encrypt sensitive fields if encryption key is set
	if len(s.encryptionKey) > 0 {
		encrypted, err := s.encryptSensitiveFields(string(data))
		if err != nil {
			return fmt.Errorf("failed to encrypt secrets: %w", err)
		}
		data = []byte(encrypted)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write with restrictive permissions
	if err := os.WriteFile(s.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write secrets file: %w", err)
	}

	return nil
}

func (s *SecretStore) createDefaultConfig() error {
	config := SecretConfig{
		Telegram: struct {
			BotToken       string `yaml:"bot_token"`
			AdminUserIDs   []int64 `yaml:"admin_user_ids"`
			AllowedChatIDs []int64 `yaml:"allowed_chat_ids"`
		}{
			BotToken:       "YOUR_TELEGRAM_BOT_TOKEN_HERE",
			AdminUserIDs:   []int64{},
			AllowedChatIDs: []int64{},
		},
		Database: struct {
			SQLitePath    string `yaml:"sqlite_path"`
			PostgresURL   string `yaml:"postgres_url"`
			RedisURL      string `yaml:"redis_url"`
			MaxConnections int    `yaml:"max_connections"`
		}{
			SQLitePath:    "./data/bot.db",
			PostgresURL:   "postgres://user:password@localhost:5432/habr_bot?sslmode=disable",
			RedisURL:      "redis://localhost:6379/0",
			MaxConnections: 10,
		},
		API: struct {
			HabrRSSURLs        []string `yaml:"habr_rss_urls"`
			GitHubToken        string   `yaml:"github_token"`
			PrometheusUsername string   `yaml:"prometheus_username"`
			PrometheusPassword string   `yaml:"prometheus_password"`
			JWTSecret          string   `yaml:"jwt_secret"`
		}{
			HabrRSSURLs: []string{
				"https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru",
				"https://habr.com/ru/rss/hub/programming/all/?fl=ru",
				"https://habr.com/ru/rss/hub/devops/all/?fl=ru",
			},
			GitHubToken:        "",
			PrometheusUsername: "",
			PrometheusPassword: "",
			JWTSecret:          generateSecureRandomString(32),
		},
		Scheduler: struct {
			FetchIntervalMinutes int     `yaml:"fetch_interval_minutes"`
			CleanupIntervalHours int     `yaml:"cleanup_interval_hours"`
			BackoffMultiplier    float64 `yaml:"backoff_multiplier"`
			MaxRetries           int     `yaml:"max_retries"`
			RateLimitPerMinute   int     `yaml:"rate_limit_per_minute"`
		}{
			FetchIntervalMinutes: 5,
			CleanupIntervalHours: 1,
			BackoffMultiplier:    2.0,
			MaxRetries:           3,
			RateLimitPerMinute:   60,
		},
		Encryption: struct {
			MasterKey string `yaml:"master_key"`
		}{
			MasterKey: generateSecureRandomString(32),
		},
		Notifications: struct {
			WebhookURLs       []string `yaml:"webhook_urls"`
			EmailSMTPHost     string   `yaml:"email_smtp_host"`
			EmailSMTPPort     int      `yaml:"email_smtp_port"`
			EmailUsername     string   `yaml:"email_username"`
			EmailPassword     string   `yaml:"email_password"`
			SlackWebhookURL   string   `yaml:"slack_webhook_url"`
			DiscordWebhookURL string   `yaml:"discord_webhook_url"`
		}{
			WebhookURLs:       []string{},
			EmailSMTPHost:     "smtp.example.com",
			EmailSMTPPort:     587,
			EmailUsername:     "",
			EmailPassword:     "",
			SlackWebhookURL:   "",
			DiscordWebhookURL: "",
		},
		FeatureFlags: struct {
			EnableTopics         bool `yaml:"enable_topics"`
			EnableSubscriptions  bool `yaml:"enable_subscriptions"`
			EnableAnalytics      bool `yaml:"enable_analytics"`
			EnableAutoModeration bool `yaml:"enable_auto_moderation"`
			EnableBackup         bool `yaml:"enable_backup"`
		}{
			EnableTopics:         true,
			EnableSubscriptions:  true,
			EnableAnalytics:      true,
			EnableAutoModeration: false,
			EnableBackup:         true,
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(s.configPath, data, 0600)
}

func (s *SecretStore) populateSecrets(config SecretConfig) {
	s.secrets["telegram.bot_token"] = config.Telegram.BotToken
	for i, id := range config.Telegram.AdminUserIDs {
		s.secrets[fmt.Sprintf("telegram.admin_user_%d", i)] = fmt.Sprintf("%d", id)
	}
	for i, id := range config.Telegram.AllowedChatIDs {
		s.secrets[fmt.Sprintf("telegram.allowed_chat_%d", i)] = fmt.Sprintf("%d", id)
	}

	s.secrets["database.sqlite_path"] = config.Database.SQLitePath
	s.secrets["database.postgres_url"] = config.Database.PostgresURL
	s.secrets["database.redis_url"] = config.Database.RedisURL
	s.secrets["database.max_connections"] = fmt.Sprintf("%d", config.Database.MaxConnections)

	for i, url := range config.API.HabrRSSURLs {
		s.secrets[fmt.Sprintf("api.habr_rss_url_%d", i)] = url
	}
	s.secrets["api.github_token"] = config.API.GitHubToken
	s.secrets["api.prometheus_username"] = config.API.PrometheusUsername
	s.secrets["api.prometheus_password"] = config.API.PrometheusPassword
	s.secrets["api.jwt_secret"] = config.API.JWTSecret

	s.secrets["scheduler.fetch_interval_minutes"] = fmt.Sprintf("%d", config.Scheduler.FetchIntervalMinutes)
	s.secrets["scheduler.cleanup_interval_hours"] = fmt.Sprintf("%d", config.Scheduler.CleanupIntervalHours)
	s.secrets["scheduler.backoff_multiplier"] = fmt.Sprintf("%f", config.Scheduler.BackoffMultiplier)
	s.secrets["scheduler.max_retries"] = fmt.Sprintf("%d", config.Scheduler.MaxRetries)
	s.secrets["scheduler.rate_limit_per_minute"] = fmt.Sprintf("%d", config.Scheduler.RateLimitPerMinute)

	s.secrets["encryption.master_key"] = config.Encryption.MasterKey

	for i, url := range config.Notifications.WebhookURLs {
		s.secrets[fmt.Sprintf("notifications.webhook_url_%d", i)] = url
	}
	s.secrets["notifications.email_smtp_host"] = config.Notifications.EmailSMTPHost
	s.secrets["notifications.email_smtp_port"] = fmt.Sprintf("%d", config.Notifications.EmailSMTPPort)
	s.secrets["notifications.email_username"] = config.Notifications.EmailUsername
	s.secrets["notifications.email_password"] = config.Notifications.EmailPassword
	s.secrets["notifications.slack_webhook_url"] = config.Notifications.SlackWebhookURL
	s.secrets["notifications.discord_webhook_url"] = config.Notifications.DiscordWebhookURL

	for key, value := range map[string]bool{
		"features.enable_topics":          config.FeatureFlags.EnableTopics,
		"features.enable_subscriptions":   config.FeatureFlags.EnableSubscriptions,
		"features.enable_analytics":       config.FeatureFlags.EnableAnalytics,
		"features.enable_auto_moderation": config.FeatureFlags.EnableAutoModeration,
		"features.enable_backup":          config.FeatureFlags.EnableBackup,
	} {
		s.secrets[key] = fmt.Sprintf("%v", value)
	}
}

func (s *SecretStore) buildConfigFromSecrets() SecretConfig {
	config := SecretConfig{}

	config.Telegram.BotToken = s.getSecret("telegram.bot_token", "")
	config.Database.SQLitePath = s.getSecret("database.sqlite_path", "./data/bot.db")
	config.Database.PostgresURL = s.getSecret("database.postgres_url", "")
	config.Database.RedisURL = s.getSecret("database.redis_url", "")
	config.API.JWTSecret = s.getSecret("api.jwt_secret", "")
	config.Encryption.MasterKey = s.getSecret("encryption.master_key", "")

	return config
}

func (s *SecretStore) getSecret(key, defaultValue string) string {
	s.mux.RLock()
	defer s.mux.RUnlock()

	if value, exists := s.secrets[key]; exists {
		return value
	}
	return defaultValue
}

// Get retrieves a secret by key
func (s *SecretStore) Get(key string) (string, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	if value, exists := s.secrets[key]; exists {
		return value, nil
	}
	return "", errors.New("secret not found")
}

// GetOrEnv retrieves a secret or falls back to environment variable
func (s *SecretStore) GetOrEnv(key, envVar string) string {
	if value, err := s.Get(key); err == nil {
		return value
	}
	return os.Getenv(envVar)
}

// Set updates or adds a secret
func (s *SecretStore) Set(key, value string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.secrets[key] = value
}

// Delete removes a secret
func (s *SecretStore) Delete(key string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	delete(s.secrets, key)
}

// Encrypt encrypts data using AES-GCM
func (s *SecretStore) Encrypt(plaintext string) (string, error) {
	if len(s.encryptionKey) == 0 {
		return plaintext, nil
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts data using AES-GCM
func (s *SecretStore) Decrypt(ciphertext string) (string, error) {
	if len(s.encryptionKey) == 0 {
		return ciphertext, nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (s *SecretStore) encryptSensitiveFields(data string) (string, error) {
	// In a real implementation, you would selectively encrypt sensitive fields
	// For now, we'll return the data as-is
	return data, nil
}

func generateSecureRandomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}
