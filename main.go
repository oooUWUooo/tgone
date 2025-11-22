package main

import (
	"fmt"
	"html"
	"log"
	"os"
	"strings"
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
	bot      *tgbotapi.BotAPI
	fp       *gofeed.Parser
	limiter  *rate.Limiter
	articles map[string]bool // to track sent articles
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
	}
}

func (b *Bot) Start() {
	log.Printf("Authorized on account %s", b.bot.Self.UserName)

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

func (b *Bot) sendWelcomeMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Привет! Я бот, который предоставляет RSS-ленту статей с Хабра по теме информационной безопасности.\n\nДоступные команды:\n/infosec или /security - получить последние статьи по информационной безопасности")
	b.bot.Send(msg)
}

func (b *Bot) sendHelpMessage(chatID int64) {
	helpText := "Доступные команды:\n" +
		"/infosec или /security - получить последние статьи по информационной безопасности\n" +
		"/help - показать это сообщение\n" +
		"/start - начать работу с ботом"
	
	msg := tgbotapi.NewMessage(chatID, helpText)
	b.bot.Send(msg)
}

func (b *Bot) sendInfoSecFeed(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Получаю последние статьи по информационной безопасности с Хабра...")
	sentMsg, err := b.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}

	articles, err := b.getHabrInfoSecFeed()
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, "Ошибка при получении статей. Пожалуйста, попробуйте позже.")
		b.bot.Send(errorMsg)
		return
	}

	if len(articles) == 0 {
		noArticlesMsg := tgbotapi.NewMessage(chatID, "На данный момент нет новых статей по информационной безопасности.")
		b.bot.Send(noArticlesMsg)
		return
	}

	// Delete the "loading" message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
	b.bot.Send(deleteMsg)

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
			log.Printf("Error sending article: %v", err)
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
		if b.articles[item.GUID] {
			continue
		}
		
		// Mark as sent
		b.articles[item.GUID] = true
		
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