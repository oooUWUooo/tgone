package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"habr-rss-bot/internal/config"
	"habr-rss-bot/internal/feed"
)

// Server serves the web UI and the /api/articles + /api/health endpoints.
type Server struct {
	cfg     *config.Config
	fetcher *feed.Fetcher
	logger  *slog.Logger
	http    *http.Server
}

// New wires up all HTTP routes and returns a ready-to-run Server.
// staticFS must be a rooted fs.FS serving the web UI files.
func New(cfg *config.Config, fetcher *feed.Fetcher, staticFS fs.FS, logger *slog.Logger) *Server {
	s := &Server{cfg: cfg, fetcher: fetcher, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/articles", s.handleArticles)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

// Run starts the HTTP server and blocks until ctx is cancelled or a fatal error occurs.
func (s *Server) Run(ctx context.Context) error {
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

// articleResponse is the JSON shape returned by /api/articles.
type articleResponse struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Summary string `json:"summary"`
	Image   string `json:"image"`
	Date    string `json:"date"`
}

func (s *Server) handleArticles(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// NOTE: No deduplication here — the API always returns the latest articles
	// so the web UI stays fresh across multiple calls.
	articles, err := s.fetcher.Fetch(s.cfg.FeedURL)
	if err != nil {
		s.logger.Error("Feed fetch failed", "error", err)
		http.Error(w, `{"error":"failed to fetch articles"}`, http.StatusInternalServerError)
		return
	}

	resp := make([]articleResponse, 0, len(articles))
	for _, a := range articles {
		resp = append(resp, articleResponse{
			Title:   a.Title,
			Link:    a.Link,
			Summary: a.Summary,
			Image:   a.ImageURL,
			Date:    a.Date.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("JSON encode failed", "error", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "bot": s.cfg.BotName})
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
