package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"habr-rss-bot/internal/cache"
	"habr-rss-bot/internal/config"
	"habr-rss-bot/internal/models"
)

// RSSService handles RSS feed operations
type RSSService struct {
	parser      *gofeed.Parser
	httpClient  *http.Client
	cache       *cache.ArticleCache
	config      *config.Config
	sentArticles map[string]time.Time
	mux         sync.RWMutex
}

// NewRSSService creates a new RSS service
func NewRSSService(cfg *config.Config) *RSSService {
	return &RSSService{
		parser: gofeed.NewParser(),
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
		cache:        cache.NewArticleCache(),
		config:       cfg,
		sentArticles: make(map[string]time.Time),
	}
}

// FetchArticles fetches articles from the Habr RSS feed
func (s *RSSService) FetchArticles(ctx context.Context) ([]models.Article, error) {
	// Check cache first
	if cached, found := s.cache.Get("latest_articles"); found {
		if articles, ok := cached.([]models.Article); ok {
			return articles, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", s.config.HabrRSSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	feed, err := s.parser.ParseString(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	var articles []models.Article
	for _, item := range feed.Items {
		// Skip already sent articles
		if s.wasArticleSent(item.GUID) {
			continue
		}

		// Mark as sent
		s.markArticleAsSent(item.GUID)

		// Parse publication date
		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}

		// Extract image URL from description
		imageURL := extractImageURL(item.Description)

		article := models.Article{
			Title:       item.Title,
			Link:        item.Link,
			Summary:     trimSummary(item.Description),
			ImageURL:    imageURL,
			Date:        pubDate,
			GUID:        item.GUID,
			Description: item.Description,
		}

		articles = append(articles, article)

		// Limit number of articles
		if len(articles) >= s.config.MaxArticles {
			break
		}
	}

	// Cache the results for 5 minutes
	if len(articles) > 0 {
		s.cache.Set("latest_articles", articles, 5*time.Minute)
	}

	return articles, nil
}

func (s *RSSService) wasArticleSent(guid string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()

	timestamp, exists := s.sentArticles[guid]
	if !exists {
		return false
	}

	// Check if article has expired
	if time.Since(timestamp) > s.config.ArticleExpiry {
		return false
	}

	return true
}

func (s *RSSService) markArticleAsSent(guid string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.sentArticles[guid] = time.Now()
}

// CleanupExpiredArticles removes expired articles from tracking
func (s *RSSService) CleanupExpiredArticles() int {
	s.mux.Lock()
	defer s.mux.Unlock()

	count := 0
	now := time.Now()
	for guid, timestamp := range s.sentArticles {
		if now.Sub(timestamp) > s.config.ArticleExpiry {
			delete(s.sentArticles, guid)
			count++
		}
	}

	return count
}

// StartCleanupRoutine starts periodic cleanup of expired articles
func (s *RSSService) StartCleanupRoutine(done <-chan struct{}) {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			expired := s.CleanupExpiredArticles()
			cacheExpired := s.cache.Cleanup()
			fmt.Printf("[RSS Service] Cleaned up %d expired articles and %d cache items\n", expired, cacheExpired)
		case <-done:
			return
		}
	}
}

func extractImageURL(description string) string {
	if !strings.Contains(description, "<img") {
		return ""
	}

	start := strings.Index(description, `src="`)
	if start == -1 {
		start = strings.Index(description, `src='`)
		if start == -1 {
			return ""
		}
		start += 5
		quote := "'"
		end := strings.Index(description[start:], quote)
		if end == -1 {
			return ""
		}
		return description[start : start+end]
	}

	start += 5
	end := strings.Index(description[start:], `"`)
	if end == -1 {
		return ""
	}

	return description[start : start+end]
}

func trimSummary(summary string) string {
	// Remove all HTML tags
	for strings.Contains(summary, "<") {
		start := strings.Index(summary, "<")
		end := strings.Index(summary[start:], ">")
		if end != -1 {
			summary = summary[:start] + " " + summary[start+end+1:]
		} else {
			break
		}
	}

	// Remove extra spaces
	fields := strings.Fields(summary)
	summary = strings.Join(fields, " ")

	// Limit to 200 characters
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}

	return summary
}
