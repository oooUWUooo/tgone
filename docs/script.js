document.addEventListener('DOMContentLoaded', function() {
    const chatMessages = document.getElementById('chatMessages');
    const messageInput = document.getElementById('messageInput');
    const sendButton = document.getElementById('sendButton');
    const clearChatBtn = document.getElementById('clearChat');
    
    // Initialize Lucide icons
    function initIcons() {
        if (window.lucide) {
            window.lucide.createIcons();
        }
    }

    // Function to add a message to the chat
    function addMessage(content, isUser = false, type = 'text') {
        const messageDiv = document.createElement('div');
        messageDiv.className = `flex gap-3 ${isUser ? 'flex-row-reverse self-end' : ''} max-w-[90%] md:max-w-[85%] message-in`;
        
        const avatar = document.createElement('div');
        avatar.className = `flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center ${isUser ? 'bg-indigo-600 text-white' : 'bg-slate-100 text-slate-500'}`;
        avatar.innerHTML = isUser ? '<i data-lucide="user" class="w-4 h-4"></i>' : '<i data-lucide="bot" class="w-4 h-4"></i>';
        
        const bubble = document.createElement('div');
        bubble.className = `${isUser ? 'bg-indigo-600 text-white rounded-tr-none' : 'bg-slate-100 text-slate-800 rounded-tl-none'} p-4 rounded-2xl shadow-sm message-content`;

        if (type === 'text') {
            bubble.innerHTML = `<p class="text-sm leading-relaxed">${processText(content)}</p>`;
        } else if (type === 'html') {
            bubble.innerHTML = content;
        } else if (type === 'loading') {
            bubble.innerHTML = `
                <div class="flex items-center gap-1.5 py-1">
                    <div class="dot-flashing"></div>
                </div>
            `;
            messageDiv.id = 'loadingMessage';
        }
        
        messageDiv.appendChild(avatar);
        messageDiv.appendChild(bubble);
        chatMessages.appendChild(messageDiv);
        
        // Scroll to bottom
        chatMessages.scrollTop = chatMessages.scrollHeight;
        initIcons();
        return messageDiv;
    }

    function processText(text) {
        // Simple escaping for user input to prevent XSS
        const div = document.createElement('div');
        div.textContent = text;
        let escaped = div.innerHTML;

        // Convert common commands to bold/code
        escaped = escaped.replace(/(\/\w+)/g, '<code class="bg-white/30 px-1 rounded">$1</code>');
        
        return escaped;
    }

    // Function to render article cards
    function renderArticles(articles) {
        if (!articles || articles.length === 0) {
            return '<p class="text-sm">К сожалению, новых статей не найдено.</p>';
        }

        let html = '<div class="space-y-4">';
        html += '<p class="text-sm font-medium mb-4">Вот последние новости информационной безопасности:</p>';
        
        articles.forEach(article => {
            const date = new Date(article.date).toLocaleDateString('ru-RU', {
                day: 'numeric',
                month: 'long',
                year: 'numeric'
            });

            html += `
                <div class="article-card group bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm hover:shadow-md transition-all">
                    ${article.image ? `
                        <div class="h-40 overflow-hidden bg-slate-100 relative">
                            <img src="${article.image}" alt="${article.title}" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500">
                            <div class="absolute top-2 right-2 bg-white/90 backdrop-blur-sm px-2 py-1 rounded text-[10px] font-bold text-slate-600 shadow-sm">HABR</div>
                        </div>
                    ` : ''}
                    <div class="p-4">
                        <div class="flex items-center gap-2 text-[10px] text-slate-400 font-bold uppercase tracking-wider mb-2">
                            <i data-lucide="calendar" class="w-3 h-3"></i>
                            ${date}
                        </div>
                        <h3 class="font-bold text-slate-900 group-hover:text-indigo-600 transition-colors mb-2 line-clamp-2 leading-tight">
                            ${article.title}
                        </h3>
                        <p class="text-xs text-slate-500 line-clamp-3 leading-relaxed mb-4">
                            ${article.summary}
                        </p>
                        <a href="${article.link}" target="_blank" class="inline-flex items-center justify-center gap-2 w-full py-2 bg-slate-50 hover:bg-indigo-50 text-slate-700 hover:text-indigo-600 rounded-lg text-xs font-bold transition-all border border-slate-100 hover:border-indigo-100">
                            Читать на Хабре
                            <i data-lucide="external-link" class="w-3 h-3"></i>
                        </a>
                    </div>
                </div>
            `;
        });
        
        html += '</div>';
        return html;
    }

    async function handleCommand(command) {
        const cmd = command.toLowerCase().trim();
        
        if (cmd === '/start') {
            setTimeout(() => {
                addMessage('Я готов к работе! Используйте /infosec для получения свежих статей по безопасности или /help для списка команд.');
            }, 500);
        } else if (cmd === '/help') {
            setTimeout(() => {
                addMessage('Доступные команды:<br>• <strong>/infosec</strong> — последние новости ИБ<br>• <strong>/help</strong> — помощь<br>• <strong>/start</strong> — перезапуск бота', false, 'html');
            }, 500);
        } else if (cmd === '/infosec' || cmd === '/security') {
            const loading = addMessage('', false, 'loading');

            try {
                const response = await fetch('/api/articles');
                if (!response.ok) throw new Error('Network response was not ok: ' + response.status);
                
                const data = await response.json();
                
                // Handle both array and wrapped response formats
                const articles = Array.isArray(data) ? data : (data.data || []);

                loading.remove();
                if (articles.length === 0) {
                    addMessage('На данный момент нет новых статей.');
                } else {
                    addMessage(renderArticles(articles), false, 'html');
                }
            } catch (error) {
                console.error('Error fetching articles:', error);
                loading.remove();
                addMessage('Извините, произошла ошибка при получении данных. Попробуйте позже.', false);
            }
        } else {
            setTimeout(() => {
                addMessage('Неизвестная команда. Введите /help чтобы увидеть список доступных команд.');
            }, 500);
        }
    }

    function sendMessage() {
        const text = messageInput.value.trim();
        if (!text) return;

        addMessage(text, true);
        messageInput.value = '';
        
        handleCommand(text);
    }

    // Event Listeners
    sendButton.addEventListener('click', sendMessage);
    messageInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') sendMessage();
    });

    clearChatBtn.addEventListener('click', () => {
        chatMessages.innerHTML = '';
        addMessage('Чат очищен. Чем я могу помочь?');
    });

    // Handle mobile menu toggle (simple version)
    document.getElementById('menuToggle').addEventListener('click', () => {
        alert('Habr InfoSec Bot v1.0.0\nПрофессиональный агрегатор новостей информационной безопасности.');
    });

    // Initial greeting if chat is empty
    if (chatMessages.children.length <= 1) {
        // Already have one welcome message in HTML
    }

    // Export for debugging
    window.bot = { addMessage, handleCommand };
});
