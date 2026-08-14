package models

import "time"

// Article represents a news article from Habr
type Article struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Summary     string    `json:"summary"`
	ImageURL    string    `json:"image,omitempty"`
	Date        time.Time `json:"date"`
	GUID        string    `json:"guid,omitempty"`
	Description string    `json:"description,omitempty"`
}

// APIResponse is a standard API response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta contains pagination and metadata
type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// ChatMessage represents a message in the chat
type ChatMessage struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	IsUser      bool      `json:"is_user"`
	MessageType string    `json:"message_type"` // text, html, articles, loading
	Timestamp   time.Time `json:"timestamp"`
}

// Command represents a bot command
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Usage       string `json:"usage"`
}

// User represents a Telegram user
type User struct {
	ID         int64     `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	Username   string    `json:"username,omitempty"`
	FirstName  string    `json:"first_name,omitempty"`
	LastName   string    `json:"last_name,omitempty"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// Subscription represents a user's RSS subscription
type Subscription struct {
	ID                 int64      `json:"id"`
	UserID             int64      `json:"user_id"`
	FeedURL            string     `json:"feed_url"`
	FeedName           string     `json:"feed_name,omitempty"`
	IsActive           bool       `json:"is_active"`
	LastCheckAt        *time.Time `json:"last_check_at,omitempty"`
	CheckIntervalMinutes int      `json:"check_interval_minutes"`
	CreatedAt          time.Time  `json:"created_at"`
}

// BotStats represents daily bot statistics
type BotStats struct {
	ID              int       `json:"id"`
	Date            string    `json:"date"`
	TotalUsers      int       `json:"total_users"`
	ActiveUsers     int       `json:"active_users"`
	MessagesSent    int       `json:"messages_sent"`
	ArticlesFetched int       `json:"articles_fetched"`
	ErrorsCount     int       `json:"errors_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	ID         int64     `json:"id"`
	UserID     *int64    `json:"user_id,omitempty"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type,omitempty"`
	EntityID   *int64    `json:"entity_id,omitempty"`
	OldValue   string    `json:"old_value,omitempty"`
	NewValue   string    `json:"new_value,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// HealthStatus represents service health status
type HealthStatus struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Uptime    string                 `json:"uptime"`
	Timestamp time.Time              `json:"timestamp"`
	Services  map[string]interface{} `json:"services,omitempty"`
}

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

// PaginatedResponse wraps a paginated response
type PaginatedResponse struct {
	Items interface{} `json:"items"`
	Meta  Meta        `json:"meta"`
}

// NewPaginationParams creates default pagination params
func NewPaginationParams() *PaginationParams {
	return &PaginationParams{
		Page:    1,
		PerPage: 20,
	}
}

// Offset returns the offset for database queries
func (p *PaginationParams) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// Limit returns the limit for database queries
func (p *PaginationParams) Limit() int {
	if p.PerPage <= 0 || p.PerPage > 100 {
		return 20
	}
	return p.PerPage
}
