package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

	"habr-rss-bot/internal/config"
	"habr-rss-bot/internal/models"
	"habr-rss-bot/internal/services"
)

// TelegramHandler handles Telegram bot messages
type TelegramHandler struct {
	bot         *tgbotapi.BotAPI
	rssService  *services.RSSService
	config      *config.Config
}

// NewTelegramHandler creates a new Telegram handler
func NewTelegramHandler(bot *tgbotapi.BotAPI, rssService *services.RSSService, cfg *config.Config) *TelegramHandler {
	return &TelegramHandler{
		bot:        bot,
		rssService: rssService,
		config:     cfg,
	}
}

// HandleMessage processes incoming Telegram messages
func (h *TelegramHandler) HandleMessage(msg *tgbotapi.Message) {
	if msg.Text == "" {
		return
	}

	text := strings.TrimSpace(msg.Text)
	chatID := msg.Chat.ID

	switch text {
	case "/start":
		h.sendWelcomeMessage(chatID)
	case "/help":
		h.sendHelpMessage(chatID)
	case "/infosec", "/security":
		h.sendInfoSecFeed(chatID)
	default:
		h.sendWelcomeMessage(chatID)
	}
}

func (h *TelegramHandler) sendWelcomeMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, 
		"Привет! Я профессиональный агрегатор новостей информационной безопасности с Хабра.\n\n"+
			"Доступные команды:\n"+
			"/infosec или /security - получить последние статьи по ИБ\n"+
			"/help - показать справку")
	
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending welcome message: %v", err)
	}
}

func (h *TelegramHandler) sendHelpMessage(chatID int64) {
	helpText := "📚 Доступные команды:\n\n" +
		"• /infosec или /security — последние статьи по информационной безопасности\n" +
		"• /help — показать это сообщение\n" +
		"• /start — начать работу с ботом\n\n" +
		"Бот автоматически отслеживает новые статьи и предотвращает дублирование."

	msg := tgbotapi.NewMessage(chatID, helpText)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending help message: %v", err)
	}
}

func (h *TelegramHandler) sendInfoSecFeed(chatID int64) {
	// Send loading message
	loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Получаю последние статьи по информационной безопасности...")
	sentMsg, err := h.bot.Send(loadingMsg)
	if err != nil {
		log.Printf("Error sending loading message: %v", err)
		sentMsg = tgbotapi.Message{MessageID: 0}
	}

	articles, err := h.rssService.FetchArticles(context.Background())
	if err != nil {
		log.Printf("Error fetching articles: %v", err)
		
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Ошибка при получении статей. Пожалуйста, попробуйте позже.")
		h.bot.Send(errorMsg)
		
		if sentMsg.MessageID != 0 {
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
			h.bot.Send(deleteMsg)
		}
		return
	}

	// Delete loading message
	if sentMsg.MessageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
		h.bot.Send(deleteMsg)
	}

	if len(articles) == 0 {
		noArticlesMsg := tgbotapi.NewMessage(chatID, "ℹ️ На данный момент нет новых статей.")
		h.bot.Send(noArticlesMsg)
		return
	}

	// Send articles
	for i, article := range articles {
		articleText := fmt.Sprintf(
			"📚 <b>%s</b>\n\n%s\n\n🔗 <a href=\"%s\">Читать на Хабре</a>",
			escapeHTML(article.Title),
			escapeHTML(article.Summary),
			article.Link,
		)

		articleMsg := tgbotapi.NewMessage(chatID, articleText)
		articleMsg.ParseMode = "HTML"

		if _, err := h.bot.Send(articleMsg); err != nil {
			log.Printf("Error sending article '%s': %v", article.Title, err)
			continue
		}

		// Small delay between messages to avoid rate limiting
		if i < len(articles)-1 {
			// Rate limiting is handled by the bot's limiter
		}
	}
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// APIHandler handles HTTP API requests
type APIHandler struct {
	rssService *services.RSSService
	config     *config.Config
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(rssService *services.RSSService, cfg *config.Config) *APIHandler {
	return &APIHandler{
		rssService: rssService,
		config:     cfg,
	}
}

// HandleArticles handles GET /api/articles requests
func (h *APIHandler) HandleArticles(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	articles, err := h.rssService.FetchArticles(r.Context())
	if err != nil {
		log.Printf("Error fetching articles for API: %v", err)
		
		response := models.APIResponse{
			Success: false,
			Error:   "Failed to fetch articles",
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return articles directly for frontend compatibility
	// Frontend expects an array, not a wrapped response
	if len(articles) == 0 {
		json.NewEncoder(w).Encode([]models.Article{})
		return
	}

	json.NewEncoder(w).Encode(articles)
}

// HandleHealth handles GET /api/health requests
func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":  "healthy",
		"version": "2.0.0",
	}
	json.NewEncoder(w).Encode(response)
}
