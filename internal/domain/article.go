package domain

import (
	"time"
)

// Article represents a news item from an RSS feed
type Article struct {
	ID        int64     `json:"id"`
	GUID      string    `json:"guid" db:"guid"`
	Title     string    `json:"title" db:"title"`
	Link      string    `json:"link" db:"link"`
	Summary   string    `json:"summary" db:"summary"`
	ImageURL  string    `json:"image_url" db:"image_url"`
	PubDate   time.Time `json:"pub_date" db:"pub_date"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	SourceID  int64     `json:"source_id" db:"source_id"`
}

// Source represents an RSS feed source
type Source struct {
	ID   int64  `json:"id"`
	Name string `json:"name" db:"name"`
	URL  string `json:"url" db:"url"`
	Category string `json:"category" db:"category"`
}

// ArticleRepository defines the interface for article data operations
type ArticleRepository interface {
	Create(article *Article) error
	GetLatest(limit int) ([]Article, error)
	GetByCategory(category string, limit int) ([]Article, error)
	Exists(guid string) (bool, error)
	DeleteOld(olderThan time.Time) error
}

// SourceRepository defines the interface for source data operations
type SourceRepository interface {
	GetAll() ([]Source, error)
	GetByID(id int64) (*Source, error)
	Create(source *Source) error
}

// ArticleUsecase defines the business logic for articles
type ArticleUsecase interface {
	FetchAndSave() error
	GetLatestArticles(limit int) ([]Article, error)
	GetArticlesByCategory(category string, limit int) ([]Article, error)
}
