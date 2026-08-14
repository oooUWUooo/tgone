package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents the log level
type Level string

const (
	LevelDebug   Level = "DEBUG"
	LevelInfo    Level = "INFO"
	LevelWarn    Level = "WARN"
	LevelError   Level = "ERROR"
	LevelCritical Level = "CRITICAL"
)

// Logger is a structured logger with context support
type Logger struct {
	handler    slog.Handler
	slogger    *slog.Logger
	service    string
	version    string
	instanceID string
	mu         sync.RWMutex
	fields     map[string]interface{}
}

// Config holds logger configuration
type Config struct {
	Level      Level
	Service    string
	Version    string
	InstanceID string
	JSON       bool
	Output     io.Writer
}

// DefaultConfig returns a default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      LevelInfo,
		Service:    "habr-rss-bot",
		Version:    "3.0.0",
		InstanceID: getInstanceID(),
		JSON:       false,
		Output:     os.Stdout,
	}
}

// New creates a new logger instance
func New(cfg *Config) *Logger {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: getLogLevel(cfg.Level),
		AddSource: true,
	}

	if cfg.JSON {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	} else {
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	logger := &Logger{
		handler:    handler,
		slogger:    slog.New(handler),
		service:    cfg.Service,
		version:    cfg.Version,
		instanceID: cfg.InstanceID,
		fields:     make(map[string]interface{}),
	}

	// Add default fields
	logger.WithField("service", cfg.Service)
	logger.WithField("version", cfg.Version)
	logger.WithField("instance_id", cfg.InstanceID)

	return logger
}

// WithField adds a field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fields[key] = value
	return l
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, v := range fields {
		l.fields[k] = v
	}
	return l
}

// WithContext returns a logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}

	// Extract common context values
	newLogger := &Logger{
		handler:    l.handler,
		slogger:    l.slogger,
		service:    l.service,
		version:    l.version,
		instanceID: l.instanceID,
		fields:     make(map[string]interface{}),
	}

	// Copy existing fields
	l.mu.RLock()
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	l.mu.RUnlock()

	// Add context-specific fields
	if reqID := getRequestID(ctx); reqID != "" {
		newLogger.fields["request_id"] = reqID
	}

	if userID := getUserID(ctx); userID != "" {
		newLogger.fields["user_id"] = userID
	}

	return newLogger
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, args ...interface{}) {
	if err != nil {
		args = append(args, "error", err.Error())
	}
	l.log(LevelError, msg, args...)
}

// Critical logs a critical error message
func (l *Logger) Critical(msg string, err error, args ...interface{}) {
	if err != nil {
		args = append(args, "error", err.Error())
	}
	l.log(LevelCritical, msg, args...)
}

// Fatal logs a fatal error message and exits
func (l *Logger) Fatal(msg string, err error, args ...interface{}) {
	l.Critical(msg, err, args...)
	os.Exit(1)
}

func (l *Logger) log(level Level, msg string, args []interface{}) {
	l.mu.RLock()
	fields := make([]interface{}, 0, len(l.fields)*2)
	for k, v := range l.fields {
		fields = append(fields, k, v)
	}
	l.mu.RUnlock()

	fields = append(fields, args...)
	fields = append(fields, "level", string(level))

	switch level {
	case LevelDebug:
		l.slogger.Debug(msg, fields...)
	case LevelInfo:
		l.slogger.Info(msg, fields...)
	case LevelWarn:
		l.slogger.Warn(msg, fields...)
	case LevelError, LevelCritical:
		l.slogger.Error(msg, fields...)
	}
}

// GetSlogger returns the underlying slog logger
func (l *Logger) GetSlogger() *slog.Logger {
	return l.slogger
}

// SetLevel dynamically changes the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Re-create handler with new level
	opts := &slog.HandlerOptions{
		Level: getLogLevel(level),
		AddSource: true,
	}
	
	var handler slog.Handler
	if _, ok := l.handler.(*slog.JSONHandler); ok {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	
	l.handler = handler
	l.slogger = slog.New(handler)
}

func getLogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelCritical:
		return slog.LevelError + 4
	default:
		return slog.LevelInfo
	}
}

func getInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}

// Context keys for request tracing
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	TraceIDKey   contextKey = "trace_id"
)

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

func getRequestID(ctx context.Context) string {
	if v := ctx.Value(RequestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getUserID(ctx context.Context) string {
	if v := ctx.Value(UserIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetCallerInfo returns caller information
func GetCallerInfo() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		file = strings.Join(parts[len(parts)-2:], "/")
	}
	
	return fmt.Sprintf("%s:%d", file, line)
}

// StandardLogger wraps the standard log package
type StandardLogger struct {
	logger *Logger
}

// NewStandardLogger creates a wrapper for the standard log package
func NewStandardLogger(logger *Logger) *StandardLogger {
	return &StandardLogger{logger: logger}
}

func (sl *StandardLogger) Print(v ...interface{}) {
	sl.logger.Info(fmt.Sprint(v...))
}

func (sl *StandardLogger) Printf(format string, v ...interface{}) {
	sl.logger.Info(fmt.Sprintf(format, v...))
}

func (sl *StandardLogger) Println(v ...interface{}) {
	sl.logger.Info(fmt.Sprintln(v...))
}

// Global logger instance
var globalLogger *Logger
var globalLoggerOnce sync.Once

// Global returns the global logger instance
func Global() *Logger {
	globalLoggerOnce.Do(func() {
		globalLogger = New(DefaultConfig())
	})
	return globalLogger
}

// SetGlobal sets the global logger instance
func SetGlobal(logger *Logger) {
	globalLogger = logger
}

// Helper functions for common logging patterns

// LogRequest logs an HTTP request
func LogRequest(ctx context.Context, method, path string, duration time.Duration, statusCode int) {
	Global().WithContext(ctx).Info("HTTP request completed",
		"method", method,
		"path", path,
		"duration_ms", duration.Milliseconds(),
		"status_code", statusCode,
	)
}

// LogDatabaseQuery logs a database query
func LogDatabaseQuery(ctx context.Context, query string, duration time.Duration, rowsAffected int64) {
	Global().WithContext(ctx).Debug("Database query executed",
		"query", query,
		"duration_ms", duration.Milliseconds(),
		"rows_affected", rowsAffected,
	)
}

// LogError logs an error with stack trace
func LogError(ctx context.Context, err error, operation string) {
	Global().WithContext(ctx).Error("Operation failed",
		err,
		"operation", operation,
		"stack_trace", GetCallerInfo(),
	)
}
