package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all application metrics
type Metrics struct {
	registry        *prometheus.Registry
	httpRequests    *prometheus.CounterVec
	httpDuration    *prometheus.HistogramVec
	dbQueries       *prometheus.CounterVec
	dbDuration      *prometheus.HistogramVec
	cacheHits       prometheus.Counter
	cacheMisses     prometheus.Counter
	rssFetches      prometheus.Counter
	rssErrors       prometheus.Counter
	activeUsers     prometheus.Gauge
	messageCounter  *prometheus.CounterVec
}

var (
	instance     *Metrics
	initOnce     sync.Once
	metricsMutex sync.RWMutex
)

// New creates a new Metrics instance
func New() *Metrics {
	registry := prometheus.NewRegistry()

	factory := promauto.With(registry)

	m := &Metrics{
		registry: registry,
		httpRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		httpDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		dbQueries: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "database_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"query_type", "table"},
		),
		dbDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "database_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"query_type"},
		),
		cacheHits: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
		),
		cacheMisses: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
		),
		rssFetches: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "rss_fetches_total",
				Help: "Total number of RSS feed fetches",
			},
		),
		rssErrors: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "rss_errors_total",
				Help: "Total number of RSS fetch errors",
			},
		),
		activeUsers: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_users",
				Help: "Number of active users",
			},
		),
		messageCounter: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bot_messages_total",
				Help: "Total number of bot messages by type",
			},
			[]string{"message_type", "command"},
		),
	}

	return m
}

// Get returns the global metrics instance
func Get() *Metrics {
	initOnce.Do(func() {
		instance = New()
	})
	return instance
}

// Handler returns the Prometheus metrics HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordHTTPRequest records an HTTP request metric
func (m *Metrics) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	m.httpRequests.WithLabelValues(method, path, fmt.Sprintf("%d", statusCode)).Inc()
	m.httpDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordDBQuery records a database query metric
func (m *Metrics) RecordDBQuery(queryType, table string, duration time.Duration) {
	m.dbQueries.WithLabelValues(queryType, table).Inc()
	m.dbDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	m.cacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses.Inc()
}

// RecordRSSFetch records an RSS fetch
func (m *Metrics) RecordRSSFetch() {
	m.rssFetches.Inc()
}

// RecordRSSError records an RSS fetch error
func (m *Metrics) RecordRSSError() {
	m.rssErrors.Inc()
}

// SetActiveUsers sets the number of active users
func (m *Metrics) SetActiveUsers(count int) {
	m.activeUsers.Set(float64(count))
}

// RecordMessage records a bot message
func (m *Metrics) RecordMessage(messageType, command string) {
	m.messageCounter.WithLabelValues(messageType, command).Inc()
}

// Middleware returns an HTTP middleware for recording metrics
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap response writer to capture status code
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		m.RecordHTTPRequest(r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
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

// Collector interface for custom metrics
type Collector interface {
	Collect(ctx context.Context) error
}

// CollectorRegistry manages custom collectors
type CollectorRegistry struct {
	collectors []Collector
	mu         sync.RWMutex
}

// NewCollectorRegistry creates a new collector registry
func NewCollectorRegistry() *CollectorRegistry {
	return &CollectorRegistry{
		collectors: make([]Collector, 0),
	}
}

// Register registers a custom collector
func (cr *CollectorRegistry) Register(collector Collector) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.collectors = append(cr.collectors, collector)
}

// CollectAll collects metrics from all registered collectors
func (cr *CollectorRegistry) CollectAll(ctx context.Context) error {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	
	for _, collector := range cr.collectors {
		if err := collector.Collect(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Gauges holds dynamic gauge values
type Gauges struct {
	values map[string]prometheus.Gauge
	mu     sync.RWMutex
}

// NewGauges creates a new Gauges instance
func NewGauges(registry *prometheus.Registry) *Gauges {
	return &Gauges{
		values: make(map[string]prometheus.Gauge),
	}
}

// RegisterGauge registers a new gauge
func (g *Gauges) RegisterGauge(name, help string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	})
	
	g.values[name] = gauge
}

// Set sets a gauge value
func (g *Gauges) Set(name string, value float64) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if gauge, ok := g.values[name]; ok {
		gauge.Set(value)
		return nil
	}
	return fmt.Errorf("gauge %s not found", name)
}

// Inc increments a gauge
func (g *Gauges) Inc(name string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if gauge, ok := g.values[name]; ok {
		gauge.Inc()
		return nil
	}
	return fmt.Errorf("gauge %s not found", name)
}

// Dec decrements a gauge
func (g *Gauges) Dec(name string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if gauge, ok := g.values[name]; ok {
		gauge.Dec()
		return nil
	}
	return fmt.Errorf("gauge %s not found", name)
}
