#!/bin/bash

# Check if TELEGRAM_BOT_TOKEN is set
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "Error: TELEGRAM_BOT_TOKEN environment variable is not set"
    echo "Please set it before running the bot:"
    echo "export TELEGRAM_BOT_TOKEN=your_bot_token_here"
    echo "For testing purposes, you can set a dummy value, but the bot won't work without a real token"
    echo "Starting with a dummy token for testing the web interface only..."
    export TELEGRAM_BOT_TOKEN="dummy_token_for_testing"
fi

# Set default port if not set
if [ -z "$PORT" ]; then
    export PORT=8080
    echo "Using default port: $PORT"
fi

echo "Starting Habr InfoSec RSS Bot and Web Server..."
./habr-rss-bot