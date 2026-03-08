// Package hub defines the set of Habr topic hubs available in this bot.
package hub

// Hub describes a single Habr topic hub.
type Hub struct {
	ID    string // URL-safe identifier used in API calls and bot commands
	Name  string // Human-readable Russian name
	Emoji string // Emoji shown in Telegram messages
	URL   string // RSS feed URL
}

// All contains every hub the bot can read from, in display order.
var All = []Hub{
	{
		ID:    "infosec",
		Name:  "Информационная безопасность",
		Emoji: "🔐",
		URL:   "https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru",
	},
	{
		ID:    "devops",
		Name:  "DevOps",
		Emoji: "⚙️",
		URL:   "https://habr.com/ru/rss/hub/devops/all/?fl=ru",
	},
	{
		ID:    "webdev",
		Name:  "Веб-разработка",
		Emoji: "🌐",
		URL:   "https://habr.com/ru/rss/hub/webdev/all/?fl=ru",
	},
	{
		ID:    "programming",
		Name:  "Программирование",
		Emoji: "💻",
		URL:   "https://habr.com/ru/rss/hub/programming/all/?fl=ru",
	},
	{
		ID:    "sysadm",
		Name:  "Системное администрирование",
		Emoji: "🖥",
		URL:   "https://habr.com/ru/rss/hub/sysadm/all/?fl=ru",
	},
	{
		ID:    "linux",
		Name:  "Linux",
		Emoji: "🐧",
		URL:   "https://habr.com/ru/rss/hub/linux/all/?fl=ru",
	},
}

// DefaultHub is used when no hub is specified by the caller.
var DefaultHub = All[0] // infosec

// ByID returns the Hub with the given ID, and ok=true if found.
func ByID(id string) (Hub, bool) {
	for _, h := range All {
		if h.ID == id {
			return h, true
		}
	}
	return Hub{}, false
}
