package domain

import (
	"context"
)

// ArticleWorker defines the interface for the background article fetcher
type ArticleWorker interface {
	Start(ctx context.Context)
	FetchNow() error
}
