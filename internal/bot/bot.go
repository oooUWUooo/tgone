// Package bot implements the Telegram bot with subscriptions and background polling.
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
	"habr-rss-bot/internal/hub"
	"habr-rss-bot/internal/storage"
)

// notifyEntry records when an article was last pushed to a specific chat.
type notifyEntry struct{ sentAt time.Time }

// Bot handles Telegram updates and proactively pushes new articles to subscribers.
type Bot struct {
	api     *tgbotapi.BotAPI
	fetcher *feed.Fetcher
	limiter *rate.Limiter
	cfg     *config.Config
	logger  *slog.Logger
	store   *storage.Store

	// notified maps chatID → guid → when it was sent.
	// This is separate from storage.Store because it is purely in-memory
	// and can be rebuilt after a restart without harm.
	notifyMu sync.Mutex
	notified map[int64]map[string]notifyEntry
}

// New authenticates with the Telegram API and returns a ready Bot.
func New(cfg *config.Config, fetcher *feed.Fetcher, store *storage.Store, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("telegram auth: %w", err)
	}
	logger.Info("Telegram bot authenticated", "username", api.Self.UserName)

	return &Bot{
		api:      api,
		fetcher:  fetcher,
		limiter:  rate.NewLimiter(rate.Every(time.Second), 3),
		cfg:      cfg,
		logger:   logger,
		store:    store,
		notified: make(map[int64]map[string]notifyEntry),
	}, nil
}

// Run starts the update loop, background poller, and cleanup goroutine.
// It blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	go b.runPoller(ctx)
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

// ── Background poller ──────────────────────────────────────────────────────

func (b *Bot) runPoller(ctx context.Context) {
	ticker := time.NewTicker(b.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.poll(ctx)
		}
	}
}

// poll checks every hub that has at least one subscriber and pushes new articles.
func (b *Bot) poll(ctx context.Context) {
	subs := b.store.All()
	if len(subs) == 0 {
		return
	}

	// Collect unique hub IDs across all subscribers
	hubSet := make(map[string]struct{})
	for _, sub := range subs {
		for _, hid := range sub.HubIDs {
			hubSet[hid] = struct{}{}
		}
	}

	// Fetch each required hub (cheap when cache is warm)
	results := make(map[string][]feed.Article, len(hubSet))
	for hid := range hubSet {
		h, ok := hub.ByID(hid)
		if !ok {
			continue
		}
		articles, err := b.fetcher.Fetch(hid, h.URL)
		if err != nil {
			b.logger.Warn("Poll: fetch failed", "hub", hid, "error", err)
			continue
		}
		results[hid] = articles
	}

	// Push new articles to each subscriber
	for _, sub := range subs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		for _, hid := range sub.HubIDs {
			if articles, ok := results[hid]; ok {
				b.pushNew(sub.ChatID, hid, articles)
			}
		}
	}
}

// pushNew sends articles not yet seen by chatID for the given hub.
func (b *Bot) pushNew(chatID int64, hubID string, articles []feed.Article) {
	fresh := b.filterUnsent(chatID, articles)
	if len(fresh) == 0 {
		return
	}
	b.logger.Info("Pushing new articles", "chat_id", chatID, "hub", hubID, "count", len(fresh))

	h, _ := hub.ByID(hubID)
	header := fmt.Sprintf("%s <b>Новые статьи — %s</b>", h.Emoji, html.EscapeString(h.Name))
	msg := tgbotapi.NewMessage(chatID, header)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
	time.Sleep(200 * time.Millisecond)

	for _, a := range fresh {
		b.markNotified(chatID, a.GUID)
		if err := b.sendArticle(chatID, a); err != nil {
			b.logger.Error("pushNew: send failed", "error", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func (b *Bot) filterUnsent(chatID int64, articles []feed.Article) []feed.Article {
	b.notifyMu.Lock()
	defer b.notifyMu.Unlock()
	chat, ok := b.notified[chatID]
	if !ok {
		// Chat has no initialized notified map yet — nothing is "fresh" by definition.
		// (initNotified must be called first on subscribe.)
		return nil
	}
	var out []feed.Article
	for _, a := range articles {
		if _, sent := chat[a.GUID]; !sent {
			out = append(out, a)
		}
	}
	return out
}

func (b *Bot) markNotified(chatID int64, guid string) {
	b.notifyMu.Lock()
	defer b.notifyMu.Unlock()
	if b.notified[chatID] == nil {
		b.notified[chatID] = make(map[string]notifyEntry)
	}
	b.notified[chatID][guid] = notifyEntry{sentAt: time.Now()}
}

// initNotified pre-populates current articles as "already sent" so a fresh
// subscriber doesn't receive a wall of old articles on first poll.
func (b *Bot) initNotified(chatID int64, hubIDs []string) {
	b.notifyMu.Lock()
	defer b.notifyMu.Unlock()
	if b.notified[chatID] == nil {
		b.notified[chatID] = make(map[string]notifyEntry)
	}
	for _, hid := range hubIDs {
		h, ok := hub.ByID(hid)
		if !ok {
			continue
		}
		articles, err := b.fetcher.Fetch(hid, h.URL)
		if err != nil {
			continue
		}
		for _, a := range articles {
			b.notified[chatID][a.GUID] = notifyEntry{sentAt: time.Now()}
		}
	}
}

// ── Cleanup ────────────────────────────────────────────────────────────────

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
	b.notifyMu.Lock()
	defer b.notifyMu.Unlock()
	cutoff := time.Now().Add(-b.cfg.ArticleExpiry)
	for chatID, entries := range b.notified {
		for guid, e := range entries {
			if e.sentAt.Before(cutoff) {
				delete(entries, guid)
			}
		}
		if len(entries) == 0 {
			delete(b.notified, chatID)
		}
	}
}

// ── Command handling ───────────────────────────────────────────────────────

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if !b.limiter.Allow() {
		b.sendText(msg.Chat.ID, "⏱ Слишком много запросов. Подождите секунду и попробуйте снова.")
		return
	}

	raw := strings.TrimSpace(msg.Text)
	b.logger.Info("Message received", "chat_id", msg.Chat.ID, "text", raw)

	// Split command from optional argument: "/hub devops" → cmd="/hub", arg="devops"
	parts := strings.SplitN(raw, " ", 2)
	cmd := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/start":
		b.handleStart(msg.Chat.ID)
	case "/help":
		b.handleHelp(msg.Chat.ID)
	case "/infosec", "/security":
		b.handleFeed(msg.Chat.ID, "infosec")
	case "/hubs":
		b.handleHubs(msg.Chat.ID)
	case "/hub":
		if arg == "" {
			b.sendText(msg.Chat.ID, "Укажите хаб: /hub infosec\nСписок хабов: /hubs")
		} else {
			b.handleFeed(msg.Chat.ID, arg)
		}
	case "/search":
		if arg == "" {
			b.sendText(msg.Chat.ID, "Укажите запрос: /search уязвимость")
		} else {
			b.handleSearch(msg.Chat.ID, arg)
		}
	case "/subscribe":
		b.handleSubscribe(msg.Chat.ID, arg)
	case "/unsubscribe":
		b.handleUnsubscribe(msg.Chat.ID)
	case "/status":
		b.handleStatus(msg.Chat.ID)
	default:
		b.sendText(msg.Chat.ID, "Неизвестная команда. Введите /help для справки.")
	}
}

func (b *Bot) handleStart(chatID int64) {
	b.sendHTML(chatID,
		"👋 <b>Привет! Я агрегатор новостей с Хабра.</b>\n\n"+
			"Читаю хабы: ИБ, DevOps, Linux, веб-разработку, Go, Python, ML и другие.\n\n"+
			"<b>Основные команды:</b>\n"+
			"/infosec — статьи по информационной безопасности\n"+
			"/hubs — список всех доступных хабов\n"+
			"/hub <i>devops</i> — статьи из конкретного хаба\n"+
			"/search <i>запрос</i> — поиск по заголовкам и текстам\n"+
			"/subscribe — подписаться на авто-обновления\n"+
			"/unsubscribe — отписаться\n"+
			"/status — статус вашей подписки\n"+
			"/help — справка",
	)
}

func (b *Bot) handleHelp(chatID int64) {
	b.sendHTML(chatID,
		"<b>Команды:</b>\n\n"+
			"/infosec — статьи по ИБ (сокращение для /hub infosec)\n"+
			"/hubs — список всех хабов\n"+
			"/hub <i>id</i> — статьи хаба\n"+
			"  доступные id: infosec, devops, webdev, programming,\n"+
			"  sysadm, linux, golang, python, machine_learning\n\n"+
			"/search <i>запрос</i> — поиск по всем хабам\n\n"+
			"/subscribe [<i>hub1 hub2 ...</i>] — подписка на авто-обновления\n"+
			"  без аргументов — подписка на infosec\n"+
			"/unsubscribe — отписаться\n"+
			"/status — проверить статус подписки\n\n"+
			"/help — это сообщение",
	)
}

func (b *Bot) handleHubs(chatID int64) {
	lines := []string{"<b>Доступные хабы:</b>\n"}
	for _, h := range hub.All {
		lines = append(lines, fmt.Sprintf("%s <code>/hub %s</code> — %s", h.Emoji, h.ID, h.Name))
	}
	b.sendHTML(chatID, strings.Join(lines, "\n"))
}

func (b *Bot) handleFeed(chatID int64, hubID string) {
	h, ok := hub.ByID(hubID)
	if !ok {
		b.sendText(chatID, fmt.Sprintf("Хаб «%s» не найден. Список: /hubs", hubID))
		return
	}

	loading, _ := b.api.Send(tgbotapi.NewMessage(chatID, "⏳ Загружаю статьи..."))
	articles, err := b.fetcher.Fetch(h.ID, h.URL)
	b.deleteMsg(chatID, loading.MessageID)

	if err != nil {
		b.logger.Error("Feed fetch error", "hub", hubID, "error", err)
		b.sendText(chatID, "Ошибка при получении статей. Попробуйте позже.")
		return
	}
	if len(articles) == 0 {
		b.sendText(chatID, "Статей пока нет.")
		return
	}

	header := fmt.Sprintf("%s <b>%s</b> — %d статей", h.Emoji, html.EscapeString(h.Name), len(articles))
	msg := tgbotapi.NewMessage(chatID, header)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
	time.Sleep(150 * time.Millisecond)

	for _, a := range articles {
		if err := b.sendArticle(chatID, a); err != nil {
			b.logger.Error("Send article failed", "error", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func (b *Bot) handleSearch(chatID int64, query string) {
	loading, _ := b.api.Send(tgbotapi.NewMessage(chatID,
		fmt.Sprintf("🔍 Ищу «%s» по всем хабам...", query)))

	// Aggregate articles from all hubs (all cached — cheap)
	var all []feed.Article
	for _, h := range hub.All {
		articles, err := b.fetcher.Fetch(h.ID, h.URL)
		if err != nil {
			continue
		}
		all = append(all, articles...)
	}
	b.deleteMsg(chatID, loading.MessageID)

	results := feed.Search(all, query)
	if len(results) == 0 {
		b.sendText(chatID, fmt.Sprintf("По запросу «%s» ничего не найдено.", query))
		return
	}

	header := fmt.Sprintf("🔍 <b>«%s»</b> — найдено %d статей",
		html.EscapeString(query), len(results))
	msg := tgbotapi.NewMessage(chatID, header)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
	time.Sleep(150 * time.Millisecond)

	limit := 5
	if len(results) < limit {
		limit = len(results)
	}
	for _, a := range results[:limit] {
		if err := b.sendArticle(chatID, a); err != nil {
			b.logger.Error("Send search result failed", "error", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	if len(results) > limit {
		b.sendText(chatID, fmt.Sprintf("...и ещё %d статей. Уточните запрос.", len(results)-limit))
	}
}

func (b *Bot) handleSubscribe(chatID int64, arg string) {
	hubIDs := []string{hub.DefaultHub.ID}

	if arg != "" {
		parts := strings.Fields(arg)
		valid := make([]string, 0, len(parts))
		unknown := make([]string, 0)
		for _, p := range parts {
			if _, ok := hub.ByID(p); ok {
				valid = append(valid, p)
			} else {
				unknown = append(unknown, p)
			}
		}
		if len(unknown) > 0 {
			b.sendText(chatID, fmt.Sprintf("Неизвестные хабы: %s\nСписок хабов: /hubs",
				strings.Join(unknown, ", ")))
			if len(valid) == 0 {
				return
			}
		}
		if len(valid) > 0 {
			hubIDs = valid
		}
	}

	// Pre-populate current articles so the subscriber doesn't get a wall of old posts
	b.initNotified(chatID, hubIDs)
	b.store.Subscribe(chatID, hubIDs)

	names := make([]string, 0, len(hubIDs))
	for _, hid := range hubIDs {
		if h, ok := hub.ByID(hid); ok {
			names = append(names, h.Emoji+" "+h.Name)
		}
	}
	b.sendHTML(chatID, fmt.Sprintf(
		"✅ <b>Подписка активна!</b>\n\nХабы: %s\n\nНовые статьи приходят каждые ~%d мин.\nОтписаться: /unsubscribe",
		strings.Join(names, ", "),
		int(b.cfg.PollInterval.Minutes()),
	))
}

func (b *Bot) handleUnsubscribe(chatID int64) {
	if b.store.Unsubscribe(chatID) {
		b.notifyMu.Lock()
		delete(b.notified, chatID)
		b.notifyMu.Unlock()
		b.sendText(chatID, "✅ Вы отписались от авто-обновлений.")
	} else {
		b.sendText(chatID, "Вы не были подписаны.")
	}
}

func (b *Bot) handleStatus(chatID int64) {
	sub := b.store.Get(chatID)
	if sub == nil {
		b.sendHTML(chatID,
			"❌ <b>Подписка не активна.</b>\n\n"+
				"Чтобы подписаться: /subscribe\n"+
				"Список хабов: /hubs",
		)
		return
	}
	names := make([]string, 0, len(sub.HubIDs))
	for _, hid := range sub.HubIDs {
		if h, ok := hub.ByID(hid); ok {
			names = append(names, h.Emoji+" "+h.Name)
		}
	}
	b.sendHTML(chatID, fmt.Sprintf(
		"✅ <b>Подписка активна</b>\n\n"+
			"Хабы: %s\n\n"+
			"Активна с: %s\n"+
			"Обновления каждые ~%d мин.\n\n"+
			"Отписаться: /unsubscribe",
		strings.Join(names, ", "),
		sub.CreatedAt.Format("02.01.2006"),
		int(b.cfg.PollInterval.Minutes()),
	))
}

// ── Helpers ────────────────────────────────────────────────────────────────

func (b *Bot) sendArticle(chatID int64, a feed.Article) error {
	readTime := ""
	if a.ReadingTime > 0 {
		readTime = fmt.Sprintf("\n🕐 %d мин чтения", a.ReadingTime)
	}
	text := fmt.Sprintf(
		"📚 <b>%s</b>\n\n%s\n\n🔗 <a href=\"%s\">Читать на Хабре</a>%s",
		html.EscapeString(a.Title),
		html.EscapeString(a.Summary),
		a.Link,
		readTime,
	)
	out := tgbotapi.NewMessage(chatID, text)
	out.ParseMode = "HTML"
	out.DisableWebPagePreview = true
	_, err := b.api.Send(out)
	return err
}

func (b *Bot) sendText(chatID int64, text string) {
	if _, err := b.api.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		b.logger.Error("sendText failed", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) sendHTML(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("sendHTML failed", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) deleteMsg(chatID int64, msgID int) {
	if msgID == 0 {
		return
	}
	if _, err := b.api.Send(tgbotapi.NewDeleteMessage(chatID, msgID)); err != nil {
		b.logger.Warn("deleteMsg failed", "msg_id", msgID)
	}
}
