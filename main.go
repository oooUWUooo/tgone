package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/mmcdole/gofeed"
	"golang.org/x/time/rate"
)

type Article struct {
	Title   string
	Link    string
	Summary string
	Date    time.Time
}

type Bot struct {
	bot         *tgbotapi.BotAPI
	fp          *gofeed.Parser
	limiter     *rate.Limiter
	articles    map[string]bool // to track sent articles
	articlesMux sync.RWMutex    // mutex to protect articles map
	httpClient  *http.Client    // HTTP client with timeout
	articleExpiry time.Duration // How long to keep articles in memory (e.g., 24 hours)
	articleTimestamps map[string]time.Time // Track when articles were added
}

func NewBot(token string) *Bot {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	return &Bot{
		bot:      bot,
		fp:       gofeed.NewParser(),
		limiter:  rate.NewLimiter(rate.Every(1*time.Second), 1),
		articles: make(map[string]bool),
		articleTimestamps: make(map[string]time.Time),
		articleExpiry: 24 * time.Hour, // Keep articles for 24 hours
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (b *Bot) Start() {
	log.Printf("Authorized on account %s", b.bot.Self.UserName)

	// Start periodic cleanup of expired articles
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
		defer ticker.Stop()
		for range ticker.C {
			b.cleanupExpiredArticles()
			log.Println("Cleaned up expired articles")
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := b.bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}

	for update := range updates {
		if update.Message != nil {
			go b.handleMessage(update.Message)
		}
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if !b.limiter.Allow() {
		return
	}

	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	switch text {
	case "/start":
		b.sendWelcomeMessage(chatID)
	case "/help":
		b.sendHelpMessage(chatID)
	case "/infosec", "/security":
		b.sendInfoSecFeed(chatID)
	default:
		b.sendWelcomeMessage(chatID)
	}
}

// Safe method to check if an article was already sent
func (b *Bot) wasArticleSent(guid string) bool {
	b.articlesMux.Lock() // Need write lock because we might cleanup
	defer b.articlesMux.Unlock()
	
	// Check if article exists
	if exists, ok := b.articles[guid]; ok && exists {
		// Check if the article has expired
		if time.Since(b.articleTimestamps[guid]) > b.articleExpiry {
			// Remove expired article
			delete(b.articles, guid)
			delete(b.articleTimestamps, guid)
			return false
		}
		return true
	}
	return false
}

// Safe method to mark an article as sent
func (b *Bot) markArticleAsSent(guid string) {
	b.articlesMux.Lock()
	defer b.articlesMux.Unlock()
	
	b.articles[guid] = true
	b.articleTimestamps[guid] = time.Now()
}

// Clean up expired articles periodically
func (b *Bot) cleanupExpiredArticles() {
	b.articlesMux.Lock()
	defer b.articlesMux.Unlock()
	
	now := time.Now()
	for guid, timestamp := range b.articleTimestamps {
		if now.Sub(timestamp) > b.articleExpiry {
			delete(b.articles, guid)
			delete(b.articleTimestamps, guid)
		}
	}
}

func (b *Bot) sendWelcomeMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Привет! Я бот, который предоставляет RSS-ленту статей с Хабра по теме информационной безопасности.\n\nДоступные команды:\n/infosec или /security - получить последние статьи по информационной безопасности")
	_, err := b.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending welcome message: %v", err)
	}
}

func (b *Bot) sendHelpMessage(chatID int64) {
	helpText := "Доступные команды:\n" +
		"/infosec или /security - получить последние статьи по информационной безопасности\n" +
		"/help - показать это сообщение\n" +
		"/start - начать работу с ботом"

	msg := tgbotapi.NewMessage(chatID, helpText)
	_, err := b.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending help message: %v", err)
	}
}

func (b *Bot) sendInfoSecFeed(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Получаю последние статьи по информационной безопасности с Хабра...")
	sentMsg, err := b.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending loading message: %v", err)
		// If we can't send the loading message, try to proceed anyway
		// Create a dummy message ID to avoid issues later
		sentMsg = tgbotapi.Message{MessageID: 0}
	}

	articles, err := b.getHabrInfoSecFeed()
	if err != nil {
		log.Printf("Error getting Habr feed: %v", err)
		errorMsg := tgbotapi.NewMessage(chatID, "Ошибка при получении статей. Пожалуйста, попробуйте позже.")
		b.bot.Send(errorMsg)
		// If we sent the loading message, try to delete it
		if sentMsg.MessageID != 0 {
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
			b.bot.Send(deleteMsg)
		}
		return
	}

	if len(articles) == 0 {
		// If we sent the loading message, try to delete it
		if sentMsg.MessageID != 0 {
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
			b.bot.Send(deleteMsg)
		}
		noArticlesMsg := tgbotapi.NewMessage(chatID, "На данный момент нет новых статей по информационной безопасности.")
		b.bot.Send(noArticlesMsg)
		return
	}

	// Delete the "loading" message if we successfully got articles
	if sentMsg.MessageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
		b.bot.Send(deleteMsg)
	}

	// Send articles
	for _, article := range articles {
		articleMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"📚 <b>%s</b>\n\n%s\n\n🔗 <a href=\"%s\">Читать на Хабре</a>",
			html.EscapeString(article.Title),
			html.EscapeString(article.Summary),
			article.Link,
		))
		articleMsg.ParseMode = "HTML"
		
		_, err := b.bot.Send(articleMsg)
		if err != nil {
			log.Printf("Error sending article '%s': %v", article.Title, err)
			// Continue to next article instead of stopping
			continue
		}
		
		// Small delay between messages to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}
}

func (b *Bot) getHabrInfoSecFeed() ([]Article, error) {
	// URL for Habr infosec category
	url := "https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru"

	feed, err := b.fp.ParseURL(url)
	if err != nil {
		return nil, err
	}

	var articles []Article
	for _, item := range feed.Items {
		// Skip if we've already sent this article
		if b.wasArticleSent(item.GUID) {
			continue
		}

		// Mark as sent
		b.markArticleAsSent(item.GUID)

		// Parse publication date
		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}

		// Create article
		article := Article{
			Title:   item.Title,
			Link:    item.Link,
			Summary: b.trimSummary(item.Description),
			Date:    pubDate,
		}

		articles = append(articles, article)

		// Limit to 10 most recent articles
		if len(articles) >= 10 {
			break
		}
	}

	return articles, nil
}

func (b *Bot) trimSummary(summary string) string {
	// Remove HTML tags and trim length
	summary = strings.ReplaceAll(summary, "<br>", " ")
	summary = strings.ReplaceAll(summary, "<p>", " ")
	summary = strings.ReplaceAll(summary, "</p>", " ")
	summary = strings.ReplaceAll(summary, "<strong>", "")
	summary = strings.ReplaceAll(summary, "</strong>", "")
	summary = strings.ReplaceAll(summary, "<em>", "")
	summary = strings.ReplaceAll(summary, "</em>", "")

	// Remove extra spaces
	summary = strings.Join(strings.Fields(summary), " ")

	// Limit to 200 characters
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}

	return summary
}

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	bot := NewBot(token)

	log.Println("Starting Habr InfoSec RSS Bot...")
	bot.Start()
}
