package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"habr-rss-bot/pkg/logger"
	"habr-rss-bot/pkg/metrics"
)

// Middleware is a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// Chain chains multiple middlewares together
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// Logging returns a middleware that logs HTTP requests
func Logging(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Generate request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			
			// Add request ID to context and response headers
			ctx := logger.WithRequestID(r.Context(), requestID)
			r = r.WithContext(ctx)
			
			// Wrap response writer to capture status code
			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			next.ServeHTTP(wrapped, r)
			
			duration := time.Since(start)
			
			log.WithContext(ctx).Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"status_code", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"user_agent", r.UserAgent(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// Recovery returns a middleware that recovers from panics
func Recovery(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.WithContext(r.Context()).Error("Panic recovered",
						nil,
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
					)
					
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			
			next.ServeHTTP(w, r)
		})
	}
}

// Metrics returns a middleware that records HTTP metrics
func Metrics(m *metrics.Metrics) Middleware {
	return func(next http.Handler) http.Handler {
		return m.Middleware(next)
	}
}

// CORS returns a middleware that adds CORS headers
func CORS(allowedOrigins []string, allowedMethods []string, allowedHeaders []string) Middleware {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	
	if len(allowedMethods) == 0 {
		allowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	
	if len(allowedHeaders) == 0 {
		allowedHeaders = []string{"Content-Type", "Authorization", "X-Request-ID"}
	}
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Check if origin is allowed
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if origin == "" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
			}
			
			w.Header().Set("Access-Control-Allow-Methods", join(allowedMethods))
			w.Header().Set("Access-Control-Allow-Headers", join(allowedHeaders))
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit returns a middleware that rate limits requests
func RateLimit(limit int, window time.Duration) Middleware {
	type client struct {
		count    int
		resetAt  time.Time
	}
	
	clients := make(map[string]*client)
	mu := make(chan struct{}, 1)
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			
			mu <- struct{}{}
			c, exists := clients[ip]
			if !exists {
				c = &client{
					count:   1,
					resetAt: time.Now().Add(window),
				}
				clients[ip] = c
				mu <- struct{}{}
			} else {
				mu <- struct{}{}
				
				if time.Now().After(c.resetAt) {
					c.count = 1
					c.resetAt = time.Now().Add(window)
				} else {
					c.count++
				}
				
				if c.count > limit {
					w.Header().Set("Retry-After", "60")
					http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
					return
				}
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// Auth returns a middleware that checks for API key authentication
func Auth(apiKeys map[string]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("api_key")
			}
			
			if apiKey == "" {
				http.Error(w, "Missing API key", http.StatusUnauthorized)
				return
			}
			
			userID, ok := apiKeys[apiKey]
			if !ok {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}
			
			ctx := logger.WithUserID(r.Context(), userID)
			r = r.WithContext(ctx)
			
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID returns a middleware that adds a request ID to the context
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			
			ctx := context.WithValue(r.Context(), "request_id", requestID)
			r = r.WithContext(ctx)
			
			w.Header().Set("X-Request-ID", requestID)
			
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout returns a middleware that adds a timeout to requests
func Timeout(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			r = r.WithContext(ctx)
			
			done := make(chan bool, 1)
			go func() {
				next.ServeHTTP(w, r)
				done <- true
			}()
			
			select {
			case <-done:
				return
			case <-ctx.Done():
				http.Error(w, "Request timeout", http.StatusGatewayTimeout)
				return
			}
		})
	}
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// join joins a slice of strings with commas
func join(slice []string) string {
	result := ""
	for i, s := range slice {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// SecurityHeaders returns a middleware that adds security headers
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			
			next.ServeHTTP(w, r)
		})
	}
}

// Compress returns a middleware that compresses responses (placeholder)
func Compress() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement gzip compression
			next.ServeHTTP(w, r)
		})
	}
}
