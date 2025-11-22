# GitHub Pages Setup for Habr InfoSec RSS Bot

This repository includes a GitHub Pages site with a web interface for the Habr InfoSec RSS Telegram bot.

## How to Activate GitHub Pages

1. Push this repository to GitHub
2. Go to your repository settings
3. Scroll down to the "Pages" section
4. Under "Source", select "Deploy from a branch"
5. Choose the `main` branch and `/docs` folder
6. Click "Save"
7. After activation, your site will be available at: `https://<your-username>.github.io/<repository-name>`

## Features of the GitHub Pages Site

- **Interactive Chat Interface**: Simulates the Telegram bot experience in the browser
- **Real RSS Integration**: Fetches actual articles from Habr's information security hub
- **Full Command Support**: Supports `/start`, `/help`, `/infosec`, and `/security` commands
- **Responsive Design**: Works well on both desktop and mobile devices
- **Real-time Article Fetching**: When you use `/infosec` or `/security`, the site fetches the latest articles directly from Habr's RSS feed

## Technical Details

- The site is located in the `/docs` directory
- Uses the `.nojekyll` file to disable Jekyll processing and serve static files
- Includes a GitHub Actions workflow for automated deployment
- Uses the feednami JavaScript library to parse RSS feeds in the browser
- Connects to Habr's RSS feed: `https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru`

## Using the Web Interface

1. Visit your GitHub Pages URL
2. Type commands like `/infosec` or `/help` in the input field
3. Press Enter or click "Send"
4. See responses from the simulated bot
5. When using `/infosec` or `/security`, you'll get real articles from Habr's information security section

## Repository Structure

```
/
├── docs/                    # GitHub Pages site files
│   ├── index.html          # Main page with chat interface
│   ├── script.js           # JavaScript functionality for the bot
│   ├── styles.css          # Styling for the interface
│   ├── .nojekyll           # Disables Jekyll processing
│   └── _config.yml         # GitHub Pages configuration
├── .github/workflows/      # GitHub Actions workflows
│   └── pages.yml           # Automated deployment for GitHub Pages
├── main.go                 # Telegram bot source code
├── go.mod, go.sum          # Go dependencies
└── README.md               # Main project documentation
```

The GitHub Pages site provides a web-based alternative to the Telegram bot, allowing users to interact with the same functionality directly in their browser.