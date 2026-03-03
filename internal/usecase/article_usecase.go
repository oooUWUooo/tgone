package usecase

import (
	"context"
	"habr-rss-bot/internal/domain"
	"log"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

type articleUsecase struct {
	articleRepo domain.ArticleRepository
	sourceRepo  domain.SourceRepository
	fp          *gofeed.Parser
	timeout     time.Duration
}

func NewArticleUsecase(a domain.ArticleRepository, s domain.SourceRepository, timeout time.Duration) domain.ArticleUsecase {
	return &articleUsecase{
		articleRepo: a,
		sourceRepo:  s,
		fp:          gofeed.NewParser(),
		timeout:     timeout,
	}
}

func (u *articleUsecase) FetchAndSave() error {
	sources, err := u.sourceRepo.GetAll()
	if err != nil {
		return err
	}

	for _, source := range sources {
		ctx, cancel := context.WithTimeout(context.Background(), u.timeout)
		feed, err := u.fp.ParseURLWithContext(source.URL, ctx)
		cancel()
		if err != nil {
			log.Printf("Error parsing feed %s: %v", source.URL, err)
			continue
		}

		for _, item := range feed.Items {
			exists, err := u.articleRepo.Exists(item.GUID)
			if err != nil {
				log.Printf("Error checking existence of %s: %v", item.GUID, err)
				continue
			}

			if exists {
				continue
			}

			pubDate := time.Now()
			if item.PublishedParsed != nil {
				pubDate = *item.PublishedParsed
			}

			imageURL := u.extractImage(item.Description)

			article := &domain.Article{
				GUID:     item.GUID,
				Title:    item.Title,
				Link:     item.Link,
				Summary:  u.trimSummary(item.Description),
				ImageURL: imageURL,
				PubDate:  pubDate,
				SourceID: source.ID,
			}

			if err := u.articleRepo.Create(article); err != nil {
				log.Printf("Error saving article %s: %v", article.Title, err)
			}
		}
	}

	return nil
}

func (u *articleUsecase) GetLatestArticles(limit int) ([]domain.Article, error) {
	return u.articleRepo.GetLatest(limit)
}

func (u *articleUsecase) GetArticlesByCategory(category string, limit int) ([]domain.Article, error) {
	return u.articleRepo.GetByCategory(category, limit)
}

func (u *articleUsecase) extractImage(description string) string {
	if strings.Contains(description, "<img") {
		start := strings.Index(description, "src=\"")
		if start != -1 {
			start += 5
			end := strings.Index(description[start:], "\"")
			if end != -1 {
				return description[start : start+end]
			}
		}
	}
	return ""
}

func (u *articleUsecase) trimSummary(summary string) string {
	// Remove ALL HTML tags
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
	summary = strings.Join(strings.Fields(summary), " ")

	// Limit to 200 characters
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}

	return summary
}
