package telegram

import (
	"fmt"
	"habr-rss-bot/internal/domain"
	"html"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramBot struct {
	bot            *tgbotapi.BotAPI
	articleUsecase domain.ArticleUsecase
}

func NewTelegramBot(token string, u domain.ArticleUsecase) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &TelegramBot{
		bot:            bot,
		articleUsecase: u,
	}, nil
}

func (b *TelegramBot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := b.bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go b.handleMessage(update.Message)
	}
}

func (b *TelegramBot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	switch {
	case text == "/start":
		b.sendWelcomeMessage(chatID)
	case text == "/help", text == "❓ Помощь":
		b.sendHelpMessage(chatID)
	case text == "/infosec", text == "🛡 InfoSec":
		b.sendArticlesByCategory(chatID, "infosecurity")
	case text == "/it", text == "💻 IT News":
		b.sendArticlesByCategory(chatID, "it")
	case text == "/latest", text == "🆕 Последние":
		b.sendLatestArticles(chatID)
	default:
		b.sendMessage(chatID, "Неизвестная команда. Используйте /help для списка доступных команд.")
	}
}

func (b *TelegramBot) sendWelcomeMessage(chatID int64) {
	text := "🚀 <b>Добро пожаловать в RSS Pro Aggregator!</b>\n\n" +
		"Я профессиональный бот для отслеживания новостей из сферы ИБ и IT.\n\n" +
		"Используйте кнопки меню или команды:\n" +
		"/infosec - статьи по ИБ\n" +
		"/it - новости IT\n" +
		"/latest - 10 свежих статей из всех источников\n" +
		"/help - справка"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"

	// Add keyboard
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🛡 InfoSec"),
			tgbotapi.NewKeyboardButton("💻 IT News"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🆕 Последние"),
			tgbotapi.NewKeyboardButton("❓ Помощь"),
		),
	)
	msg.ReplyMarkup = keyboard

	b.bot.Send(msg)
}

func (b *TelegramBot) sendHelpMessage(chatID int64) {
	helpText := "🛠 <b>Доступные команды:</b>\n\n" +
		"🔹 /infosec - Статьи по информационной безопасности\n" +
		"🔹 /it - Общие IT новости\n" +
		"🔹 /latest - Последние статьи из всех категорий\n" +
		"🔹 /start - Перезапуск и меню"

	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "HTML"
	b.bot.Send(msg)
}

func (b *TelegramBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	b.bot.Send(msg)
}

func (b *TelegramBot) sendArticlesByCategory(chatID int64, category string) {
	articles, err := b.articleUsecase.GetArticlesByCategory(category, 5)
	if err != nil {
		b.sendMessage(chatID, "❌ Ошибка при получении статей.")
		return
	}

	b.sendArticleMessages(chatID, articles)
}

func (b *TelegramBot) sendLatestArticles(chatID int64) {
	articles, err := b.articleUsecase.GetLatestArticles(10)
	if err != nil {
		b.sendMessage(chatID, "❌ Ошибка при получении последних статей.")
		return
	}

	b.sendArticleMessages(chatID, articles)
}

func (b *TelegramBot) sendArticleMessages(chatID int64, articles []domain.Article) {
	if len(articles) == 0 {
		b.sendMessage(chatID, "📭 На данный момент новых статей нет.")
		return
	}

	for _, a := range articles {
		text := fmt.Sprintf(
			"📚 <b>%s</b>\n\n%s\n\n🔗 <a href=\"%s\">Читать далее</a>",
			html.EscapeString(a.Title),
			html.EscapeString(a.Summary),
			a.Link,
		)
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"

		// Optional: Add inline button
		inlineBtn := tgbotapi.NewInlineKeyboardButtonURL("Перейти на Хабр", a.Link)
		inlineKbd := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(inlineBtn))
		msg.ReplyMarkup = inlineKbd

		b.bot.Send(msg)
	}
}
