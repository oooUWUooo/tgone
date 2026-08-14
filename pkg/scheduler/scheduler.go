package scheduler

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"habr-rss-bot/internal/models"
	"habr-rss-bot/pkg/secrets"
)

// SchedulerConfig holds configuration for the scheduler
type SchedulerConfig struct {
	FetchIntervalMinutes int
	CleanupIntervalHours int
	BackoffMultiplier    float64
	MaxRetries           int
	RateLimitPerMinute   int
}

// Task represents a scheduled task
type Task struct {
	ID          string
	Name        string
	Handler     func(context.Context) error
	Interval    time.Duration
	LastRun     time.Time
	NextRun     time.Time
	RetryCount  int
	MaxRetries  int
	Enabled     bool
	mux         sync.Mutex
}

// Scheduler manages scheduled tasks with smart backoff and rate limiting
type Scheduler struct {
	mux           sync.RWMutex
	tasks         map[string]*Task
	config        *SchedulerConfig
	secretsStore  *secrets.SecretStore
	rateLimiter   *RateLimiter
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mux          sync.Mutex
	tokens       float64
	maxTokens    float64
	refillRate   float64 // tokens per second
	lastRefill   time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(tokensPerMinute int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(tokensPerMinute),
		maxTokens:  float64(tokensPerMinute),
		refillRate: float64(tokensPerMinute) / 60.0,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under rate limiting
func (rl *RateLimiter) Allow() bool {
	rl.mux.Lock()
	defer rl.mux.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens = math.Min(rl.maxTokens, rl.tokens+elapsed*rl.refillRate)
	rl.lastRefill = now

	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}
	return false
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for !rl.Allow() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}

// NewScheduler creates a new scheduler
func NewScheduler(config *SchedulerConfig, secretsStore *secrets.SecretStore) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		tasks:        make(map[string]*Task),
		config:       config,
		secretsStore: secretsStore,
		rateLimiter:  NewRateLimiter(config.RateLimitPerMinute),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// AddTask adds a new scheduled task
func (s *Scheduler) AddTask(id, name string, handler func(context.Context) error, interval time.Duration) {
	s.mux.Lock()
	defer s.mux.Unlock()

	task := &Task{
		ID:         id,
		Name:       name,
		Handler:    handler,
		Interval:   interval,
		NextRun:    time.Now().Add(interval),
		MaxRetries: s.config.MaxRetries,
		Enabled:    true,
	}

	s.tasks[id] = task
	log.Printf("📅 Scheduled task '%s' (ID: %s) to run every %v", name, id, interval)
}

// RemoveTask removes a scheduled task
func (s *Scheduler) RemoveTask(id string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	delete(s.tasks, id)
	log.Printf("❌ Removed task ID: %s", id)
}

// EnableTask enables a disabled task
func (s *Scheduler) EnableTask(id string) error {
	s.mux.RLock()
	task, exists := s.tasks[id]
	s.mux.RUnlock()

	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.mux.Lock()
	task.Enabled = true
	task.NextRun = time.Now().Add(task.Interval)
	task.mux.Unlock()

	log.Printf("✅ Enabled task: %s", id)
	return nil
}

// DisableTask disables a task without removing it
func (s *Scheduler) DisableTask(id string) error {
	s.mux.RLock()
	task, exists := s.tasks[id]
	s.mux.RUnlock()

	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.mux.Lock()
	task.Enabled = false
	task.mux.Unlock()

	log.Printf("⏸️ Disabled task: %s", id)
	return nil
}

// Start starts all scheduled tasks
func (s *Scheduler) Start() {
	log.Println("🚀 Starting scheduler...")

	s.mux.RLock()
	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.mux.RUnlock()

	for _, task := range tasks {
		s.wg.Add(1)
		go s.runTaskLoop(task)
	}

	log.Printf("✅ Scheduler started with %d tasks", len(tasks))
}

// Stop gracefully stops all scheduled tasks
func (s *Scheduler) Stop() {
	log.Println("🛑 Stopping scheduler...")

	s.cancel()
	s.wg.Wait()
	log.Println("✅ Scheduler stopped")
}

func (s *Scheduler) runTaskLoop(task *Task) {
	defer s.wg.Done()

	timer := time.NewTimer(time.Until(task.NextRun))
	defer timer.Stop()

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("🔴 Stopping task loop for '%s'", task.Name)
			return

		case <-timer.C:
			task.mux.Lock()
			if !task.Enabled {
				task.mux.Unlock()
				timer.Reset(task.Interval)
				continue
			}
			task.mux.Unlock()

			// Apply rate limiting
			if err := s.rateLimiter.Wait(s.ctx); err != nil {
				log.Printf("⚠️ Rate limiter interrupted: %v", err)
				return
			}

			// Execute task with retry logic
			success := s.executeWithRetry(task)

			task.mux.Lock()
			task.LastRun = time.Now()
			if success {
				task.RetryCount = 0
				task.NextRun = time.Now().Add(task.Interval)
			} else {
				// Apply exponential backoff
				backoff := s.calculateBackoff(task.RetryCount)
				task.NextRun = time.Now().Add(backoff)
				log.Printf("⚠️ Task '%s' will retry in %v after %d failures", task.Name, backoff, task.RetryCount)
			}
			nextRun := task.NextRun
			task.mux.Unlock()

			log.Printf("📊 Task '%s' next run: %v", task.Name, nextRun.Format(time.RFC3339))
			timer.Reset(time.Until(nextRun))
		}
	}
}

func (s *Scheduler) executeWithRetry(task *Task) bool {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	for attempt := 1; attempt <= task.MaxRetries; attempt++ {
		task.mux.Lock()
		task.RetryCount = attempt
		task.mux.Unlock()

		err := task.Handler(ctx)
		if err == nil {
			log.Printf("✅ Task '%s' completed successfully on attempt %d", task.Name, attempt)
			return true
		}

		log.Printf("❌ Task '%s' failed (attempt %d/%d): %v", task.Name, attempt, task.MaxRetries, err)

		if attempt < task.MaxRetries {
			backoff := s.calculateBackoff(attempt)
			log.Printf("⏳ Waiting %v before retry...", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return false
			}
		}
	}

	return false
}

func (s *Scheduler) calculateBackoff(retryCount int) time.Duration {
	baseDelay := 30 * time.Second
	multiplier := s.config.BackoffMultiplier

	// Exponential backoff with jitter
	delay := float64(baseDelay) * math.Pow(multiplier, float64(retryCount-1))

	// Add jitter (±20%)
	jitter := delay * 0.2 * (rand.Float64()*2 - 1)
	delay += jitter

	// Cap at 1 hour
	if delay > float64(time.Hour) {
		delay = float64(time.Hour)
	}

	return time.Duration(delay)
}

// GetTaskStatus returns the status of a task
func (s *Scheduler) GetTaskStatus(id string) (map[string]interface{}, error) {
	s.mux.RLock()
	task, exists := s.tasks[id]
	s.mux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	task.mux.Lock()
	defer task.mux.Unlock()

	return map[string]interface{}{
		"id":          task.ID,
		"name":        task.Name,
		"enabled":     task.Enabled,
		"interval":    task.Interval.String(),
		"last_run":    task.LastRun.Format(time.RFC3339),
		"next_run":    task.NextRun.Format(time.RFC3339),
		"retry_count": task.RetryCount,
		"max_retries": task.MaxRetries,
	}, nil
}

// GetAllTaskStatuses returns statuses of all tasks
func (s *Scheduler) GetAllTaskStatuses() []map[string]interface{} {
	s.mux.RLock()
	defer s.mux.RUnlock()

	statuses := make([]map[string]interface{}, 0, len(s.tasks))
	for _, task := range s.tasks {
		task.mux.Lock()
		status := map[string]interface{}{
			"id":          task.ID,
			"name":        task.Name,
			"enabled":     task.Enabled,
			"interval":    task.Interval.String(),
			"last_run":    task.LastRun.Format(time.RFC3339),
			"next_run":    task.NextRun.Format(time.RFC3339),
			"retry_count": task.RetryCount,
			"max_retries": task.MaxRetries,
		}
		task.mux.Unlock()
		statuses = append(statuses, status)
	}

	return statuses
}

// CreateFetchArticlesTask creates a standard task for fetching articles
func (s *Scheduler) CreateFetchArticlesTask(handler func(context.Context) ([]models.Article, error)) {
	s.AddTask("fetch_articles", "Fetch Articles from RSS Feeds", func(ctx context.Context) error {
		_, err := handler(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch articles: %w", err)
		}
		return nil
	}, time.Duration(s.config.FetchIntervalMinutes)*time.Minute)
}

// CreateCleanupTask creates a standard task for cleaning up expired data
func (s *Scheduler) CreateCleanupTask(handler func(context.Context) (int, error)) {
	s.AddTask("cleanup_expired", "Cleanup Expired Articles and Cache", func(ctx context.Context) error {
		removed, err := handler(ctx)
		if err != nil {
			return fmt.Errorf("failed to cleanup: %w", err)
		}
		log.Printf("🧹 Cleaned up %d expired items", removed)
		return nil
	}, time.Duration(s.config.CleanupIntervalHours)*time.Hour)
}

// CreateBackupTask creates a standard task for backing up data
func (s *Scheduler) CreateBackupTask(handler func(context.Context) error) {
	s.AddTask("backup_data", "Backup Database and Configuration", func(ctx context.Context) error {
		if err := handler(ctx); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		log.Println("💾 Backup completed successfully")
		return nil
	}, 24*time.Hour)
}
