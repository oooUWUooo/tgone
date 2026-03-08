# Habr InfoSec — веб-интерфейс

Фронтенд автоматически деплоится на GitHub Pages из папки `docs/`.

## Подключение к API

По умолчанию фронтенд обращается к `/api/articles` — это работает,
когда `docs/` встроен в Go-бинарь через `go:embed`.

Для GitHub Pages открой раздел **Настройки** в интерфейсе и укажи
полный URL задеплоенного сервера: `https://your-server.com/api/articles`.
URL сохраняется в `localStorage`.
