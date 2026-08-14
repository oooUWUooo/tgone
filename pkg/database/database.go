package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"habr-rss-bot/internal/models"
	"habr-rss-bot/pkg/logger"
)

// Database wraps the sql.DB connection
type Database struct {
	db       *sql.DB
	logger   *logger.Logger
	mu       sync.RWMutex
	maxConns int
}

// Config holds database configuration
type Config struct {
	DSN         string
	Driver      string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime time.Duration
}

// DefaultConfig returns a default database configuration
func DefaultConfig() *Config {
	return &Config{
		DSN:         "./data/habr_bot.db",
		Driver:      "sqlite3",
		MaxOpenConns: 25,
		MaxIdleConns: 5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// New creates a new database connection
func New(cfg *Config, log *logger.Logger) (*Database, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	
	if log == nil {
		log = logger.Global()
	}
	
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	
	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	database := &Database{
		db:       db,
		logger:   log,
		maxConns: cfg.MaxOpenConns,
	}
	
	// Run migrations
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	
	log.Info("Database connection established",
		"driver", cfg.Driver,
		"max_conns", cfg.MaxOpenConns,
	)
	
	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	d.logger.Info("Closing database connection")
	return d.db.Close()
}

// migrate runs database migrations
func (d *Database) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id BIGINT UNIQUE NOT NULL,
			username VARCHAR(255),
			first_name VARCHAR(255),
			last_name VARCHAR(255),
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guid VARCHAR(512) UNIQUE NOT NULL,
			title TEXT NOT NULL,
			link TEXT NOT NULL,
			summary TEXT,
			image_url TEXT,
			published_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE TABLE IF NOT EXISTS user_articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			article_id INTEGER NOT NULL,
			is_read BOOLEAN DEFAULT FALSE,
			is_favorite BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
			UNIQUE(user_id, article_id)
		)`,
		
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			feed_url TEXT NOT NULL,
			feed_name VARCHAR(255),
			is_active BOOLEAN DEFAULT TRUE,
			last_check_at TIMESTAMP,
			check_interval_minutes INTEGER DEFAULT 60,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, feed_url)
		)`,
		
		`CREATE TABLE IF NOT EXISTS bot_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE UNIQUE NOT NULL,
			total_users INTEGER DEFAULT 0,
			active_users INTEGER DEFAULT 0,
			messages_sent INTEGER DEFAULT 0,
			articles_fetched INTEGER DEFAULT 0,
			errors_count INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			action VARCHAR(100) NOT NULL,
			entity_type VARCHAR(50),
			entity_id INTEGER,
			old_value TEXT,
			new_value TEXT,
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
		)`,
		
		`CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_guid ON articles(guid)`,
		`CREATE INDEX IF NOT EXISTS idx_user_articles_user_id ON user_articles(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_active ON subscriptions(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON audit_log(user_id)`,
	}
	
	for i, migration := range migrations {
		if _, err := d.db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}
	
	d.logger.Info("Database migrations completed successfully")
	return nil
}

// User operations

// UpsertUser creates or updates a user
func (d *Database) UpsertUser(ctx context.Context, telegramID int64, username, firstName, lastName string) error {
	query := `
		INSERT INTO users (telegram_id, username, first_name, last_name, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(telegram_id) DO UPDATE SET
			username = excluded.username,
			first_name = excluded.first_name,
			last_name = excluded.last_name,
			updated_at = CURRENT_TIMESTAMP
	`
	
	_, err := d.db.ExecContext(ctx, query, telegramID, username, firstName, lastName)
	return err
}

// GetUserByTelegramID gets a user by Telegram ID
func (d *Database) GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	query := `SELECT id, telegram_id, username, first_name, last_name, is_active, created_at 
			  FROM users WHERE telegram_id = ?`
	
	row := d.db.QueryRowContext(ctx, query, telegramID)
	
	var user models.User
	var createdAt time.Time
	
	err := row.Scan(&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName, &user.IsActive, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	
	user.CreatedAt = createdAt
	return &user, nil
}

// GetActiveUsersCount returns the count of active users
func (d *Database) GetActiveUsersCount(ctx context.Context) (int, error) {
	var count int
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE is_active = TRUE").Scan(&count)
	return count, err
}

// Article operations

// SaveArticle saves an article to the database
func (d *Database) SaveArticle(ctx context.Context, article *models.Article) error {
	query := `
		INSERT INTO articles (guid, title, link, summary, image_url, published_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(guid) DO NOTHING
	`
	
	_, err := d.db.ExecContext(ctx, query, 
		article.GUID, 
		article.Title, 
		article.Link, 
		article.Summary, 
		article.ImageURL, 
		article.Date,
	)
	
	return err
}

// GetRecentArticles returns recent articles
func (d *Database) GetRecentArticles(ctx context.Context, limit int) ([]models.Article, error) {
	query := `SELECT guid, title, link, summary, image_url, published_at 
			  FROM articles 
			  ORDER BY published_at DESC 
			  LIMIT ?`
	
	rows, err := d.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var articles []models.Article
	for rows.Next() {
		var article models.Article
		var publishedAt time.Time
		
		err := rows.Scan(&article.GUID, &article.Title, &article.Link, &article.Summary, &article.ImageURL, &publishedAt)
		if err != nil {
			return nil, err
		}
		
		article.Date = publishedAt
		articles = append(articles, article)
	}
	
	return articles, rows.Err()
}

// ArticleExists checks if an article exists by GUID
func (d *Database) ArticleExists(ctx context.Context, guid string) (bool, error) {
	var exists bool
	err := d.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM articles WHERE guid = ?)", guid).Scan(&exists)
	return exists, err
}

// Subscription operations

// AddSubscription adds a subscription for a user
func (d *Database) AddSubscription(ctx context.Context, userID int64, feedURL, feedName string, intervalMinutes int) error {
	query := `
		INSERT INTO subscriptions (user_id, feed_url, feed_name, check_interval_minutes)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, feed_url) DO UPDATE SET
			is_active = TRUE,
			check_interval_minutes = excluded.check_interval_minutes
	`
	
	_, err := d.db.ExecContext(ctx, query, userID, feedURL, feedName, intervalMinutes)
	return err
}

// GetUserSubscriptions gets all subscriptions for a user
func (d *Database) GetUserSubscriptions(ctx context.Context, userID int64) ([]models.Subscription, error) {
	query := `SELECT id, user_id, feed_url, feed_name, is_active, last_check_at, check_interval_minutes, created_at
			  FROM subscriptions 
			  WHERE user_id = ? AND is_active = TRUE`
	
	rows, err := d.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var subscriptions []models.Subscription
	for rows.Next() {
		var sub models.Subscription
		var lastCheckAt, createdAt sql.NullTime
		
		err := rows.Scan(&sub.ID, &sub.UserID, &sub.FeedURL, &sub.FeedName, &sub.IsActive, &lastCheckAt, &sub.CheckIntervalMinutes, &createdAt)
		if err != nil {
			return nil, err
		}
		
		if lastCheckAt.Valid {
			sub.LastCheckAt = &lastCheckAt.Time
		}
		sub.CreatedAt = createdAt.Time
		
		subscriptions = append(subscriptions, sub)
	}
	
	return subscriptions, rows.Err()
}

// UpdateSubscriptionLastCheck updates the last check time for a subscription
func (d *Database) UpdateSubscriptionLastCheck(ctx context.Context, subID int64) error {
	query := `UPDATE subscriptions SET last_check_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := d.db.ExecContext(ctx, query, subID)
	return err
}

// Stats operations

// RecordDailyStats records daily statistics
func (d *Database) RecordDailyStats(ctx context.Context, date string, totalUsers, activeUsers, messagesSent, articlesFetched, errorsCount int) error {
	query := `
		INSERT INTO bot_stats (date, total_users, active_users, messages_sent, articles_fetched, errors_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(date) DO UPDATE SET
			total_users = excluded.total_users,
			active_users = excluded.active_users,
			messages_sent = excluded.messages_sent,
			articles_fetched = excluded.articles_fetched,
			errors_count = excluded.errors_count,
			updated_at = CURRENT_TIMESTAMP
	`
	
	_, err := d.db.ExecContext(ctx, query, date, totalUsers, activeUsers, messagesSent, articlesFetched, errorsCount)
	return err
}

// AuditLog operations

// LogAuditEntry logs an audit entry
func (d *Database) LogAuditEntry(ctx context.Context, userID *int64, action, entityType string, entityID *int64, oldValue, newValue, ipAddress, userAgent string) error {
	query := `
		INSERT INTO audit_log (user_id, action, entity_type, entity_id, old_value, new_value, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := d.db.ExecContext(ctx, query, userID, action, entityType, entityID, oldValue, newValue, ipAddress, userAgent)
	return err
}

// Health check

// Health checks the database health
func (d *Database) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	var version string
	err := d.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get SQLite version: %w", err)
	}
	
	d.logger.Debug("Database health check passed", "sqlite_version", version)
	return nil
}

// Stats returns database statistics
func (d *Database) Stats() sql.DBStats {
	return d.db.Stats()
}
