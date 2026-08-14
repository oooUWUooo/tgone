package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"habr-rss-bot/internal/models"
	"habr-rss-bot/pkg/logger"
	"habr-rss-bot/pkg/metrics"
)

// Server represents the API server
type Server struct {
	logger  *logger.Logger
	metrics *metrics.Metrics
	startTime time.Time
	version   string
}

// NewServer creates a new API server
func NewServer(log *logger.Logger, m *metrics.Metrics, version string) *Server {
	return &Server{
		logger:    log,
		metrics:   m,
		startTime: time.Now(),
		version:   version,
	}
}

// Response is a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta contains pagination metadata
type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// WriteJSON writes a JSON response
func (s *Server) WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := Response{
		Success: statusCode < 400,
		Data:    data,
	}
	
	if statusCode >= 400 {
		if err, ok := data.(error); ok {
			response.Error = err.Error()
			response.Data = nil
		} else if errMsg, ok := data.(string); ok {
			response.Error = errMsg
			response.Data = nil
		}
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode JSON response", err)
	}
}

// WriteError writes an error response
func (s *Server) WriteError(w http.ResponseWriter, statusCode int, message string) {
	s.WriteJSON(w, statusCode, message)
}

// HealthHandler handles GET /api/health
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	uptime := time.Since(s.startTime)
	
	status := models.HealthStatus{
		Status:    "healthy",
		Version:   s.version,
		Uptime:    uptime.String(),
		Timestamp: time.Now().UTC(),
		Services: map[string]interface{}{
			"database": "connected",
			"cache":    "active",
		},
	}
	
	s.WriteJSON(w, http.StatusOK, status)
}

// ArticlesHandler handles GET /api/articles
func (s *Server) ArticlesHandler(fetcher ArticleFetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			s.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		
		ctx := r.Context()
		
		// Get pagination params
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		
		if page <= 0 {
			page = 1
		}
		if perPage <= 0 || perPage > 100 {
			perPage = 20
		}
		
		// Fetch articles
		articles, err := fetcher.FetchArticles(ctx)
		if err != nil {
			s.logger.Error("Failed to fetch articles", err)
			s.metrics.RecordRSSError()
			s.WriteError(w, http.StatusInternalServerError, "Failed to fetch articles")
			return
		}
		
		s.metrics.RecordRSSFetch()
		
		// Apply pagination
		total := len(articles)
		start := (page - 1) * perPage
		end := start + perPage
		
		if start >= total {
			articles = []models.Article{}
		} else {
			if end > total {
				end = total
			}
			articles = articles[start:end]
		}
		
		meta := &Meta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: (total + perPage - 1) / perPage,
		}
		
		response := map[string]interface{}{
			"articles": articles,
			"meta":     meta,
		}
		
		s.WriteJSON(w, http.StatusOK, response)
	}
}

// StatsHandler handles GET /api/stats
func (s *Server) StatsHandler(statsProvider StatsProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			s.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		
		ctx := r.Context()
		
		stats, err := statsProvider.GetStats(ctx)
		if err != nil {
			s.logger.Error("Failed to get stats", err)
			s.WriteError(w, http.StatusInternalServerError, "Failed to get statistics")
			return
		}
		
		s.WriteJSON(w, http.StatusOK, stats)
	}
}

// UsersHandler handles user-related endpoints
func (s *Server) UsersHandler(userProvider UserProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		switch r.Method {
		case http.MethodGet:
			s.handleGetUsers(w, r, userProvider)
		case http.MethodPost:
			s.handleCreateUser(w, r, userProvider)
		default:
			s.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request, provider UserProvider) {
	ctx := r.Context()
	
	// Get pagination params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	
	users, total, err := provider.GetUsers(ctx, page, perPage)
	if err != nil {
		s.logger.Error("Failed to get users", err)
		s.WriteError(w, http.StatusInternalServerError, "Failed to get users")
		return
	}
	
	meta := &Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: (total + perPage - 1) / perPage,
	}
	
	response := map[string]interface{}{
		"users": users,
		"meta":  meta,
	}
	
	s.WriteJSON(w, http.StatusOK, response)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, provider UserProvider) {
	var req struct {
		TelegramID int64  `json:"telegram_id"`
		Username   string `json:"username"`
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	ctx := r.Context()
	
	user, err := provider.CreateUser(ctx, req.TelegramID, req.Username, req.FirstName, req.LastName)
	if err != nil {
		s.logger.Error("Failed to create user", err)
		s.WriteError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}
	
	s.WriteJSON(w, http.StatusCreated, user)
}

// SubscriptionsHandler handles subscription-related endpoints
func (s *Server) SubscriptionsHandler(subProvider SubscriptionProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		switch r.Method {
		case http.MethodGet:
			s.handleGetSubscriptions(w, r, subProvider)
		case http.MethodPost:
			s.handleCreateSubscription(w, r, subProvider)
		default:
			s.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

func (s *Server) handleGetSubscriptions(w http.ResponseWriter, r *http.Request, provider SubscriptionProvider) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		s.WriteError(w, http.StatusBadRequest, "user_id parameter is required")
		return
	}
	
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		s.WriteError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}
	
	ctx := r.Context()
	
	subs, err := provider.GetSubscriptions(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get subscriptions", err)
		s.WriteError(w, http.StatusInternalServerError, "Failed to get subscriptions")
		return
	}
	
	s.WriteJSON(w, http.StatusOK, subs)
}

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request, provider SubscriptionProvider) {
	var req struct {
		UserID       int64  `json:"user_id"`
		FeedURL      string `json:"feed_url"`
		FeedName     string `json:"feed_name"`
		IntervalMins int    `json:"check_interval_minutes"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	if req.IntervalMins <= 0 {
		req.IntervalMins = 60
	}
	
	ctx := r.Context()
	
	sub, err := provider.CreateSubscription(ctx, req.UserID, req.FeedURL, req.FeedName, req.IntervalMins)
	if err != nil {
		s.logger.Error("Failed to create subscription", err)
		s.WriteError(w, http.StatusInternalServerError, "Failed to create subscription")
		return
	}
	
	s.WriteJSON(w, http.StatusCreated, sub)
}

// ArticleFetcher interface for fetching articles
type ArticleFetcher interface {
	FetchArticles(ctx context.Context) ([]models.Article, error)
}

// StatsProvider interface for getting statistics
type StatsProvider interface {
	GetStats(ctx context.Context) (interface{}, error)
}

// UserProvider interface for user operations
type UserProvider interface {
	GetUsers(ctx context.Context, page, perPage int) ([]models.User, int, error)
	CreateUser(ctx context.Context, telegramID int64, username, firstName, lastName string) (*models.User, error)
}

// SubscriptionProvider interface for subscription operations
type SubscriptionProvider interface {
	GetSubscriptions(ctx context.Context, userID int64) ([]models.Subscription, error)
	CreateSubscription(ctx context.Context, userID int64, feedURL, feedName string, intervalMins int) (*models.Subscription, error)
}

// WithCORS wraps a handler with CORS headers
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Wrap response writer to capture status code
			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			next.ServeHTTP(wrapped, r)
			
			duration := time.Since(start)
			
			log.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
