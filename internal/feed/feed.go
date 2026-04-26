// Package feed fetches and parses Habr RSS feeds with transparent caching.
package feed

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"habr-rss-bot/internal/cache"
)

// Article represents a single parsed RSS article.
type Article struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Summary     string    `json:"summary"`
	ImageURL    string    `json:"image"`
	HubID       string    `json:"hub"`
	GUID        string    `json:"-"`
	Date        time.Time `json:"date"`
	ReadingTime int       `json:"reading_time"` // estimated minutes, min 1
}

// Fetcher fetches RSS articles with an in-memory TTL cache.
// It has no deduplication side-effects — callers own that responsibility.
type Fetcher struct {
	parser   *gofeed.Parser
	cache    *cache.Cache[[]Article]
	maxItems int
}

var (
	htmlTagRe  = regexp.MustCompile(`<[^>]+>`)
	spaceRe    = regexp.MustCompile(`\s+`)
	imageSrcRe = regexp.MustCompile(`src="([^"]+)"`)
)

// New creates a Fetcher that returns at most maxItems articles and caches
// results for cacheTTL before re-fetching.
func New(maxItems int, cacheTTL time.Duration) *Fetcher {
	client := &http.Client{Timeout: 30 * time.Second}
	p := gofeed.NewParser()
	p.Client = client
	return &Fetcher{
		parser:   p,
		cache:    cache.New[[]Article](cacheTTL),
		maxItems: maxItems,
	}
}

// Fetch returns articles for the given hub, using the cache when fresh.
// hubID is used as the cache key and embedded in each returned Article.
func (f *Fetcher) Fetch(hubID, url string) ([]Article, error) {
	if cached, ok := f.cache.Get(hubID); ok {
		return cached, nil
	}

	feed, err := f.parser.ParseURL(url)
	if err != nil {
		return nil, err
	}

	limit := f.maxItems
	if limit <= 0 || limit > len(feed.Items) {
		limit = len(feed.Items)
	}

	articles := make([]Article, 0, limit)
	for i := 0; i < limit; i++ {
		item := feed.Items[i]
		pub := time.Now()
		if item.PublishedParsed != nil {
			pub = *item.PublishedParsed
		}
		clean := stripHTML(item.Description)
		articles = append(articles, Article{
			Title:       item.Title,
			Link:        item.Link,
			Summary:     trimSummary(clean, 220),
			ImageURL:    extractImage(item.Description),
			HubID:       hubID,
			GUID:        item.GUID,
			Date:        pub,
			ReadingTime: estimateReadingTime(clean),
		})
	}

	f.cache.Set(hubID, articles)
	return articles, nil
}

// InvalidateCache evicts hubID from the cache, forcing a fresh fetch next time.
func (f *Fetcher) InvalidateCache(hubID string) {
	f.cache.Delete(hubID)
}

// Search returns articles whose title or summary contain query (case-insensitive).
func Search(articles []Article, query string) []Article {
	q := strings.ToLower(query)
	out := make([]Article, 0)
	for _, a := range articles {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Summary), q) {
			out = append(out, a)
		}
	}
	return out
}

// ── internal helpers ───────────────────────────────────────────────────────

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func extractImage(description string) string {
	if m := imageSrcRe.FindStringSubmatch(description); len(m) == 2 {
		return m[1]
	}
	return ""
}

func trimSummary(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

// estimateReadingTime returns a rough reading-time in minutes at 160 wpm,
// which is a more accurate estimate for Russian technical content.
func estimateReadingTime(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 1
	}
	if m := words / 160; m >= 1 {
		return m
	}
	return 1
}
