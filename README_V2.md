# Habr InfoSec RSS Bot v2.0 - Улучшенная версия

## 🚀 Что нового в версии 2.0

### Архитектурные улучшения

1. **Модульная архитектура** - код разделён на логические пакеты:
   - `internal/config` - централизованное управление конфигурацией
   - `internal/models` - общие модели данных
   - `internal/services` - бизнес-логика (RSS сервис)
   - `internal/handlers` - обработчики Telegram и HTTP API
   - `internal/cache` - потокобезопасный кэш с TTL
   - `cmd/bot` - точка входа приложения

2. **Улучшенная конфигурация**:
   - Поддержка переменных окружения для всех параметров
   - Гибкая настройка через `.env` файл
   - Настройки для rate limiting, таймаутов, количества статей

3. **Кэширование**:
   - Добавлен слой кэширования для RSS ленты (5 минут по умолчанию)
   - Снижение нагрузки на сервер Хабра
   - Ускорение ответа API

4. **Graceful Shutdown**:
   - Корректная обработка сигналов завершения
   - Очистка ресурсов при остановке

5. **Health Check Endpoint**:
   - Новый эндпоинт `/api/health` для мониторинга
   - Информация о статусе и версии приложения

### Технические улучшения

- ✅ Контексты для отмены операций
- ✅ Улучшенная обработка ошибок
- ✅ Расширенное логирование
- ✅ Потокобезопасные структуры данных
- ✅ Оптимизированная работа с памятью

## 📁 Структура проекта

```
/workspace
├── cmd/
│   └── bot/
│       └── main.go              # Точка входа приложения v2.0
├── internal/
│   ├── config/
│   │   └── config.go            # Конфигурация приложения
│   ├── models/
│   │   └── models.go            # Модели данных
│   ├── services/
│   │   └── rss_service.go       # RSS сервис с кэшированием
│   ├── handlers/
│   │   └── handlers.go          # Обработчики Telegram и HTTP
│   └── cache/
│       └── cache.go             # Кэш с TTL
├── docs/                        # Веб-интерфейс
│   ├── index.html
│   ├── script.js
│   └── styles.css
├── go.mod
├── go.sum
├── habr-rss-bot                 # Старая версия бота
├── habr-rss-bot-v2              # Новая версия бота ✨
└── README.md
```

## 🔧 Установка и запуск

### Быстрый старт

```bash
# Сборка новой версии
go build -o habr-rss-bot-v2 ./cmd/bot

# Запуск в режиме только веб-интерфейса
./habr-rss-bot-v2

# Или с Telegram ботом
TELEGRAM_BOT_TOKEN=your_token ./habr-rss-bot-v2
```

### Расширенная конфигурация

Создайте файл `.env`:

```bash
# Telegram
TELEGRAM_BOT_TOKEN=your_bot_token_here

# Сервер
PORT=8080

# RSS лента
HABR_RSS_URL=https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru

# Статьи
MAX_ARTICLES=10
ARTICLE_EXPIRY=24h
CLEANUP_INTERVAL=1h

# Rate limiting
RATE_LIMIT_EVERY=1s
RATE_LIMIT_BURST=1

# HTTP клиент
HTTP_TIMEOUT=30s

# Логирование
LOG_LEVEL=info
```

Запуск с файлом `.env`:

```bash
# С использованием godotenv или export
export $(cat .env | xargs)
./habr-rss-bot-v2
```

## 🌐 API Endpoints

### GET /api/articles
Возвращает последние статьи в формате JSON:

```json
{
  "success": true,
  "data": [
    {
      "title": "Название статьи",
      "link": "https://habr.com/...",
      "summary": "Краткое описание...",
      "image": "https://...",
      "date": "2026-08-14T16:15:36Z",
      "guid": "https://habr.com/ru/articles/..."
    }
  ]
}
```

### GET /api/health
Проверка здоровья приложения:

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "version": "2.0.0"
  }
}
```

### GET /
Веб-интерфейс из папки `/docs`

## 🎯 Сравнение версий

| Функция | v1.0 | v2.0 |
|---------|------|------|
| Модульная архитектура | ❌ | ✅ |
| Кэширование RSS | ❌ | ✅ |
| Конфигурация через env | ⚠️ Частично | ✅ Полностью |
| Graceful shutdown | ❌ | ✅ |
| Health check endpoint | ❌ | ✅ |
| Расширенное логирование | ⚠️ Базовое | ✅ |
| Контексты для отмены | ❌ | ✅ |
| Потокобезопасный кэш | ⚠️ Базовый | ✅ Улучшенный |

## 🧪 Тестирование

```bash
# Проверка работы API
curl http://localhost:8080/api/health
curl http://localhost:8080/api/articles

# Запуск веб-интерфейса
# Откройте http://localhost:8080 в браузере
```

## 📦 Зависимости

```bash
go mod tidy
```

Основные зависимости:
- `github.com/go-telegram-bot-api/telegram-bot-api` - Telegram Bot API
- `github.com/mmcdole/gofeed` - RSS парсер
- `golang.org/x/time/rate` - Rate limiting

## 🚀 Production Deployment

### Docker (опционально)

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bot ./cmd/bot

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/bot .
COPY --from=builder /app/docs ./docs
EXPOSE 8080
CMD ["./bot"]
```

### Systemd service

```ini
[Unit]
Description=Habr InfoSec RSS Bot
After=network.target

[Service]
Type=simple
User=bot
WorkingDirectory=/opt/habr-rss-bot
EnvironmentFile=/opt/habr-rss-bot/.env
ExecStart=/opt/habr-rss-bot/habr-rss-bot-v2
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## 🎨 Веб-интерфейс

Веб-интерфейс включает:
- Современный адаптивный дизайн
- Интерактивный чат с ботом
- Карточки статей с изображениями
- Анимации и плавные переходы
- Мобильную версию

## 📝 Лицензия

Проект распространяется под лицензией MIT.

## 🤝 Вклад в проект

1. Fork репозитория
2. Создайте feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменений (`git commit -m 'Add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Откройте Pull Request

---

**Автор**: Улучшено с помощью AI Assistant  
**Версия**: 2.0.0  
**Дата**: 2026
