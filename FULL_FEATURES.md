# Habr InfoSec RSS Bot - Complete Features Documentation

## Overview

This project implements a comprehensive solution that includes:
1. A Telegram bot for fetching information security articles from Habr
2. A web interface with full bot functionality 
3. A backend API to support the web interface
4. GitHub Pages deployment capability

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Telegram      │    │   Backend API    │    │   Web Interface │
│     Bot         │    │    (Go)          │    │   (HTML/CSS/JS) │
└─────────┬───────┘    └─────────┬────────┘    └─────────┬───────┘
          │                      │                       │
          │     Commands         │    API Requests       │
          └──────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────┴─────────────┐
                    │    RSS Fetching &        │
                    │   Article Management     │
                    │      (Go Backend)        │
                    └───────────────────────────┘
```

## Features

### Telegram Bot Features
- `/start` - Welcome message and introduction
- `/help` - Help information about commands
- `/infosec` or `/security` - Fetch latest information security articles from Habr
- Deduplication of articles using GUID tracking
- Automatic cleanup of old articles (24-hour expiry)
- Rate limiting to prevent spam
- HTML sanitization for security

### Web Interface Features
- Interactive chat interface that mirrors the Telegram bot
- Real-time fetching of articles via backend API
- Responsive design for mobile and desktop
- Full command support (`/start`, `/help`, `/infosec`, `/security`)
- Real Habr RSS feed integration

### Backend API
- `/api/articles` - Returns latest infosec articles in JSON format
- CORS support for browser access
- Article deduplication and caching
- Rate limiting and security measures

## Technical Implementation

### Backend (Go)
- **Telegram Integration**: Uses `go-telegram-bot-api` for Telegram communication
- **RSS Parsing**: Uses `gofeed` library to parse RSS feeds
- **Concurrency**: Thread-safe operations with mutexes
- **Memory Management**: Automatic cleanup of expired articles
- **Web Server**: Built-in HTTP server for API and static file serving

### Frontend (HTML/CSS/JS)
- **Modern UI**: Clean, responsive chat interface
- **API Integration**: Fetches articles from backend API endpoint
- **Message Formatting**: Proper display of articles with links
- **Error Handling**: Graceful handling of API errors

## Deployment Options

### Option 1: Telegram Bot Only
```bash
TELEGRAM_BOT_TOKEN=your_token_here ./habr-rss-bot
```

### Option 2: Web Interface Only (no Telegram token needed)
```bash
TELEGRAM_BOT_TOKEN=dummy_token_for_testing ./habr-rss-bot
```

### Option 3: GitHub Pages Deployment
1. Push repository to GitHub
2. Enable GitHub Pages in repository settings
3. Select `/docs` folder as source
4. Access at `https://<username>.github.io/<repository>`

## API Endpoints

### GET /api/articles
Returns latest information security articles from Habr in JSON format:
```json
[
  {
    "title": "Article Title",
    "link": "https://habr.com/...",
    "summary": "Article summary text..."
  }
]
```

### GET /
Serves the web interface from `/docs` directory

## File Structure

```
/
├── main.go                 # Main application with bot and web server
├── go.mod, go.sum         # Go dependencies
├── habr-rss-bot           # Compiled binary
├── run.sh                 # Startup script
├── README.md              # Main documentation
├── docs/                  # GitHub Pages files
│   ├── index.html         # Web interface
│   ├── script.js          # Frontend JavaScript
│   └── styles.css         # Frontend styling
└── FULL_FEATURES.md       # This documentation
```

## Running the Application

### Prerequisites
- Go 1.21+ installed

### Building
```bash
go build -o habr-rss-bot .
```

### Running with Telegram Bot
```bash
TELEGRAM_BOT_TOKEN=your_token_here PORT=8080 ./habr-rss-bot
```

### Running Web Interface Only
```bash
TELEGRAM_BOT_TOKEN=dummy_token_for_testing PORT=8080 ./habr-rss-bot
```

### Accessing the Interfaces
- **Web Interface**: http://localhost:8080
- **API**: http://localhost:8080/api/articles
- **Telegram Bot**: Through Telegram app (when running with valid token)

## Key Improvements

1. **Dual Mode Operation**: Works with or without Telegram token
2. **CORS Support**: Web interface can access backend API
3. **Article Deduplication**: Prevents duplicate articles using GUID tracking
4. **Memory Management**: Automatic cleanup of old articles
5. **Security**: HTML sanitization and rate limiting
6. **Responsive Design**: Works on all device sizes
7. **GitHub Pages Ready**: Easy deployment to GitHub Pages
8. **API First**: Clean separation between backend and frontend

## Troubleshooting

### Web Interface Not Loading Articles
- Ensure backend server is running
- Check browser console for CORS errors
- Verify `/api/articles` endpoint is accessible

### Telegram Bot Not Working
- Verify token is correct
- Check Telegram API connectivity
- Ensure bot has proper permissions

### GitHub Pages Deployment
- Verify `/docs` folder contains all necessary files
- Check that GitHub Actions workflow is properly configured
- Ensure correct branch and folder settings in GitHub Pages