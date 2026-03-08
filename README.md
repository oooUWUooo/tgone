# Habr InfoSec RSS Bot

Telegram-бот + веб-дашборд для чтения RSS-ленты «Информационная безопасность» с Хабра.
Написан на Go, упакован в Docker, фронтенд деплоится на GitHub Pages.

---

## Быстрый старт

```bash
git clone https://github.com/oooUWUooo/tgone.git
cd tgone
cp .env.example .env
docker compose up -d --build
```

- Бот 1: `http://localhost:8081`
- Бот 2: `http://localhost:8082`

---

## Запуск без Docker

```bash
go run .                                       # веб-режим (без Telegram)
TELEGRAM_BOT_TOKEN=<token> go run .            # полный режим
PORT=9090 TELEGRAM_BOT_TOKEN=<token> go run .  # свой порт
```

---

## Несколько ботов

```bash
make docker-build
make add-bot TOKEN=111:AAA PORT=8083 NAME=SecurityBot3
make add-bot TOKEN=222:BBB PORT=8084 NAME=SecurityBot4
```

---

## Переменные окружения

| Переменная             | По умолчанию                                           | Описание                                      |
|------------------------|--------------------------------------------------------|-----------------------------------------------|
| TELEGRAM_BOT_TOKEN     | —                                                      | Токен от @BotFather (обязателен для Telegram) |
| PORT                   | 8080                                                   | Порт HTTP-сервера внутри контейнера           |
| BOT_NAME               | HabrInfoSecBot                                         | Имя бота в логах и /api/health                |
| FEED_URL               | https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru   | RSS-лента                                     |
| MAX_ARTICLES           | 10                                                     | Максимум статей за запрос                     |
| ARTICLE_EXPIRY_HOURS   | 24                                                     | Через сколько часов статья считается новой    |

---

## API

| Метод | Путь            | Описание                      |
|-------|-----------------|-------------------------------|
| GET   | /api/articles   | Последние статьи в JSON       |
| GET   | /api/health     | {"status":"ok","bot":"..."}   |
| GET   | /               | Веб-дашборд                   |

---

## Веб-дашборд / GitHub Pages

Фронтенд (docs/) встроен в бинарь через go:embed и отдаётся на /.
При деплое на GitHub Pages укажи URL API в разделе Настройки дашборда.

Настройка GitHub Pages:
1. Settings → Pages → Source: Deploy from a branch
2. Branch: main, папка: /docs
3. В дашборде → Настройки → вставь URL: https://your-server.com/api/articles

---

## Telegram-команды

| Команда              | Действие                         |
|----------------------|----------------------------------|
| /start               | Приветствие                      |
| /infosec, /security  | Последние статьи по ИБ           |
| /help                | Справка                          |

---

## Makefile

```
make build        — собрать бинарь
make docker-build — собрать Docker-образ
make up           — docker compose up -d --build
make down         — остановить
make logs         — логи
make add-bot      — дополнительный экземпляр
```

---

## Архитектура

```
main.go                   точка входа, graceful shutdown, go:embed docs/
internal/
  config/config.go        конфигурация из env
  feed/feed.go            RSS-парсер (без сайд-эффектов)
  bot/bot.go              Telegram-бот, per-instance дедупликация
  server/server.go        HTTP: /api/articles, /api/health, static
docs/                     веб-дашборд (встроен в бинарь)
Dockerfile                multi-stage build, ~20 МБ
docker-compose.yml        шаблон для N экземпляров
.github/workflows/        автодеплой docs/ на GitHub Pages
```

---

## Исправленные баги

| # | Проблема | Исправление |
|---|----------|-------------|
| 1 | bot.Start() не вызывался — бот молчал | Бот в горутине, ListenAndServe блокирует main |
| 2 | Дедупликация ломала /api/articles | feed.Fetcher.Fetch без сайд-эффектов |
| 3 | Summary обрезался по байтам — кириллица рвалась | []rune вместо []byte |
| 4 | Гонка данных в wasArticleSent | Разделение wasSeen / markSeen / cleanup |
| 5 | Нет graceful shutdown | signal.NotifyContext + Shutdown с таймаутом |
| 6 | Нет health-check | /api/health + HEALTHCHECK в Dockerfile |
