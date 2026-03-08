.PHONY: build run test tidy docker-build up down logs ps add-bot

BINARY := habr-rss-bot
IMAGE  := habr-rss-bot

## ── Local development ──────────────────────────────────────────────────────────

build:          ## Build the binary
	go build -ldflags="-s -w" -o $(BINARY) .

run:            ## Run locally (web-only mode if TELEGRAM_BOT_TOKEN is not set)
	go run .

test:           ## Run tests
	go test ./...

tidy:           ## Tidy and vendor dependencies
	go mod tidy

lint:           ## Lint (requires golangci-lint)
	golangci-lint run ./...

## ── Docker ─────────────────────────────────────────────────────────────────────

docker-build:   ## Build Docker image
	docker build -t $(IMAGE) .

up:             ## Build images and start all bot instances
	docker compose up -d --build

down:           ## Stop all bot instances
	docker compose down

logs:           ## Follow logs for all instances
	docker compose logs -f

ps:             ## Show running containers
	docker compose ps

## ── Spawn a one-off bot instance ───────────────────────────────────────────────
## Usage: make add-bot TOKEN=<telegram_token> PORT=<host_port> NAME=<bot_name>
##   e.g. make add-bot TOKEN=12345:AAA PORT=8085 NAME=MyBot
add-bot:
	@[ -n "$(TOKEN)" ] || (echo "TOKEN is required" && exit 1)
	@[ -n "$(PORT)"  ] || (echo "PORT is required"  && exit 1)
	docker run -d \
		--name habr-bot-$(PORT) \
		--restart unless-stopped \
		-e TELEGRAM_BOT_TOKEN=$(TOKEN) \
		-e BOT_NAME=$(or $(NAME),HabrBot-$(PORT)) \
		-e PORT=8080 \
		-p $(PORT):8080 \
		$(IMAGE)
	@echo "Bot started → http://localhost:$(PORT)"

help:           ## Print this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
