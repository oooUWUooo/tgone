// Package server serves the web UI and all REST/SSE API endpoints.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"habr-rss-bot/internal/config"
	"habr-rss-bot/internal/feed"
	"habr-rss-bot/internal/hub"
	"habr-rss-bot/internal/storage"
)

// Server wires together the HTTP layer and background feed refresher.
type Server struct {
	cfg     *config.Config
	fetcher *feed.Fetcher
	store   *storage.Store
	logger  *slog.Logger
	broker  *sseBroker
	http    *http.Server
}

// New builds all routes and returns a ready-to-run Server.
func New(cfg *config.Config, fetcher *feed.Fetcher, store *storage.Store, staticFS fs.FS, logger *slog.Logger) *Server {
	s := &Server{
		cfg:     cfg,
		fetcher: fetcher,
		store:   store,
		logger:  logger,
		broker:  newSSEBroker(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/articles", s.handleArticles) // ?hub=infosec
	mux.HandleFunc("/api/hubs", s.handleHubs)
	mux.HandleFunc("/api/search", s.handleSearch) // ?q=query
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/events", s.handleSSE) // Server-Sent Events
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // must be 0 for SSE long-lived responses
		IdleTimeout:  120 * time.Second,
	}
	return s
}

// Run starts the HTTP server and a background cache-warmer.
// It blocks until ctx is cancelled or a fatal listener error occurs.
func (s *Server) Run(ctx context.Context) error {
	go s.runRefresher(ctx)
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutCtx); err != nil {
			s.logger.Error("Graceful shutdown error", "error", err)
		}
	}()

	s.logger.Info("HTTP server started", "addr", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// runRefresher warms the cache for every hub on startup and then periodically,
// notifying all connected SSE clients after each refresh cycle.
func (s *Server) runRefresher(ctx context.Context) {
	s.refreshAll()

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshAll()
		}
	}
}

func (s *Server) refreshAll() {
	total := 0
	for _, h := range hub.All {
		s.fetcher.InvalidateCache(h.ID)
		articles, err := s.fetcher.Fetch(h.ID, h.URL)
		if err != nil {
			s.logger.Warn("Cache refresh failed", "hub", h.ID, "error", err)
			continue
		}
		total += len(articles)
	}
	s.broker.Publish(fmt.Sprintf(`{"type":"refresh","total":%d,"ts":"%s"}`,
		total, time.Now().UTC().Format(time.RFC3339)))
}

// ── JSON response types ────────────────────────────────────────────────────

type articleResponse struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Summary     string `json:"summary"`
	Image       string `json:"image"`
	Hub         string `json:"hub"`
	Date        string `json:"date"`
	ReadingTime int    `json:"reading_time"`
}

func toResponse(a feed.Article) articleResponse {
	return articleResponse{
		Title:       a.Title,
		Link:        a.Link,
		Summary:     a.Summary,
		Image:       a.ImageURL,
		Hub:         a.HubID,
		Date:        a.Date.Format(time.RFC3339),
		ReadingTime: a.ReadingTime,
	}
}

// ── Handlers ───────────────────────────────────────────────────────────────

// GET /api/articles?hub=infosec
func (s *Server) handleArticles(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}

	hubID := r.URL.Query().Get("hub")
	if hubID == "" {
		hubID = hub.DefaultHub.ID
	}
	h, ok := hub.ByID(hubID)
	if !ok {
		writeErr(w, "unknown hub", http.StatusBadRequest)
		return
	}

	articles, err := s.fetcher.Fetch(h.ID, h.URL)
	if err != nil {
		s.logger.Error("Feed fetch failed", "hub", hubID, "error", err)
		writeErr(w, "failed to fetch articles", http.StatusInternalServerError)
		return
	}

	resp := make([]articleResponse, 0, len(articles))
	for _, a := range articles {
		resp = append(resp, toResponse(a))
	}
	writeJSON(w, resp)
}

// GET /api/hubs
func (s *Server) handleHubs(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	type hubResp struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Emoji string `json:"emoji"`
	}
	hubs := make([]hubResp, 0, len(hub.All))
	for _, h := range hub.All {
		hubs = append(hubs, hubResp{ID: h.ID, Name: h.Name, Emoji: h.Emoji})
	}
	writeJSON(w, hubs)
}

// GET /api/search?q=query
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	q := r.URL.Query().Get("q")
	if q == "" {
		writeErr(w, "missing q parameter", http.StatusBadRequest)
		return
	}

	var all []feed.Article
	for _, h := range hub.All {
		articles, err := s.fetcher.Fetch(h.ID, h.URL)
		if err != nil {
			continue
		}
		all = append(all, articles...)
	}

	results := feed.Search(all, q)
	resp := make([]articleResponse, 0, len(results))
	for _, a := range results {
		resp = append(resp, toResponse(a))
	}
	writeJSON(w, resp)
}

// GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	type hubStat struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Emoji string `json:"emoji"`
		Count int    `json:"count"`
	}
	type statsResp struct {
		Hubs        []hubStat `json:"hubs"`
		Subscribers int       `json:"subscribers"`
		BotName     string    `json:"bot_name"`
	}

	resp := statsResp{
		Subscribers: s.store.Count(),
		BotName:     s.cfg.BotName,
	}
	for _, h := range hub.All {
		articles, _ := s.fetcher.Fetch(h.ID, h.URL)
		resp.Hubs = append(resp.Hubs, hubStat{
			ID:    h.ID,
			Name:  h.Name,
			Emoji: h.Emoji,
			Count: len(articles),
		})
	}
	writeJSON(w, resp)
}

// GET /api/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	writeJSON(w, map[string]interface{}{
		"status":      "ok",
		"bot":         s.cfg.BotName,
		"subscribers": s.store.Count(),
	})
}

// GET /api/events — Server-Sent Events stream for live feed refresh notifications.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported by this server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := s.broker.Subscribe()
	defer s.broker.Unsubscribe(ch)

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// ── SSE broker ─────────────────────────────────────────────────────────────

type sseBroker struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

func newSSEBroker() *sseBroker {
	return &sseBroker{clients: make(map[chan string]struct{})}
}

func (b *sseBroker) Subscribe() chan string {
	ch := make(chan string, 4)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *sseBroker) Unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func (b *sseBroker) Publish(data string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- data:
		default: // drop on slow clients rather than blocking
		}
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
