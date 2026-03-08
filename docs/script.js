document.addEventListener('DOMContentLoaded', function () {
    const chatMessages = document.getElementById('chatMessages');
    const messageInput = document.getElementById('messageInput');
    const sendButton   = document.getElementById('sendButton');
    const clearChatBtn = document.getElementById('clearChat');

    function initIcons() {
        if (window.lucide) window.lucide.createIcons();
    }

    /* ── Message rendering ────────────────────────────────────────────────── */

    function addMessage(content, isUser = false, type = 'text') {
        const wrap = document.createElement('div');
        wrap.className = `flex gap-3 ${isUser ? 'flex-row-reverse self-end' : ''} max-w-[90%] md:max-w-[85%] message-in`;

        const avatar = document.createElement('div');
        avatar.className = `flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center ${isUser ? 'bg-indigo-600 text-white' : 'bg-slate-100 text-slate-500'}`;
        avatar.innerHTML = isUser
            ? '<i data-lucide="user" class="w-4 h-4"></i>'
            : '<i data-lucide="bot"  class="w-4 h-4"></i>';

        const bubble = document.createElement('div');
        bubble.className = `${isUser
            ? 'bg-indigo-600 text-white rounded-tr-none'
            : 'bg-slate-100 text-slate-800 rounded-tl-none'
        } p-4 rounded-2xl shadow-sm message-content`;

        if (type === 'loading') {
            bubble.innerHTML = `<div class="flex items-center gap-1.5 py-1"><div class="dot-flashing"></div></div>`;
            wrap.id = 'loadingMessage';
        } else if (type === 'html') {
            bubble.innerHTML = content;
        } else {
            bubble.innerHTML = `<p class="text-sm leading-relaxed">${escapeAndFormat(content)}</p>`;
        }

        wrap.appendChild(avatar);
        wrap.appendChild(bubble);
        chatMessages.appendChild(wrap);
        chatMessages.scrollTop = chatMessages.scrollHeight;
        initIcons();
        return wrap;
    }

    function escapeAndFormat(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML.replace(/(\/\w+)/g, '<code class="bg-white/30 px-1 rounded">$1</code>');
    }

    /* ── Article card rendering ───────────────────────────────────────────── */

    function renderArticles(articles) {
        if (!articles || articles.length === 0) {
            return '<p class="text-sm">Статей не найдено.</p>';
        }

        const cards = articles.map(a => {
            const date = new Date(a.date).toLocaleDateString('ru-RU', {
                day: 'numeric', month: 'long', year: 'numeric'
            });
            const img = a.image
                ? `<div class="h-40 overflow-hidden bg-slate-100 relative">
                       <img src="${sanitizeUrl(a.image)}" alt="" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500">
                       <div class="absolute top-2 right-2 bg-white/90 backdrop-blur-sm px-2 py-1 rounded text-[10px] font-bold text-slate-600 shadow-sm">HABR</div>
                   </div>`
                : '';
            return `
                <div class="article-card group bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm hover:shadow-md transition-all">
                    ${img}
                    <div class="p-4">
                        <div class="flex items-center gap-2 text-[10px] text-slate-400 font-bold uppercase tracking-wider mb-2">
                            <i data-lucide="calendar" class="w-3 h-3"></i>${date}
                        </div>
                        <h3 class="font-bold text-slate-900 group-hover:text-indigo-600 transition-colors mb-2 line-clamp-2 leading-tight">
                            ${escapeHtml(a.title)}
                        </h3>
                        <p class="text-xs text-slate-500 line-clamp-3 leading-relaxed mb-4">${escapeHtml(a.summary)}</p>
                        <a href="${sanitizeUrl(a.link)}" target="_blank" rel="noopener noreferrer"
                           class="inline-flex items-center justify-center gap-2 w-full py-2 bg-slate-50 hover:bg-indigo-50 text-slate-700 hover:text-indigo-600 rounded-lg text-xs font-bold transition-all border border-slate-100 hover:border-indigo-100">
                            Читать на Хабре
                            <i data-lucide="external-link" class="w-3 h-3"></i>
                        </a>
                    </div>
                </div>`;
        }).join('');

        return `<div class="space-y-4">
                    <p class="text-sm font-medium mb-4">Последние новости информационной безопасности:</p>
                    ${cards}
                </div>`;
    }

    function escapeHtml(str) {
        const d = document.createElement('div');
        d.textContent = String(str);
        return d.innerHTML;
    }

    function sanitizeUrl(url) {
        try {
            const u = new URL(url);
            return (u.protocol === 'https:' || u.protocol === 'http:') ? url : '#';
        } catch { return '#'; }
    }

    /* ── Command handling ─────────────────────────────────────────────────── */

    async function handleCommand(command) {
        const cmd = command.toLowerCase().trim();

        if (cmd === '/start') {
            setTimeout(() => addMessage(
                'Я готов к работе! Введите /infosec для свежих статей или /help для списка команд.'
            ), 400);

        } else if (cmd === '/help') {
            setTimeout(() => addMessage(
                'Доступные команды:<br>• <strong>/infosec</strong> — последние новости ИБ<br>• <strong>/help</strong> — помощь<br>• <strong>/start</strong> — перезапуск',
                false, 'html'
            ), 400);

        } else if (cmd === '/infosec' || cmd === '/security') {
            const loading = addMessage('', false, 'loading');
            try {
                const res = await fetch('/api/articles');
                if (!res.ok) throw new Error(`HTTP ${res.status}`);
                const articles = await res.json();
                loading.remove();
                addMessage(renderArticles(articles), false, 'html');
            } catch (err) {
                console.error('Fetch error:', err);
                loading.remove();
                addMessage('Ошибка при получении статей. Попробуйте позже.');
            }

        } else {
            setTimeout(() => addMessage(
                'Неизвестная команда. Введите /help чтобы увидеть список команд.'
            ), 400);
        }
    }

    /* ── Send ─────────────────────────────────────────────────────────────── */

    function sendMessage() {
        const text = messageInput.value.trim();
        if (!text) return;
        addMessage(text, true);
        messageInput.value = '';
        handleCommand(text);
    }

    sendButton.addEventListener('click', sendMessage);
    messageInput.addEventListener('keypress', e => { if (e.key === 'Enter') sendMessage(); });
    clearChatBtn.addEventListener('click', () => {
        chatMessages.innerHTML = '';
        addMessage('Чат очищен. Чем могу помочь?');
    });

    // Expose for debugging
    window.bot = { addMessage, handleCommand };
});
