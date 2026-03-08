package bot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/time/rate"

	"habr-rss-bot/internal/config"
	"habr-rss-bot/internal/feed"
)

type seenEntry struct {
	addedAt time.Time
}

// Bot is a Telegram bot that sends infosec articles from Habr RSS.
type Bot struct {
	api     *tgbotapi.BotAPI
	fetcher *feed.Fetcher
	limiter *rate.Limiter
	cfg     *config.Config
	logger  *slog.Logger

	seenMu sync.Mutex
	seen   map[string]seenEntry // per-instance deduplication
}

// New creates and authenticates a new Bot. Returns error if the token is invalid.
func New(cfg *config.Config, fetcher *feed.Fetcher, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("telegram auth: %w", err)
	}
	logger.Info("Telegram bot authenticated", "username", api.Self.UserName)

	return &Bot{
		api:     api,
		fetcher: fetcher,
		// 3 messages per second burst to avoid Telegram rate limits
		limiter: rate.NewLimiter(rate.Every(time.Second), 3),
		cfg:     cfg,
		logger:  logger,
		seen:    make(map[string]seenEntry),
	}, nil
}

// Run starts the bot update loop. It blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	go b.runCleanup(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := b.api.GetUpdatesChan(u)
	if err != nil {
		return fmt.Errorf("get updates channel: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message != nil {
				go b.handleMessage(update.Message)
			}
		}
	}
}

// runCleanup periodically removes expired entries from the seen map.
func (b *Bot) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.cleanup()
		}
	}
}

func (b *Bot) cleanup() {
	b.seenMu.Lock()
	defer b.seenMu.Unlock()
	now := time.Now()
	removed := 0
	for guid, entry := range b.seen {
		if now.Sub(entry.addedAt) > b.cfg.ArticleExpiry {
			delete(b.seen, guid)
			removed++
		}
	}
	b.logger.Debug("Cleanup complete", "removed", removed, "remaining", len(b.seen))
}

func (b *Bot) wasSeen(guid string) bool {
	b.seenMu.Lock()
	defer b.seenMu.Unlock()
	entry, ok := b.seen[guid]
	if !ok {
		return false
	}
	if time.Since(entry.addedAt) > b.cfg.ArticleExpiry {
		delete(b.seen, guid)
		return false
	}
	return true
}

func (b *Bot) markSeen(guid string) {
	b.seenMu.Lock()
	defer b.seenMu.Unlock()
	b.seen[guid] = seenEntry{addedAt: time.Now()}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if !b.limiter.Allow() {
		return
	}

	cmd := strings.TrimSpace(strings.ToLower(msg.Text))
	b.logger.Info("Message received", "chat_id", msg.Chat.ID, "text", cmd)

	switch cmd {
	case "/start":
		b.sendText(msg.Chat.ID,
			"Привет! Я агрегатор новостей информационной безопасности с Хабра.\n\n"+
				"Доступные команды:\n"+
				"/infosec — последние статьи по ИБ\n"+
				"/help — справка",
		)
	case "/help":
		b.sendText(msg.Chat.ID,
			"Команды:\n"+
				"/infosec или /security — последние статьи по ИБ\n"+
				"/help — это сообщение\n"+
				"/start — приветствие",
		)
	case "/infosec", "/security":
		b.handleInfoSec(msg.Chat.ID)
	default:
		b.sendText(msg.Chat.ID, "Неизвестная команда. Введите /help для справки.")
	}
}

func (b *Bot) handleInfoSec(chatID int64) {
	loading, err := b.api.Send(tgbotapi.NewMessage(chatID, "⏳ Загружаю статьи..."))
	if err != nil {
		b.logger.Warn("Failed to send loading message", "error", err)
	}

	articles, err := b.fetcher.Fetch(b.cfg.FeedURL)
	b.deleteMsg(chatID, loading.MessageID)
	if err != nil {
		b.logger.Error("Feed fetch error", "error", err)
		b.sendText(chatID, "Ошибка при получении статей. Попробуйте позже.")
		return
	}

	// Filter to only articles this bot instance hasn't sent yet
	var fresh []feed.Article
	for _, a := range articles {
		if !b.wasSeen(a.GUID) {
			fresh = append(fresh, a)
		}
	}

	if len(fresh) == 0 {
		b.sendText(chatID, "Новых статей пока нет. Попробуйте позже.")
		return
	}

	for _, a := range fresh {
		b.markSeen(a.GUID)
		text := fmt.Sprintf(
			"📚 <b>%s</b>\n\n%s\n\n🔗 <a href=\"%s\">Читать на Хабре</a>",
			html.EscapeString(a.Title),
			html.EscapeString(a.Summary),
			a.Link,
		)
		out := tgbotapi.NewMessage(chatID, text)
		out.ParseMode = "HTML"
		out.DisableWebPagePreview = true
		if _, err := b.api.Send(out); err != nil {
			b.logger.Error("Failed to send article", "title", a.Title, "error", err)
			continue
		}
		time.Sleep(300 * time.Millisecond) // stay well within Telegram rate limits
	}
}

func (b *Bot) sendText(chatID int64, text string) {
	if _, err := b.api.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		b.logger.Error("sendText failed", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) deleteMsg(chatID int64, msgID int) {
	if msgID == 0 {
		return
	}
	if _, err := b.api.Send(tgbotapi.NewDeleteMessage(chatID, msgID)); err != nil {
		b.logger.Warn("deleteMsg failed", "msg_id", msgID, "error", err)
	}
}
