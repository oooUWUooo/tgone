document.addEventListener('DOMContentLoaded', function() {
    const chatMessages = document.getElementById('chatMessages');
    const messageInput = document.getElementById('messageInput');
    const sendButton = document.getElementById('sendButton');
    
    // Function to add a message to the chat
    function addMessage(text, isUser = false) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${isUser ? 'user-message' : 'bot-message'}`;
        
        const messageContent = document.createElement('div');
        messageContent.className = 'message-content';
        
        // Process the text to handle HTML-like formatting
        const processedText = processMessageText(text);
        messageContent.innerHTML = processedText;
        
        messageDiv.appendChild(messageContent);
        chatMessages.appendChild(messageDiv);
        
        // Scroll to bottom
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    // Function to process message text and handle formatting
    function processMessageText(text) {
        // Convert URLs to links
        let processed = text.replace(/(https?:\/\/[^\s]+)/g, '<a href="$1" target="_blank">$1</a>');
        
        // Convert bold text
        processed = processed.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
        processed = processed.replace(/\*(.*?)\*/g, '<em>$1</em>');
        
        // Convert newlines to <br>
        processed = processed.replace(/\n/g, '<br>');
        
        return processed;
    }
    
    // Function to get real articles from Habr RSS
    function getHabrArticles() {
        return new Promise((resolve, reject) => {
            const rssUrl = 'https://habr.com/ru/rss/hub/infosecurity/all/?fl=ru';
            
            // Using feednami to parse the RSS feed
            if (typeof feednami !== 'undefined') {
                feednami.load(rssUrl, function(err, feed) {
                    if (err) {
                        console.error('Error loading RSS feed:', err);
                        reject(err);
                        return;
                    }
                    
                    const articles = feed.entries.slice(0, 10); // Get first 10 articles
                    resolve(articles);
                });
            } else {
                reject(new Error('Feednami library not loaded'));
            }
        });
    }
    
    // Function to format articles for display
    function formatArticles(articles) {
        if (!articles || articles.length === 0) {
            return 'Не удалось найти статьи по информационной безопасности.';
        }
        
        let result = 'Последние статьи по информационной безопасности:<br><br>';
        
        articles.forEach((article, index) => {
            // Clean up description by removing HTML tags and limiting length
            let description = article.description || article.contentSnippet || article.content || '';
            
            // Remove HTML tags
            const div = document.createElement('div');
            div.innerHTML = description;
            description = div.textContent || div.innerText || '';
            
            // Limit description length
            if (description.length > 200) {
                description = description.substring(0, 200) + '...';
            }
            
            result += `📚 <strong>${article.title}</strong><br>`;
            result += `${description}<br>`;
            result += `🔗 <a href="${article.link}" target="_blank">Читать на Хабре</a><br><br>`;
        });
        
        return result;
    }
    
    // Function to handle bot response for /infosec and /security commands
    async function handleInfosecCommand() {
        try {
            addMessage('Получаю последние статьи по информационной безопасности с Хабра...', false);
            const articles = await getHabrArticles();
            const formattedArticles = formatArticles(articles);
            addMessage(formattedArticles, false);
        } catch (error) {
            console.error('Error fetching articles:', error);
            addMessage('Ошибка при загрузке статей. Пожалуйста, попробуйте позже.', false);
        }
    }
    
    // Function to simulate bot response
    function getBotResponse(message) {
        const lowerMessage = message.toLowerCase().trim();
        
        if (lowerMessage === '/start' || lowerMessage === '/start ') {
            return `Привет! Я бот, который предоставляет RSS-ленту статей с Хабра по теме информационной безопасности.<br><br>Доступные команды:<br>• /help - показать справку по командам<br>• /infosec или /security - получить последние статьи по информационной безопасности`;
        } else if (lowerMessage === '/help' || lowerMessage === '/help ') {
            return `Доступные команды:<br>• /infosec или /security - получить последние статьи по информационной безопасности<br>• /help - показать это сообщение<br>• /start - начать работу с ботом`;
        } else if (lowerMessage === '/infosec' || lowerMessage === '/security' || lowerMessage === '/infosec ' || lowerMessage === '/security ') {
            // Return a loading message, actual articles will be loaded asynchronously
            return 'Получаю последние статьи по информационной безопасности с Хабра...';
        } else if (message === '') {
            return 'Пожалуйста, введите команду. Доступные команды: /start, /help, /infosec, /security';
        } else {
            return 'Я не понимаю эту команду. Попробуйте использовать одну из следующих команд: /start, /help, /infosec, /security';
        }
    }
    
    // Function to handle sending a message
    function sendMessage() {
        const message = messageInput.value.trim();
        
        if (message) {
            // Add user message
            addMessage(message, true);
            
            // Clear input
            messageInput.value = '';
            
            const lowerMessage = message.toLowerCase().trim();
            
            // Check if it's an infosec/security command to handle asynchronously
            if (lowerMessage === '/infosec' || lowerMessage === '/security' || 
                lowerMessage === '/infosec ' || lowerMessage === '/security ') {
                // Handle these commands with real RSS functionality
                handleInfosecCommand();
            } else {
                // Simulate bot thinking for other commands
                setTimeout(() => {
                    const botResponse = getBotResponse(message);
                    addMessage(botResponse, false);
                }, 1000);
            }
        }
    }
    
    // Event listeners
    sendButton.addEventListener('click', sendMessage);
    
    messageInput.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            sendMessage();
        }
    });
    
    // Add initial bot message if not already present
    if (chatMessages.children.length === 0) {
        addMessage('Привет! Я бот, который предоставляет RSS-ленту статей с Хабра по теме информационной безопасности. Введите команду, например /infosec, чтобы получить последние статьи.', false);
    }
});