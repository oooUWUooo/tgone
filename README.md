# Habr InfoSec RSS Bot

Telegram-бот + веб-дашборд для чтения 6 хабов с Хабра.
Написан на Go, упакован в Docker, фронтенд деплоится на GitHub Pages.

---

## Быстрый старт

```bash
git clone https://github.com/oooUWUooo/tgone.git
cd tgone
cp .env.example .env
docker compose up -d --build
```

Бот 1: `http://localhost:8081` · Бот 2: `http://localhost:8082`

---

## Запуск без Docker

```bash
go run .                                      # веб-режим
TELEGRAM_BOT_TOKEN=<token> go run .           # полный режим
```

---

## Telegram-команды

| Команда | Действие |
|---------|----------|
| `/start` | Приветствие |
| `/hubs` | Список доступных хабов |
| `/hub <id>` | Статьи из конкретного хаба |
| `/infosec` | Статьи по ИБ (сокращение) |
| `/search <запрос>` | Поиск по всем хабам |
| `/subscribe [hub1 hub2]` | Подписаться на авто-обновления |
| `/unsubscribe` | Отписаться |
| `/help` | Справка |

**Доступные хабы:** `infosec` · `devops` · `webdev` · `programming` · `sysadm` · `linux`

---

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `TELEGRAM_BOT_TOKEN` | — | Токен от @BotFather |
| `PORT` | `8080` | Порт HTTP-сервера |
| `BOT_NAME` | `HabrInfoSecBot` | Имя бота |
| `MAX_ARTICLES` | `20` | Статей на хаб |
| `CACHE_TTL_MINUTES` | `5` | TTL кеша фидов |
| `POLL_INTERVAL_MINUTES` | `15` | Интервал фонового поллера |
| `ARTICLE_EXPIRY_HOURS` | `24` | Время жизни записей о прочитанном |
| `DATA_FILE` | `subscriptions.json` | Путь к хранилищу подписок |

---

## API

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/articles?hub=infosec` | Статьи хаба |
| GET | `/api/hubs` | Список хабов с метаданными |
| GET | `/api/search?q=запрос` | Поиск по всем хабам |
| GET | `/api/stats` | Статистика: хабы, подписчики |
| GET | `/api/health` | Статус сервиса |
| GET | `/api/events` | SSE-поток обновлений |

---

## Архитектура

```
main.go                       точка входа, graceful shutdown, go:embed
internal/
  cache/cache.go              generic TTL-кеш
  hub/hub.go                  реестр 6 хабов Хабра
  config/config.go            env-конфигурация
  feed/feed.go                RSS-парсер + кеш + поиск + время чтения
  storage/storage.go          персистентные подписки (JSON)
  bot/bot.go                  Telegram: команды + фоновый поллер
  server/server.go            HTTP: REST + SSE
docs/                         веб-дашборд (embedded в бинарь)
Dockerfile                    multi-stage, ~20 МБ
docker-compose.yml            N экземпляров
```

---

## Что нового vs предыдущая версия

| | Было | Стало |
|---|---|---|
| Хабы | 1 (infosec) | 6 (infosec, devops, webdev, ...) |
| Кеш | нет | TTL 5 мин, no RSS-spam |
| Подписки | нет | /subscribe + JSON-персистентность |
| Поллер | нет | фоновый push подписчикам |
| Поиск | нет | /search + GET /api/search |
| API | 2 эндпоинта | 6 эндпоинтов + SSE |
| Frontend | чат | дашборд: вкладки, поиск, закладки, live |
