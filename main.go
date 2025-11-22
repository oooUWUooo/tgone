package main

import (
	"encoding/json"
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

// NewBotWithoutTelegram creates a bot instance without connecting to Telegram API
// This is used for web-only mode where only the API and web interface are needed
func NewBotWithoutTelegram() *Bot {
	return &Bot{
		bot:      nil, // No Telegram bot connection
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
	if b.bot == nil {
		// In web-only mode, don't start the Telegram bot
		log.Println("Running in web-only mode - Telegram bot disabled")
		// Keep the cleanup goroutine running
		go func() {
			ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
			defer ticker.Stop()
			for range ticker.C {
				b.cleanupExpiredArticles()
				log.Println("Cleaned up expired articles")
			}
		}()
		
		// Wait indefinitely since there's no bot to run
		select {}
	}
	
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

// API handler for web interface to fetch articles
func (b *Bot) handleArticlesAPI(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Fetch articles from Habr
	articles, err := b.getHabrInfoSecFeed()
	if err != nil {
		log.Printf("Error getting articles for API: %v", err)
		http.Error(w, "Error fetching articles", http.StatusInternalServerError)
		return
	}

	// Convert articles to JSON response
	var response []map[string]string
	for _, article := range articles {
		articleMap := map[string]string{
			"title":   article.Title,
			"link":    article.Link,
			"summary": article.Summary,
		}
		response = append(response, articleMap)
	}

	// Set content type and send JSON response
	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling articles to JSON: %v", err)
		http.Error(w, "Error formatting response", http.StatusInternalServerError)
		return
	}

	w.Write(jsonData)
}

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	
	var bot *Bot
	if token != "" && token != "dummy_token_for_testing" {
		bot = NewBot(token)
		log.Println("Starting Habr InfoSec RSS Bot...")
	} else {
		log.Println("TELEGRAM_BOT_TOKEN not set or using dummy token - starting in web-only mode")
		// Create a bot instance without connecting to Telegram API
		bot = NewBotWithoutTelegram()
	}
	
	// Set up HTTP handlers for web interface
	http.HandleFunc("/api/articles", bot.handleArticlesAPI)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve static files from docs directory
		http.FileServer(http.Dir("./docs")).ServeHTTP(w, r)
	})

	// Start the web server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}
	log.Printf("Starting web server on port %s", port)
	log.Printf("Web interface available at http://localhost:%s", port)
	log.Printf("API available at http://localhost:%s/api/articles", port)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Printf("Web server error: %v", err)
	}
}
