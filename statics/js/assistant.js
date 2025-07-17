// 智能助手页面JavaScript

// 全局变量
let chatMessages = [
    { role: "system", content: "你是一个专业的个人数据分析师和生活助手" },
    { role: "assistant", content: "你好！我是智能助手，可以帮你分析数据、提供建议。有什么我可以帮助你的吗？" }
];
let isTyping = false;
let trendChart = null;
let currentSettings = {
    enableNotifications: true,
    enableSuggestions: true,
    analysisRange: 30,
    assistantPersonality: 'professional'
};

// 模拟数据
const mockData = {
    todayStats: {
        tasks: { completed: 3, total: 5 },
        reading: { time: 2.5, unit: 'hours' },
        exercise: { sessions: 1, type: 'cardio' },
        blogs: { count: 1, words: 800 }
    },
    suggestions: [
        { icon: '💡', text: '您今天的任务完成率为60%，建议优先处理剩余的重要任务' },
        { icon: '📚', text: '基于您的阅读习惯，推荐继续阅读《深度工作》' },
        { icon: '💪', text: '您已连续3天进行锻炼，保持良好的运动习惯' },
        { icon: '⏰', text: '分析显示您在下午3-5点效率最高，建议安排重要工作' }
    ],
    trendData: {
        labels: ['7天前', '6天前', '5天前', '4天前', '3天前', '2天前', '昨天', '今天'],
        datasets: [
            {
                label: '任务完成率',
                data: [80, 75, 90, 85, 70, 95, 85, 60],
                borderColor: 'rgba(0, 212, 170, 1)',
                backgroundColor: 'rgba(0, 212, 170, 0.1)',
                tension: 0.4
            },
            {
                label: '阅读时间(小时)',
                data: [2, 1.5, 3, 2.5, 1, 2, 3, 2.5],
                borderColor: 'rgba(161, 196, 253, 1)',
                backgroundColor: 'rgba(161, 196, 253, 0.1)',
                tension: 0.4
            }
        ]
    }
};

// 智能回复模板
const responseTemplates = {
    status: {
        greeting: ['让我为您分析最近的状态', '正在分析您的个人数据...', '根据您的数据，我来为您总结一下'],
        analysis: [
            '📊 **整体状态分析**',
            '✅ **优势表现**：',
            '- 任务执行：近7天平均完成率{taskRate}%',
            '- 阅读习惯：日均阅读{readingTime}小时',
            '- 运动状态：{exerciseStatus}',
            '',
            '⚠️ **需要关注**：',
            '- {suggestions}',
            '',
            '💡 **改进建议**：',
            '- {recommendations}'
        ]
    },
    time: {
        greeting: ['让我分析一下您的时间分配', '正在分析您的时间使用模式...'],
        analysis: [
            '⏰ **时间分配分析**',
            '📈 **效率高峰**：通常在{peakTime}效率最高',
            '📊 **时间分布**：',
            '- 工作学习：{workTime}小时/天',
            '- 阅读时间：{readingTime}小时/天',
            '- 锻炼时间：{exerciseTime}小时/天',
            '',
            '🎯 **优化建议**：',
            '- {timeAdvice}'
        ]
    },
    goals: {
        greeting: ['让我查看您的目标进度', '正在统计您的目标完成情况...'],
        analysis: [
            '🎯 **目标进度追踪**',
            '📚 **阅读目标**：已完成{readingProgress}%',
            '💪 **健身目标**：已完成{exerciseProgress}%',
            '📝 **写作目标**：已完成{writingProgress}%',
            '',
            '🏆 **近期成就**：',
            '- {achievements}',
            '',
            '📈 **下一步行动**：',
            '- {nextActions}'
        ]
    },
    suggestions: {
        greeting: ['基于您的数据，我有以下建议', '根据行为模式分析，为您推荐以下建议'],
        analysis: [
            '💡 **个性化建议**',
            '🔥 **立即行动**：',
            '- {immediateActions}',
            '',
            '📅 **本周计划**：',
            '- {weeklyPlans}',
            '',
            '🎯 **长期优化**：',
            '- {longTermGoals}'
        ]
    }
};

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializePage();
    setupEventListeners();
    loadTodayStats();
    loadSuggestions();
    initializeTrendChart();
    loadSettings();
});

// 初始化页面
function initializePage() {
    console.log('智能助手页面已加载');
    
    // 模拟加载过程
    setTimeout(() => {
        updateTodayStats();
        updateSuggestions();
    }, 1000);
}

// 设置事件监听器
function setupEventListeners() {
    // 发送消息
    const sendBtn = document.getElementById('sendBtn');
    const messageInput = document.getElementById('messageInput');
    
    sendBtn.addEventListener('click', sendMessage);
    messageInput.addEventListener('keypress', function(e) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
        }
    });
    
    // 快速操作按钮
    const quickBtns = document.querySelectorAll('.quick-btn');
    quickBtns.forEach(btn => {
        btn.addEventListener('click', function() {
            const action = this.dataset.action;
            handleQuickAction(action);
        });
    });
    
    // 快速操作
    const operationBtns = document.querySelectorAll('.operation-btn');
    operationBtns.forEach(btn => {
        btn.addEventListener('click', function() {
            const action = this.dataset.action;
            handleQuickOperation(action);
        });
    });
    
    // 设置面板
    const settingsBtn = document.getElementById('settingsBtn');
    const settingsPanel = document.getElementById('settingsPanel');
    const closeSettings = document.getElementById('closeSettings');
    
    settingsBtn.addEventListener('click', () => {
        settingsPanel.classList.add('active');
    });
    
    closeSettings.addEventListener('click', () => {
        settingsPanel.classList.remove('active');
    });
    
    // 刷新数据
    const refreshBtn = document.getElementById('refreshData');
    refreshBtn.addEventListener('click', refreshData);
    
    // 设置项变化监听
    setupSettingsListeners();
}

// 发送消息
function sendMessage() {
    const messageInput = document.getElementById('messageInput');
    const message = messageInput.value.trim();
    
    if (!message || isTyping) return;
    
    // 添加用户消息到对话历史
    chatMessages.push({ role: "user", content: message });
    
    // 显示用户消息
    addMessage('user', message);
    messageInput.value = '';
    
    // 创建AI消息占位符
    const aiMessageElement = createAiMessagePlaceholder();
    
    // 发送流式请求
    sendStreamingRequest(aiMessageElement);
}

// 创建AI消息占位符
function createAiMessagePlaceholder() {
    const chatContainer = document.getElementById('chatMessages');
    const messageDiv = document.createElement('div');
    messageDiv.className = 'message assistant-message';
    messageDiv.id = 'streaming-message';
    
    const avatar = document.createElement('div');
    avatar.className = 'avatar';
    avatar.innerHTML = '<i class="fas fa-robot"></i>';
    
    const messageContent = document.createElement('div');
    messageContent.className = 'message-content';
    
    const messageText = document.createElement('div');
    messageText.className = 'message-text';
    messageText.innerHTML = '<div class="typing-indicator"><span>正在思考</span><div class="typing-dots"><div class="typing-dot"></div><div class="typing-dot"></div><div class="typing-dot"></div></div></div>';
    
    messageContent.appendChild(messageText);
    messageDiv.appendChild(avatar);
    messageDiv.appendChild(messageContent);
    
    chatContainer.appendChild(messageDiv);
    chatContainer.scrollTop = chatContainer.scrollHeight;
    
    return messageDiv;
}

// 发送流式请求
async function sendStreamingRequest(aiMessageElement) {
    try {
        const response = await fetch('/api/assistant/chat', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                messages: chatMessages,
                stream: true
            })
        });
        
        if (!response.ok) {
            throw new Error('API请求失败');
        }
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let aiResponse = '';
        
        // 开始流式读取
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            const chunk = decoder.decode(value, { stream: true });
            const lines = chunk.split('\n\n').filter(line => line.trim() !== '');
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    const data = line.replace('data: ', '');
                    if (data === '[DONE]') {
                        // 完成响应
                        chatMessages.push({ role: "assistant", content: aiResponse });
                        addTimestamp(aiMessageElement);
                        return;
                    }
                    
                    try {
                        const decodedContent = decodeURIComponent(data);
                        aiResponse += decodedContent;
                        
                        // 更新消息内容
                        const messageText = aiMessageElement.querySelector('.message-text');
                        messageText.innerHTML = formatMessage(aiResponse);
                        
                        // 滚动到底部
                        const chatContainer = document.getElementById('chatMessages');
                        chatContainer.scrollTop = chatContainer.scrollHeight;
                        
                    } catch (e) {
                        console.error('Error decoding content:', e);
                    }
                }
            }
        }
        
    } catch (error) {
        console.error('发送消息失败:', error);
        
        // 显示错误消息
        const messageText = aiMessageElement.querySelector('.message-text');
        messageText.innerHTML = '<span class="error">抱歉，请求过程中出现错误。请重试。</span>';
        
        // 降级到本地生成
        setTimeout(() => {
            const lastUserMessage = chatMessages[chatMessages.length - 1];
            if (lastUserMessage && lastUserMessage.role === 'user') {
                const response = generateAIResponse(lastUserMessage.content);
                messageText.innerHTML = formatMessage(response);
                chatMessages.push({ role: "assistant", content: response });
                addTimestamp(aiMessageElement);
            }
        }, 1000);
    }
}

// 添加时间戳到消息
function addTimestamp(messageElement) {
    const messageContent = messageElement.querySelector('.message-content');
    if (messageContent && !messageContent.querySelector('.message-time')) {
        const messageTime = document.createElement('div');
        messageTime.className = 'message-time';
        messageTime.textContent = new Date().toLocaleTimeString('zh-CN', { 
            hour: '2-digit', 
            minute: '2-digit' 
        });
        messageContent.appendChild(messageTime);
    }
}

// 添加消息到聊天记录
function addMessage(sender, content) {
    const chatContainer = document.getElementById('chatMessages');
    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${sender}-message`;
    
    const avatar = document.createElement('div');
    avatar.className = 'avatar';
    avatar.innerHTML = sender === 'user' ? '<i class="fas fa-user"></i>' : '<i class="fas fa-robot"></i>';
    
    const messageContent = document.createElement('div');
    messageContent.className = 'message-content';
    
    const messageText = document.createElement('div');
    messageText.className = 'message-text';
    
    // 支持Markdown格式
    messageText.innerHTML = formatMessage(content);
    
    const messageTime = document.createElement('div');
    messageTime.className = 'message-time';
    messageTime.textContent = new Date().toLocaleTimeString('zh-CN', { 
        hour: '2-digit', 
        minute: '2-digit' 
    });
    
    messageContent.appendChild(messageText);
    messageContent.appendChild(messageTime);
    
    messageDiv.appendChild(avatar);
    messageDiv.appendChild(messageContent);
    
    chatContainer.appendChild(messageDiv);
    chatContainer.scrollTop = chatContainer.scrollHeight;
    
    // 注释掉原有的存储逻辑，现在使用新的对话历史格式
    // chatMessages.push({
    //     sender,
    //     content,
    //     timestamp: new Date().toISOString()
    // });
}

// 格式化消息内容
function formatMessage(content) {
    // 转义HTML特殊字符
    let formatted = content
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/\n/g, '<br>');
    
    // 处理代码块
    formatted = formatted.replace(/```(\w+)?\s*([\s\S]*?)```/g, (match, lang, code) => {
        return `<div class="code-block"><pre><code>${code.trim()}</code></pre></div>`;
    });
    
    // 处理行内代码
    formatted = formatted.replace(/`([^`]+)`/g, '<code>$1</code>');
    
    // 处理粗体和斜体
    formatted = formatted
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.*?)\*/g, '<em>$1</em>');
    
    return formatted;
}

// 显示打字指示器
function showTypingIndicator() {
    if (isTyping) return;
    
    isTyping = true;
    const chatContainer = document.getElementById('chatMessages');
    const typingDiv = document.createElement('div');
    typingDiv.className = 'message assistant-message';
    typingDiv.id = 'typing-indicator';
    
    typingDiv.innerHTML = `
        <div class="avatar">
            <i class="fas fa-robot"></i>
        </div>
        <div class="message-content">
            <div class="typing-indicator">
                <span>正在思考</span>
                <div class="typing-dots">
                    <div class="typing-dot"></div>
                    <div class="typing-dot"></div>
                    <div class="typing-dot"></div>
                </div>
            </div>
        </div>
    `;
    
    chatContainer.appendChild(typingDiv);
    chatContainer.scrollTop = chatContainer.scrollHeight;
}

// 隐藏打字指示器
function hideTypingIndicator() {
    const typingIndicator = document.getElementById('typing-indicator');
    if (typingIndicator) {
        typingIndicator.remove();
    }
    isTyping = false;
}

// 生成AI回复
function generateAIResponse(message) {
    const lowerMessage = message.toLowerCase();
    
    // 简单的意图识别
    if (lowerMessage.includes('状态') || lowerMessage.includes('怎么样') || lowerMessage.includes('分析')) {
        return generateStatusResponse();
    } else if (lowerMessage.includes('时间') || lowerMessage.includes('分配')) {
        return generateTimeResponse();
    } else if (lowerMessage.includes('目标') || lowerMessage.includes('进度')) {
        return generateGoalsResponse();
    } else if (lowerMessage.includes('建议') || lowerMessage.includes('推荐')) {
        return generateSuggestionsResponse();
    } else if (lowerMessage.includes('你好') || lowerMessage.includes('hello')) {
        return generateGreetingResponse();
    } else {
        return generateDefaultResponse(message);
    }
}

// 生成状态分析回复
function generateStatusResponse() {
    const template = responseTemplates.status.analysis.join('\n');
    return template
        .replace('{taskRate}', '78')
        .replace('{readingTime}', '2.1')
        .replace('{exerciseStatus}', '保持良好的运动频率')
        .replace('{suggestions}', '睡眠时间略显不足，建议调整作息')
        .replace('{recommendations}', '建议在下午3-5点处理重要任务，这是您的高效时段');
}

// 生成时间分析回复
function generateTimeResponse() {
    const template = responseTemplates.time.analysis.join('\n');
    return template
        .replace('{peakTime}', '下午3-5点')
        .replace('{workTime}', '6.5')
        .replace('{readingTime}', '2.1')
        .replace('{exerciseTime}', '1.2')
        .replace('{timeAdvice}', '建议将重要任务安排在高效时段，增加休息间隔');
}

// 生成目标进度回复
function generateGoalsResponse() {
    const template = responseTemplates.goals.analysis.join('\n');
    return template
        .replace('{readingProgress}', '65')
        .replace('{exerciseProgress}', '72')
        .replace('{writingProgress}', '45')
        .replace('{achievements}', '连续7天保持阅读习惯，完成3篇高质量博客')
        .replace('{nextActions}', '专注提升写作频率，继续保持运动习惯');
}

// 生成建议回复
function generateSuggestionsResponse() {
    const template = responseTemplates.suggestions.analysis.join('\n');
    return template
        .replace('{immediateActions}', '完成今天剩余的2个任务，安排30分钟阅读时间')
        .replace('{weeklyPlans}', '制定下周的详细学习计划，安排3次锻炼')
        .replace('{longTermGoals}', '建立更完善的知识管理系统，提高学习效率');
}

// 生成问候回复
function generateGreetingResponse() {
    const greetings = [
        '您好！我是您的智能助手，有什么可以帮助您的吗？',
        '您好！很高兴为您服务，我可以帮您分析数据、提供建议或管理任务。',
        '您好！我已经准备好为您提供个性化的数据分析和建议了。'
    ];
    return greetings[Math.floor(Math.random() * greetings.length)];
}

// 生成默认回复
function generateDefaultResponse(message) {
    const responses = [
        '这是一个有趣的问题，让我基于您的数据来分析一下...',
        '我理解您的需求，根据您的使用模式，我建议...',
        '基于您的历史数据，我可以为您提供以下见解...',
        '让我帮您分析一下这个问题，根据您的个人数据...'
    ];
    return responses[Math.floor(Math.random() * responses.length)] + '\n\n' + 
           '如果您需要具体的数据分析，可以尝试问我：\n' +
           '• "我最近的状态怎么样？"\n' +
           '• "帮我分析一下时间分配"\n' +
           '• "我的目标进度如何？"\n' +
           '• "给我一些建议"';
}

// 处理快速操作
function handleQuickAction(action) {
    const actions = {
        'status': '我最近的状态怎么样？',
        'time': '帮我分析一下时间分配',
        'goals': '我的目标进度如何？',
        'suggestions': '给我一些建议'
    };
    
    if (actions[action]) {
        document.getElementById('messageInput').value = actions[action];
        sendQuickMessage(actions[action], action);
    }
}

// 发送快速消息（带类型）
function sendQuickMessage(message, type) {
    if (!message || isTyping) return;
    
    // 添加用户消息到对话历史
    chatMessages.push({ role: "user", content: message });
    
    // 显示用户消息
    addMessage('user', message);
    document.getElementById('messageInput').value = '';
    
    // 创建AI消息占位符
    const aiMessageElement = createAiMessagePlaceholder();
    
    // 发送流式请求
    sendStreamingRequest(aiMessageElement);
}

// 处理快速操作
function handleQuickOperation(action) {
    const operations = {
        'new-task': '/todolist',
        'record-exercise': '/exercise',
        'write-blog': '/editor',
        'add-reading': '/reading'
    };
    
    if (operations[action]) {
        window.location.href = operations[action];
    }
}

// 更新今日统计
function updateTodayStats() {
    const stats = mockData.todayStats;
    
    document.getElementById('todayTasks').textContent = `${stats.tasks.completed}/${stats.tasks.total}`;
    document.getElementById('todayReading').textContent = `${stats.reading.time}h`;
    document.getElementById('todayExercise').textContent = stats.exercise.sessions > 0 ? '已完成' : '未完成';
    document.getElementById('todayBlogs').textContent = `${stats.blogs.count}篇`;
}

// 从API数据更新今日统计
function updateTodayStatsFromAPI(stats) {
    document.getElementById('todayTasks').textContent = `${stats.tasks.completed}/${stats.tasks.total}`;
    document.getElementById('todayReading').textContent = `${stats.reading.time}h`;
    document.getElementById('todayExercise').textContent = stats.exercise.sessions > 0 ? '已完成' : '未完成';
    document.getElementById('todayBlogs').textContent = `${stats.blogs.count}篇`;
}

// 更新建议列表
function updateSuggestions() {
    const suggestionsList = document.getElementById('suggestionsList');
    suggestionsList.innerHTML = '';
    
    mockData.suggestions.forEach(suggestion => {
        const suggestionDiv = document.createElement('div');
        suggestionDiv.className = 'suggestion-item';
        suggestionDiv.innerHTML = `
            <div class="suggestion-icon">${suggestion.icon}</div>
            <div class="suggestion-text">${suggestion.text}</div>
        `;
        suggestionsList.appendChild(suggestionDiv);
    });
}

// 从API数据更新建议列表
function updateSuggestionsFromAPI(suggestions) {
    const suggestionsList = document.getElementById('suggestionsList');
    suggestionsList.innerHTML = '';
    
    suggestions.forEach(suggestion => {
        const suggestionDiv = document.createElement('div');
        suggestionDiv.className = 'suggestion-item';
        suggestionDiv.innerHTML = `
            <div class="suggestion-icon">${suggestion.icon}</div>
            <div class="suggestion-text">${suggestion.text}</div>
        `;
        suggestionsList.appendChild(suggestionDiv);
    });
}

// 初始化趋势图表
function initializeTrendChart() {
    const ctx = document.getElementById('trendChart').getContext('2d');
    
    trendChart = new Chart(ctx, {
        type: 'line',
        data: mockData.trendData,
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: {
                        color: 'rgba(255, 255, 255, 0.8)',
                        font: {
                            size: 11
                        }
                    }
                }
            },
            scales: {
                x: {
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.6)',
                        font: {
                            size: 10
                        }
                    },
                    grid: {
                        color: 'rgba(255, 255, 255, 0.1)'
                    }
                },
                y: {
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.6)',
                        font: {
                            size: 10
                        }
                    },
                    grid: {
                        color: 'rgba(255, 255, 255, 0.1)'
                    }
                }
            },
            interaction: {
                intersect: false,
                mode: 'index'
            }
        }
    });
}

// 加载设置
function loadSettings() {
    const saved = localStorage.getItem('assistantSettings');
    if (saved) {
        currentSettings = {...currentSettings, ...JSON.parse(saved)};
    }
    
    // 应用设置到界面
    document.getElementById('enableNotifications').checked = currentSettings.enableNotifications;
    document.getElementById('enableSuggestions').checked = currentSettings.enableSuggestions;
    document.getElementById('analysisRange').value = currentSettings.analysisRange;
    document.getElementById('assistantPersonality').value = currentSettings.assistantPersonality;
}

// 设置监听器
function setupSettingsListeners() {
    const settings = ['enableNotifications', 'enableSuggestions', 'analysisRange', 'assistantPersonality'];
    
    settings.forEach(setting => {
        const element = document.getElementById(setting);
        if (element) {
            element.addEventListener('change', function() {
                currentSettings[setting] = element.type === 'checkbox' ? element.checked : element.value;
                saveSettings();
            });
        }
    });
}

// 保存设置
function saveSettings() {
    localStorage.setItem('assistantSettings', JSON.stringify(currentSettings));
    console.log('设置已保存', currentSettings);
}

// 刷新数据
function refreshData() {
    const refreshBtn = document.getElementById('refreshData');
    refreshBtn.style.transform = 'rotate(180deg)';
    
    setTimeout(() => {
        updateTodayStats();
        updateSuggestions();
        if (trendChart) {
            trendChart.update();
        }
        refreshBtn.style.transform = 'rotate(0deg)';
    }, 1000);
}

// 加载今日统计数据
function loadTodayStats() {
    console.log('正在加载今日统计数据...');
    
    fetch('/api/assistant/stats')
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                updateTodayStatsFromAPI(data.stats);
            } else {
                console.error('获取统计数据失败:', data);
                updateTodayStats(); // 使用模拟数据
            }
        })
        .catch(error => {
            console.error('API调用失败:', error);
            updateTodayStats(); // 使用模拟数据
        });
}

// 加载建议数据
function loadSuggestions() {
    console.log('正在加载智能建议...');
    
    fetch('/api/assistant/suggestions')
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                updateSuggestionsFromAPI(data.suggestions);
            } else {
                console.error('获取建议失败:', data);
                updateSuggestions(); // 使用模拟数据
            }
        })
        .catch(error => {
            console.error('API调用失败:', error);
            updateSuggestions(); // 使用模拟数据
        });
}

// 处理错误
function handleError(error) {
    console.error('智能助手错误:', error);
    addMessage('assistant', '抱歉，我遇到了一些问题。请稍后再试，或者刷新页面重新开始。');
}

// 工具函数
function formatTime(date) {
    return new Date(date).toLocaleTimeString('zh-CN', { 
        hour: '2-digit', 
        minute: '2-digit' 
    });
}

function formatDate(date) {
    return new Date(date).toLocaleDateString('zh-CN', { 
        month: 'long', 
        day: 'numeric' 
    });
}

// 导出功能供外部使用
window.AssistantApp = {
    sendMessage,
    addMessage,
    refreshData,
    handleError
};