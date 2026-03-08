# Habr InfoSec RSS Bot

Telegram-бот + веб-интерфейс для чтения RSS-ленты «Информационная безопасность» с Хабра.
Написан на Go, упакован в Docker — поднимай сколько угодно экземпляров.

## Быстрый старт

```bash
git clone <repo>
cd habr-rss-bot
cp .env.example .env          # вставь токены от @BotFather
docker compose up -d --build
```

Готово. Первый бот доступен на `http://localhost:8081`, второй — на `http://localhost:8082`.

---

## Запуск без Docker (локальная разработка)

```bash
go run .                                      # веб-режим, без Telegram
TELEGRAM_BOT_TOKEN=<token> go run .           # полный режим
PORT=9090 TELEGRAM_BOT_TOKEN=<token> go run . # свой порт
```

---

## Несколько ботов — как клонировать

### Вариант 1 — docker-compose (рекомендуется)

Открой `docker-compose.yml`, скопируй блок `bot-2` в `bot-3` и далее.
В `.env` добавь `BOT3_TOKEN` и `BOT3_PORT`.

```bash
make up
```

### Вариант 2 — `make add-bot` (одна команда)

```bash
# Собрать образ один раз
make docker-build

# Запустить произвольное количество ботов
make add-bot TOKEN=111:AAA PORT=8083 NAME=SecurityBot3
make add-bot TOKEN=222:BBB PORT=8084 NAME=SecurityBot4
make add-bot TOKEN=333:CCC PORT=8085 NAME=SecurityBot5
```

Каждый бот — отдельный контейнер со своей дедупликацией статей.

---

## Переменные окружения

| Переменная              | По умолчанию                                            | Описание                              |
|-------------------------|---------------------------------------------------------|---------------------------------------|
| `TELEGRAM_BOT_TOKEN`    | —                                                       | Токен от @BotFather (обязателен для Telegram) |
| `PORT`                  | `8080`                                                  | Порт HTTP-сервера внутри контейнера   |
| `BOT_NAME`              | `HabrInfoSecBot`                                        | Название бота (в логах и `/api/health`) |
| `FEED_URL`              | `https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru`  | RSS-лента                             |
| `MAX_ARTICLES`          | `10`                                                    | Максимум статей за запрос             |
| `ARTICLE_EXPIRY_HOURS`  | `24`                                                    | Через сколько часов статья считается «новой» снова |

---

## API

| Метод | Путь               | Описание                          |
|-------|--------------------|-----------------------------------|
| GET   | `/api/articles`    | Последние статьи в JSON           |
| GET   | `/api/health`      | `{"status":"ok","bot":"..."}`     |
| GET   | `/`                | Веб-интерфейс чат-бота            |

---

## Команды Makefile

```
make build          — собрать бинарь локально
make run            — запустить локально
make test           — тесты
make docker-build   — собрать Docker-образ
make up             — docker compose up -d --build
make down           — docker compose down
make logs           — следить за логами
make add-bot        — запустить ещё один экземпляр
```

---

## Архитектура

```
main.go                 — точка входа, graceful shutdown, embed docs/
internal/
  config/config.go      — конфигурация из env-переменных
  feed/feed.go          — RSS-парсер (без сайд-эффектов)
  bot/bot.go            — Telegram-бот, дедупликация per-instance
  server/server.go      — HTTP-сервер, /api/articles, /api/health
docs/                   — веб-интерфейс (встроен в бинарь через embed)
Dockerfile              — multi-stage build, итоговый образ ~20 МБ
docker-compose.yml      — шаблон для N экземпляров
```

---

## Исправленные баги исходного кода

| # | Проблема | Исправление |
|---|----------|-------------|
| 1 | **`bot.Start()` никогда не вызывался** — Telegram-бот не работал совсем | Бот запускается в горутине, `ListenAndServe` блокирует main |
| 2 | **Дедупликация ломала API** — статьи помечались «просмотренными» при вызове `/api/articles`, и веб-UI переставал показывать новые статьи | `feed.Fetcher.Fetch` не имеет сайд-эффектов; дедупликация только в боте |
| 3 | **Обрезка summary по байтам** — кириллица обрезалась посередине символа | Используются `[]rune` для подсчёта Unicode-символов |
| 4 | **Гонка данных** — `wasArticleSent` брал write-lock «на всякий случай» и делал cleanup внутри read-операции | Чёткое разделение `wasSeen` / `markSeen` / `cleanup`, каждый со своей блокировкой |
| 5 | **Нет graceful shutdown** — SIGTERM убивал процесс немедленно | `signal.NotifyContext` + `server.Shutdown` с 5-секундным таймаутом |
| 6 | **Нет health-check** — Docker не знал, жив ли контейнер | `/api/health` + `HEALTHCHECK` в Dockerfile |
