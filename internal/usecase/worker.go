package usecase

import (
	"context"
	"habr-rss-bot/internal/domain"
	"log"
	"sync"
	"time"
)

type articleWorker struct {
	articleUsecase domain.ArticleUsecase
	fetchInterval  time.Duration
}

func NewArticleWorker(u domain.ArticleUsecase, interval time.Duration) domain.ArticleWorker {
	return &articleWorker{
		articleUsecase: u,
		fetchInterval:  interval,
	}
}

func (w *articleWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.fetchInterval)
	defer ticker.Stop()

	// Initial fetch
	if err := w.articleUsecase.FetchAndSave(); err != nil {
		log.Printf("Worker: Initial fetch failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker: Stopping...")
			return
		case <-ticker.C:
			if err := w.articleUsecase.FetchAndSave(); err != nil {
				log.Printf("Worker: Periodic fetch failed: %v", err)
			}
		}
	}
}

func (w *articleWorker) FetchNow() error {
	return w.articleUsecase.FetchAndSave()
}

// Pool for parallel fetching if needed in the future
type WorkerPool struct {
	workers []domain.ArticleWorker
	wg      sync.WaitGroup
}
