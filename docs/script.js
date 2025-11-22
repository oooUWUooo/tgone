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
    
    // Function to simulate bot response
    function getBotResponse(message) {
        const lowerMessage = message.toLowerCase().trim();
        
        if (lowerMessage === '/start' || lowerMessage === '/start ') {
            return `Привет! Я бот, который предоставляет RSS-ленту статей с Хабра по теме информационной безопасности.<br><br>Доступные команды:<br>• /help - показать справку по командам<br>• /infosec или /security - получить последние статьи по информационной безопасности`;
        } else if (lowerMessage === '/help' || lowerMessage === '/help ') {
            return `Доступные команды:<br>• /infosec или /security - получить последние статьи по информационной безопасности<br>• /help - показать это сообщение<br>• /start - начать работу с ботом`;
        } else if (lowerMessage === '/infosec' || lowerMessage === '/security' || lowerMessage === '/infosec ' || lowerMessage === '/security ') {
            // Simulate getting articles from Habr
            return `Получаю последние статьи по информационной безопасности с Хабра...<br><br>📚 <strong>Современные методы атак на веб-приложения</strong><br><br>В статье рассматриваются новые методы атак на веб-приложения, включая XSS, CSRF и SQL-инъекции. Подробно разобраны способы защиты и лучшие практики разработки безопасного кода.<br><br>🔗 <a href="https://habr.com/ru/articles/example1" target="_blank">Читать на Хабре</a><br><br>📚 <strong>Анализ уязвимостей в системах аутентификации</strong><br><br>Статья посвящена анализу типичных уязвимостей в системах аутентификации и авторизации. Рассмотрены как традиционные, так и современные подходы к защите пользовательских данных.<br><br>🔗 <a href="https://habr.com/ru/articles/example2" target="_blank">Читать на Хабре</a><br><br>📚 <strong>Криптографические методы защиты данных</strong><br><br>В статье представлены современные криптографические методы защиты данных, включая шифрование, хеширование и цифровые подписи. Обсуждаются как симметричные, так и асимметричные алгоритмы.<br><br>🔗 <a href="https://habr.com/ru/articles/example3" target="_blank">Читать на Хабре</a>`;
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
            
            // Simulate bot thinking
            setTimeout(() => {
                const botResponse = getBotResponse(message);
                addMessage(botResponse, false);
            }, 1000);
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