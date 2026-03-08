package feed

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// Article represents a single parsed RSS article.
type Article struct {
	Title    string    `json:"title"`
	Link     string    `json:"link"`
	Summary  string    `json:"summary"`
	ImageURL string    `json:"image"`
	GUID     string    `json:"-"`
	Date     time.Time `json:"date"`
}

// Fetcher fetches and parses RSS articles.
type Fetcher struct {
	parser   *gofeed.Parser
	maxItems int
}

var (
	htmlTagRe  = regexp.MustCompile(`<[^>]+>`)
	spaceRe    = regexp.MustCompile(`\s+`)
	imageSrcRe = regexp.MustCompile(`src="([^"]+)"`)
)

// New creates a new Fetcher that returns at most maxItems articles per fetch.
func New(maxItems int) *Fetcher {
	client := &http.Client{Timeout: 30 * time.Second}
	parser := gofeed.NewParser()
	parser.Client = client
	return &Fetcher{parser: parser, maxItems: maxItems}
}

// Fetch retrieves articles from the given RSS URL.
// This function has NO side effects — it does not track which articles
// have been seen. Deduplication is the caller's responsibility.
func (f *Fetcher) Fetch(url string) ([]Article, error) {
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

		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}

		articles = append(articles, Article{
			Title:    item.Title,
			Link:     item.Link,
			Summary:  trimSummary(item.Description, 200),
			ImageURL: extractImage(item.Description),
			GUID:     item.GUID,
			Date:     pubDate,
		})
	}
	return articles, nil
}

// stripHTML removes all HTML tags and collapses whitespace.
func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// extractImage pulls the first src attribute out of an HTML description.
func extractImage(description string) string {
	if m := imageSrcRe.FindStringSubmatch(description); len(m) == 2 {
		return m[1]
	}
	return ""
}

// trimSummary strips HTML and limits text to maxLen Unicode characters.
func trimSummary(s string, maxLen int) string {
	s = stripHTML(s)
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
