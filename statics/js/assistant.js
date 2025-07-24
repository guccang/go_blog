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
let mcpTools = [];

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
    loadMCPTools();
});

// 初始化页面
function initializePage() {
    console.log('智能助手页面已加载');
    
    // 页面初始化完成，数据加载由其他函数处理
    console.log('页面初始化完成，等待API数据加载...');
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
    let toolCallCount = 0;
    let currentToolCall = null;
    
    try {
        const response = await fetch('/api/assistant/chat', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                messages: chatMessages,
                stream: true,
                selected_tools: getSelectedTools()
            })
        });
        
        if (!response.ok) {
            throw new Error('API请求失败');
        }
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let aiResponse = '';
        let buffer = '';
        
        // 开始流式读取
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n\n');
            buffer = lines.pop() || ''; // 保留最后一个不完整的行
            
            for (const line of lines) {
                if (line.trim() === '') continue;
                
                if (line.startsWith('data: ')) {
                    const data = line.replace('data: ', '');
                    if (data === '[DONE]') {
                        // 完成响应
                        chatMessages.push({ role: "assistant", content: aiResponse });
                        addTimestamp(aiMessageElement);
                        hideToolCallStatus(aiMessageElement);
                        return;
                    }
                    
                    try {
                        // 先将+替换为%20，再进行URL解码
                        const processedData = data.replace(/\+/g, '%20');
                        console.log('🟨 原始data:', data);
                        console.log('🟨 processedData:', processedData);
                        
                        const decodedContent = decodeURIComponent(processedData);
                        console.log('🟨 decodedContent:', JSON.stringify(decodedContent));
                        console.log('🟨 包含\\n:', decodedContent.includes('\n'));
                        console.log('🟨 包含\\r\\n:', decodedContent.includes('\r\n'));
                        
                        // 检测工具调用相关的内容，包括完整的工具调用和其碎片
                        const isToolCallContent = decodedContent.includes('[Calling tool ') || 
                                                decodedContent.includes(' with args ') ||
                                                decodedContent.trim() === ']' ||
                                                /^文件.*?的?内容如下：?\s*$/i.test(decodedContent.trim());
                        
                        if (decodedContent.includes('[Calling tool ') && decodedContent.includes(' with args ')) {
                            // 完整的工具调用检测
                            toolCallCount++;
                            const toolMatch = decodedContent.match(/\[Calling tool (\w+(?:\.\w+)*) with args (.*?)\]/);
                            if (toolMatch) {
                                currentToolCall = {
                                    name: toolMatch[1],
                                    args: toolMatch[2],
                                    count: toolCallCount
                                };
                                showToolCallStatus(aiMessageElement, currentToolCall);
                            }
                        } else if (!isToolCallContent && decodedContent.trim()) {
                            // 开始接收实际响应内容，隐藏工具调用状态
                            if (currentToolCall && decodedContent.length > 10) {
                                hideToolCallStatus(aiMessageElement);
                                currentToolCall = null;
                            }
                            // 只添加非工具调用相关的内容到响应中
                            aiResponse += decodedContent;
                            
                            console.log('✅ 添加到aiResponse:', JSON.stringify(decodedContent));
                        } else if (isToolCallContent) {
                            console.log('🚫 过滤工具调用内容:', JSON.stringify(decodedContent));
                        }
                        
                        // 更新消息内容
                        const messageText = aiMessageElement.querySelector('.message-text');
                        if (messageText) {
                            messageText.innerHTML = formatMessage(aiResponse);
                        }
                        
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

// 显示工具调用状态
function showToolCallStatus(messageElement, toolCall) {
    const messageText = messageElement.querySelector('.message-text');
    if (!messageText) return;
    
    // 移除现有的工具调用状态
    const existingStatus = messageElement.querySelector('.tool-call-status');
    if (existingStatus) {
        existingStatus.remove();
    }
    
    // 创建工具调用状态指示器
    const toolStatus = document.createElement('div');
    toolStatus.className = 'tool-call-status';
    toolStatus.innerHTML = `
        <div class="tool-call-indicator">
            <div class="tool-call-spinner">
                <i class="fas fa-cog fa-spin"></i>
            </div>
            <div class="tool-call-info">
                <div class="tool-call-title">
                    <i class="fas fa-tools"></i>
                    正在调用工具 ${toolCall.count} 
                </div>
                <div class="tool-call-details">
                    <strong>${toolCall.name}</strong>
                    <span class="tool-call-args">${formatToolArgs(toolCall.args)}</span>
                </div>
                <div class="tool-call-progress">
                    <div class="progress-bar">
                        <div class="progress-fill"></div>
                    </div>
                    <span class="progress-text">执行中...</span>
                </div>
            </div>
        </div>
    `;
    
    // 插入到消息内容之前
    messageText.style.display = 'none'; // 暂时隐藏普通内容
    messageElement.querySelector('.message-content').insertBefore(toolStatus, messageText);
    
    // 开始进度动画
    startProgressAnimation(toolStatus);
}

// 隐藏工具调用状态
function hideToolCallStatus(messageElement) {
    const toolStatus = messageElement.querySelector('.tool-call-status');
    const messageText = messageElement.querySelector('.message-text');
    
    if (toolStatus && messageText) {
        // 显示完成状态
        const progressText = toolStatus.querySelector('.progress-text');
        const progressFill = toolStatus.querySelector('.progress-fill');
        const spinner = toolStatus.querySelector('.tool-call-spinner i');
        
        if (progressText && progressFill && spinner) {
            progressText.textContent = '完成';
            progressFill.style.width = '100%';
            progressFill.style.background = '#00d4aa';
            spinner.className = 'fas fa-check';
            spinner.style.animation = 'none';
            spinner.style.color = '#00d4aa';
        }
        
        // 延迟移除状态并显示正常内容
        setTimeout(() => {
            toolStatus.style.opacity = '0';
            toolStatus.style.transform = 'translateY(-10px)';
            setTimeout(() => {
                toolStatus.remove();
                messageText.style.display = 'block';
            }, 300);
        }, 800);
    }
}

// 格式化工具参数显示
function formatToolArgs(args) {
    if (!args || args === '{}' || args === 'map[]') {
        return '无参数';
    }
    
    try {
        // 尝试解析并格式化JSON参数
        const parsed = JSON.parse(args.replace(/map\[(.*?)\]/, '{$1}'));
        const formatted = Object.entries(parsed)
            .map(([key, value]) => `${key}: ${JSON.stringify(value)}`)
            .join(', ');
        return formatted.length > 60 ? formatted.substring(0, 57) + '...' : formatted;
    } catch (e) {
        // 如果解析失败，直接显示原始参数（截断过长的）
        return args.length > 40 ? args.substring(0, 37) + '...' : args;
    }
}

// 开始进度条动画
function startProgressAnimation(statusElement) {
    const progressFill = statusElement.querySelector('.progress-fill');
    const progressText = statusElement.querySelector('.progress-text');
    
    if (!progressFill || !progressText) return;
    
    let progress = 0;
    const interval = setInterval(() => {
        progress += Math.random() * 15; // 随机增长
        if (progress > 90) progress = 90; // 最多到90%，等待实际完成
        
        progressFill.style.width = progress + '%';
        
        // 更新状态文本
        if (progress < 30) {
            progressText.textContent = '正在连接...';
        } else if (progress < 60) {
            progressText.textContent = '执行工具...';
        } else if (progress < 90) {
            progressText.textContent = '处理结果...';
        } else {
            progressText.textContent = '即将完成...';
            clearInterval(interval);
        }
    }, 300 + Math.random() * 200); // 300-500ms间隔
    
    // 存储interval引用以便清理
    statusElement.setAttribute('data-interval', interval);
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

// 使用与博客系统相同的Markdown渲染函数
function formatMessage(content) {
    if (!content) return '';
    
    console.log('🔵 formatMessage - 原始内容:');
    console.log(content);
    console.log('🔵 内容长度:', content.length);
    
    // 预处理：移除LLM返回内容中的代码块包裹
    let processedContent = preprocessLLMContent(content);
    
    console.log('🟡 formatMessage - 预处理后内容:');
    console.log(processedContent);
    console.log('🟡 处理后长度:', processedContent.length);
    
    // 检查marked库是否已加载
    if (typeof marked === 'undefined') {
        console.error('❌ marked.js library not loaded!');
        return processedContent.replace(/\n/g, '<br>');
    }
    
    try {
        // 初始化marked配置
        initializeMarkdown();
        
        // 使用marked渲染markdown
        let rendered;
        if (typeof marked.parse === 'function') {
            rendered = marked.parse(processedContent);
        } else if (typeof marked === 'function') {
            rendered = marked(processedContent);
        } else {
            throw new Error('No valid marked parsing method found');
        }
        
        console.log('🟢 formatMessage - 渲染后的HTML:');
        console.log(rendered);
        console.log('🟢 HTML长度:', rendered.length);
        
        return rendered;
        
    } catch (error) {
        console.error('❌ Error rendering markdown:', error);
        return processedContent.replace(/\n/g, '<br>');
    }
}

// 预处理LLM返回内容，移除代码块包裹
function preprocessLLMContent(content) {
    if (!content) return content;
    
    console.log('🔴 preprocessLLMContent - 开始预处理:');
    console.log(content);
    
    let processed = content;
    
    // 1. 匹配并移除 ```markdown ... ``` 或 ```md ... ``` (支持换行和不换行格式)
    const markdownBlockPattern = /```(?:markdown|md)\s*([\s\S]*?)\s*```/gi;
    let matches = processed.match(markdownBlockPattern);
    
    if (matches) {
        console.log('🟠 发现markdown代码块:', matches.length, '个');
        processed = processed.replace(markdownBlockPattern, (match, innerContent) => {
            console.log('🟠 移除markdown代码块包裹，内容:', innerContent.substring(0, 100) + '...');
            return innerContent.trim();
        });
    }
    
    // 2. 匹配并移除普通的 ``` ... ``` 代码块（当整个内容被包裹时）
    const genericCodeBlockPattern = /```\s*([\s\S]*?)\s*```/g;
    matches = processed.match(genericCodeBlockPattern);
    
    if (matches) {
        console.log('🟣 发现普通代码块包裹:', matches.length, '个');
        processed = processed.replace(genericCodeBlockPattern, (match, innerContent) => {
            console.log('🟣 移除普通代码块包裹，内容:', innerContent.substring(0, 100) + '...');
            return innerContent.trim();
        });
    }
    
    // 3. 移除开头的描述性文本
    processed = processed.replace(/^.*?文件.*?的?内容如下：?\s*\n*/i, '');
    processed = processed.replace(/^返回内容如上.*?\n*/i, '');
    processed = processed.replace(/^\[Calling tool.*?\]\s*/i, '');
    processed = processed.replace(/^\]\s*/i, '');
    
    // 4. 清理开头和结尾的多余空行
    processed = processed.trim();
    
    console.log('🟢 preprocessLLMContent - 预处理完成:');
    console.log(processed);
    
    return processed;
}

// 初始化Markdown配置（仅在需要时调用一次）
let markdownInitialized = false;
function initializeMarkdown() {
    if (markdownInitialized) return;
    
    try {
        // 先检查marked的可用方法
        if (typeof marked.use === 'function') {
            // 新版本marked (v4+)
            marked.use({
                gfm: true,
                tables: true,
                breaks: false,
                pedantic: false,
                smartLists: true,
                smartypants: false
            });
        } else if (typeof marked.setOptions === 'function') {
            // 旧版本marked
            const renderer = new marked.Renderer();
            marked.setOptions({
                renderer: renderer,
                gfm: true,
                tables: true,
                breaks: false,
                pedantic: false,
                sanitize: false,
                smartLists: true,
                smartypants: false
            });
        }
        
        markdownInitialized = true;
        
    } catch (error) {
        console.error('Failed to initialize markdown:', error);
    }
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
    // 现在从loadTodayStats函数调用真实API，这里不再使用mockData
    console.log('updateTodayStats called - deferring to API data');
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
    // 现在从loadSuggestions函数调用真实API，这里不再使用mockData
    console.log('updateSuggestions called - deferring to API data');
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
    
    // 首先加载真实数据，如果失败则使用模拟数据
    loadTrendData().then(trendData => {
        trendChart = new Chart(ctx, {
            type: 'line',
            data: trendData,
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
        loadTodayStats(); // 使用真实API调用
        loadSuggestions(); // 使用真实API调用
        loadTrendData().then(trendData => {
            if (trendChart) {
                trendChart.data = trendData;
                trendChart.update();
            }
        });
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
                updateTodayStatsFromMockData(); // 使用模拟数据
            }
        })
        .catch(error => {
            console.error('API调用失败:', error);
            updateTodayStatsFromMockData(); // 使用模拟数据
        });
}

// 从模拟数据更新今日统计（作为fallback）
function updateTodayStatsFromMockData() {
    const stats = {
        tasks: { completed: 3, total: 5 },
        reading: { time: 2.5, unit: 'hours' },
        exercise: { sessions: 1, type: 'cardio' },
        blogs: { count: 1, words: 800 }
    };
    updateTodayStatsFromAPI(stats);
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
                updateSuggestionsFromMockData(); // 使用模拟数据
            }
        })
        .catch(error => {
            console.error('API调用失败:', error);
            updateSuggestionsFromMockData(); // 使用模拟数据
        });
}

// 从模拟数据更新建议列表（作为fallback）
function updateSuggestionsFromMockData() {
    const suggestions = [
        { icon: '💡', text: '您今天的任务完成率为60%，建议优先处理剩余的重要任务' },
        { icon: '📚', text: '基于您的阅读习惯，推荐继续阅读《深度工作》' },
        { icon: '💪', text: '您已连续3天进行锻炼，保持良好的运动习惯' },
        { icon: '⏰', text: '分析显示您在下午3-5点效率最高，建议安排重要工作' }
    ];
    updateSuggestionsFromAPI(suggestions);
}

// 加载趋势数据
function loadTrendData() {
    console.log('正在加载趋势数据...');
    
    return fetch('/api/assistant/trends')
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                return data.trendData;
            } else {
                console.error('获取趋势数据失败:', data);
                return getMockTrendData();
            }
        })
        .catch(error => {
            console.error('趋势数据API调用失败:', error);
            return getMockTrendData();
        });
}

// 获取模拟趋势数据（作为fallback）
function getMockTrendData() {
    return {
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
    };
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

// 获取选中的工具
function getSelectedTools() {
    // 优先从大面板获取选择，如果大面板不存在则从小面板获取
    const selectedTools = [];
    
    // 先尝试从大面板获取
    const largeCheckboxes = document.querySelectorAll('.mcp-tool-checkbox-large:checked');
    if (largeCheckboxes.length > 0) {
        largeCheckboxes.forEach(checkbox => {
            selectedTools.push(checkbox.value);
        });
    } else {
        // 如果大面板没有选择，从小面板获取
        const smallCheckboxes = document.querySelectorAll('.mcp-tool-checkbox:not(.mcp-tool-checkbox-large):checked');
        smallCheckboxes.forEach(checkbox => {
            selectedTools.push(checkbox.value);
        });
    }
    
    // 如果没有选择任何工具，返回null表示使用所有可用工具
    return selectedTools.length > 0 ? selectedTools : null;
}

// 全选工具
function selectAllTools() {
    const checkboxes = document.querySelectorAll('.mcp-tool-checkbox');
    checkboxes.forEach(checkbox => {
        checkbox.checked = true;
    });
}

// 全不选工具
function selectNoTools() {
    const checkboxes = document.querySelectorAll('.mcp-tool-checkbox');
    checkboxes.forEach(checkbox => {
        checkbox.checked = false;
    });
}

// MCP工具相关函数
function loadMCPTools() {
    console.log('正在加载MCP工具...');
    
    // 并行获取工具列表和服务器状态
    Promise.all([
        fetch('/api/mcp/tools').then(r => r.json()),
        fetch('/api/mcp?action=status').then(r => r.json()).catch(() => ({ data: {} }))
    ])
    .then(([toolsResponse, statusResponse]) => {
        if (toolsResponse.success) {
            mcpTools = toolsResponse.data || [];
            const serverStatus = statusResponse.data || {};
            console.log('MCP工具加载成功:', mcpTools);
            console.log('服务器状态:', serverStatus);
            updateMCPToolsStatus(mcpTools, serverStatus);
            updateMCPToolsStatusLarge(mcpTools, serverStatus);
        } else {
            console.error('获取MCP工具失败:', toolsResponse.message);
            updateMCPToolsStatus([], {});
            updateMCPToolsStatusLarge([], {});
        }
    })
    .catch(error => {
        console.error('MCP工具API调用失败:', error);
        updateMCPToolsStatus([], {});
        updateMCPToolsStatusLarge([], {});
    });
}

function updateMCPToolsStatus(tools = [], serverStatus = {}) {
    const toolsContainer = document.getElementById('mcp-tools-status');
    if (!toolsContainer) return;
    
    if (tools.length === 0) {
        toolsContainer.innerHTML = `
            <div class="mcp-status-empty">
                <div style="font-size: 2rem; margin-bottom: 12px;">🔧</div>
                <div class="mcp-status-none">暂无配置的MCP工具</div>
                <div style="margin-top: 8px; font-size: 0.8rem; color: rgba(255, 255, 255, 0.6);">
                    MCP工具可以大大增强助手的功能
                </div>
                <a href="/mcp" class="mcp-config-link">
                    <i class="fas fa-plus"></i> 前往配置
                </a>
            </div>
        `;
        return;
    }
    
    // 按服务器分组显示工具
    const toolsByServer = groupToolsByServer(tools);
    const serversHtml = Object.keys(toolsByServer).map(serverName => {
        const serverTools = toolsByServer[serverName];
        const isConnected = serverStatus[serverName]?.connected || false;
        const isEnabled = serverStatus[serverName]?.enabled || false;
        const statusClass = isConnected ? 'connected' : (isEnabled ? 'disconnected' : 'disabled');
        
        return `
            <div class="mcp-server-item ${statusClass}">
                <div class="mcp-server-name">${serverName}</div>
                <div class="mcp-server-desc">${serverTools.length} 个工具可用</div>
            </div>
        `;
    }).join('');
    
    toolsContainer.innerHTML = `
        <div class="mcp-tools-header">
            <div class="mcp-tools-count">
                <i class="fas fa-tools"></i>
                ${tools.length} 个可用工具
            </div>
            <div style="display: flex; gap: 8px;">
                <button class="mcp-refresh-btn" onclick="loadMCPTools()">
                    <i class="fas fa-sync-alt"></i>
                </button>
                <button class="mcp-refresh-btn" onclick="toggleMCPToolsExpanded()">
                    <i class="fas fa-expand-arrows-alt"></i>
                </button>
            </div>
        </div>
        
        <div class="mcp-servers-list">
            ${serversHtml}
        </div>
        
        <div class="mcp-tools-search" style="margin-bottom: 12px;">
            <input type="text" id="mcp-tools-search" placeholder="搜索工具..." 
                   style="width: 100%; padding: 8px 12px; background: rgba(255,255,255,0.1); 
                          border: 1px solid rgba(255,255,255,0.2); border-radius: 6px; 
                          color: white; font-size: 0.9rem;"
                   onkeyup="filterMCPTools(this.value)">
        </div>
        
        <details class="mcp-tools-details" open>
            <summary>
                <i class="fas fa-list"></i> 工具选择 (${tools.length})
            </summary>
            <div class="mcp-tools-list" id="mcp-tools-list">
                ${tools.map(tool => {
                    const serverName = tool.name.split('.')[0];
                    const toolName = tool.name.split('.').slice(1).join('.');
                    return `
                        <div class="mcp-tool-item" data-tool-name="${tool.name.toLowerCase()}" data-server="${serverName.toLowerCase()}" data-desc="${(tool.description || '').toLowerCase()}">
                            <label class="mcp-tool-checkbox-label">
                                <input type="checkbox" class="mcp-tool-checkbox" value="${tool.name}">
                                <div class="mcp-tool-content">
                                    <div class="mcp-tool-name">
                                        <i class="fas fa-cog"></i>
                                        ${toolName}
                                        <span style="opacity: 0.6; font-size: 0.8em; margin-left: 8px;">(${serverName})</span>
                                    </div>
                                </div>
                            </label>
                        </div>
                    `;
                }).join('')}
            </div>
            <div class="mcp-tools-actions">
                <button class="mcp-tools-select-all" onclick="selectAllTools()">
                    <i class="fas fa-check-double"></i> 全选
                </button>
                <button class="mcp-tools-select-none" onclick="selectNoTools()">
                    <i class="fas fa-times"></i> 全不选
                </button>
            </div>
        </details>
        
        <div class="mcp-tools-stats" style="margin-top: 12px; padding: 8px; background: rgba(255,255,255,0.05); border-radius: 6px; font-size: 0.8rem; color: rgba(255,255,255,0.7);">
            <div style="display: flex; justify-content: space-between;">
                <span><i class="fas fa-check"></i> <span id="selected-tools-count">0</span> 已选择</span>
                <span><i class="fas fa-server"></i> ${Object.keys(toolsByServer).length} 个服务器</span>
            </div>
        </div>
    `;
    
    // 更新选中工具计数（默认全不选）
    updateSelectedToolsCount();
}

// 按服务器分组工具的辅助函数
function groupToolsByServer(tools) {
    const grouped = {};
    tools.forEach(tool => {
        const serverName = tool.name.split('.')[0];
        if (!grouped[serverName]) {
            grouped[serverName] = [];
        }
        grouped[serverName].push(tool);
    });
    return grouped;
}

function showMCPToolsDialog() {
    if (mcpTools.length === 0) {
        alert('暂无可用的MCP工具，请先在MCP配置页面添加工具配置。');
        return;
    }
    
    const toolsList = mcpTools.map(tool => `
        <div class="mcp-tool-option" data-tool-name="${tool.name}">
            <h4>${tool.name}</h4>
            <p>${tool.description}</p>
            ${tool.parameters ? `<details>
                <summary>参数说明</summary>
                <pre>${JSON.stringify(tool.parameters, null, 2)}</pre>
            </details>` : ''}
        </div>
    `).join('');
    
    const dialog = document.createElement('div');
    dialog.className = 'mcp-tools-dialog';
    dialog.innerHTML = `
        <div class="mcp-tools-dialog-content">
            <h3>可用的MCP工具</h3>
            <div class="mcp-tools-grid">${toolsList}</div>
            <div class="mcp-tools-dialog-actions">
                <button onclick="closeMCPToolsDialog()">关闭</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(dialog);
    
    // 添加工具选择事件
    dialog.querySelectorAll('.mcp-tool-option').forEach(option => {
        option.addEventListener('click', function() {
            const toolName = this.getAttribute('data-tool-name');
            const chatInput = document.getElementById('chat-input');
            if (chatInput) {
                chatInput.value = `请使用 ${toolName} 工具帮我`;
                chatInput.focus();
            }
            closeMCPToolsDialog();
        });
    });
}

function closeMCPToolsDialog() {
    const dialog = document.querySelector('.mcp-tools-dialog');
    if (dialog) {
        dialog.remove();
    }
}

// MCP工具搜索和过滤功能
function filterMCPTools(searchTerm) {
    const toolItems = document.querySelectorAll('.mcp-tool-item');
    const term = searchTerm.toLowerCase().trim();
    let visibleCount = 0;
    
    toolItems.forEach(item => {
        const toolName = item.dataset.toolName || '';
        const server = item.dataset.server || '';
        const desc = item.dataset.desc || '';
        
        const matches = toolName.includes(term) || 
                       server.includes(term) || 
                       desc.includes(term);
        
        if (matches || term === '') {
            item.style.display = 'flex';
            visibleCount++;
        } else {
            item.style.display = 'none';
        }
    });
    
    // 更新工具详情摘要
    const summary = document.querySelector('.mcp-tools-details summary');
    if (summary) {
        const totalCount = toolItems.length;
        if (term === '') {
            summary.innerHTML = `<i class="fas fa-list"></i> 工具选择 (${totalCount})`;
        } else {
            summary.innerHTML = `<i class="fas fa-search"></i> 搜索结果 (${visibleCount}/${totalCount})`;
        }
    }
}

// 切换MCP工具区域展开/收缩
function toggleMCPToolsExpanded() {
    const toolsCard = document.querySelector('#mcp-tools-status').closest('.info-card');
    const currentHeight = toolsCard.style.minHeight;
    
    if (currentHeight === '350px' || !currentHeight) {
        // 展开到更大
        toolsCard.style.minHeight = '500px';
        toolsCard.style.maxHeight = '70vh';
        
        // 更新工具列表最大高度
        const toolsList = document.querySelector('.mcp-tools-list');
        if (toolsList) {
            toolsList.style.maxHeight = '350px';
        }
        
        // 更新按钮图标
        const expandBtn = document.querySelector('button[onclick="toggleMCPToolsExpanded()"] i');
        if (expandBtn) {
            expandBtn.className = 'fas fa-compress-arrows-alt';
        }
    } else {
        // 收缩到正常大小
        toolsCard.style.minHeight = '350px';
        toolsCard.style.maxHeight = 'none';
        
        // 恢复工具列表最大高度
        const toolsList = document.querySelector('.mcp-tools-list');
        if (toolsList) {
            toolsList.style.maxHeight = '200px';
        }
        
        // 更新按钮图标
        const expandBtn = document.querySelector('button[onclick="toggleMCPToolsExpanded()"] i');
        if (expandBtn) {
            expandBtn.className = 'fas fa-expand-arrows-alt';
        }
    }
}

// 更新选中工具计数
function updateSelectedToolsCount() {
    const selectedCheckboxes = document.querySelectorAll('.mcp-tool-checkbox:checked');
    const countElement = document.getElementById('selected-tools-count');
    if (countElement) {
        countElement.textContent = selectedCheckboxes.length;
    }
}

// 重写全选和全不选函数，添加计数更新
function selectAllTools() {
    const checkboxes = document.querySelectorAll('.mcp-tool-checkbox');
    checkboxes.forEach(checkbox => {
        checkbox.checked = true;
    });
    updateSelectedToolsCount();
}

function selectNoTools() {
    const checkboxes = document.querySelectorAll('.mcp-tool-checkbox');
    checkboxes.forEach(checkbox => {
        checkbox.checked = false;
    });
    updateSelectedToolsCount();
}

// 大面板更新函数
function updateMCPToolsStatusLarge(tools = [], serverStatus = {}) {
    const toolsContainer = document.getElementById('mcp-tools-status-large');
    if (!toolsContainer) return;
    
    if (tools.length === 0) {
        toolsContainer.innerHTML = `
            <div class="mcp-status-empty">
                <div style="font-size: 4rem; margin-bottom: 20px;">🔧</div>
                <div class="mcp-status-none">暂无配置的MCP工具</div>
                <div style="margin-top: 12px; font-size: 1rem; color: rgba(255, 255, 255, 0.6);">
                    MCP工具可以大大增强助手的功能，支持文件系统、数据库等多种工具类型
                </div>
                <a href="/mcp" class="mcp-config-link">
                    <i class="fas fa-plus"></i> 前往配置
                </a>
            </div>
        `;
        return;
    }
    
    // 按服务器分组显示工具
    const toolsByServer = groupToolsByServer(tools);
    const serversHtml = Object.keys(toolsByServer).map(serverName => {
        const serverTools = toolsByServer[serverName];
        const isConnected = serverStatus[serverName]?.connected || false;
        const isEnabled = serverStatus[serverName]?.enabled || false;
        const statusClass = isConnected ? 'connected' : (isEnabled ? 'disconnected' : 'disabled');
        
        return `
            <div class="mcp-server-item ${statusClass}">
                <div class="mcp-server-name">${serverName}</div>
                <div class="mcp-server-desc">${serverTools.length} 个工具可用</div>
            </div>
        `;
    }).join('');
    
    toolsContainer.innerHTML = `
        <div class="mcp-tools-large-grid">
            <div class="mcp-servers-section">
                <h4><i class="fas fa-server"></i> 服务器状态</h4>
                <div class="mcp-servers-list">
                    ${serversHtml}
                </div>
                <div class="mcp-tools-stats" style="margin-top: 15px; padding: 12px; background: rgba(255,255,255,0.08); border-radius: 6px; font-size: 0.9rem; color: rgba(255,255,255,0.8);">
                    <div style="display: flex; justify-content: space-between; margin-bottom: 8px;">
                        <span><i class="fas fa-server"></i> ${Object.keys(toolsByServer).length} 个服务器</span>
                        <span><i class="fas fa-tools"></i> ${tools.length} 个工具</span>
                    </div>
                    <div style="display: flex; justify-content: space-between;">
                        <span><i class="fas fa-check"></i> <span id="selected-tools-count-large">0</span> 已选择</span>
                        <span><i class="fas fa-sync-alt"></i> <button onclick="loadMCPTools()" style="background: none; border: none; color: #00d4aa; cursor: pointer; font-size: 0.9rem;">刷新</button></span>
                    </div>
                </div>
            </div>
            
            <div class="mcp-tools-section">
                <h4><i class="fas fa-list"></i> 工具管理</h4>
                
                <div class="mcp-tools-search-large">
                    <input type="text" id="mcp-tools-search-large" placeholder="搜索工具名称、服务器或描述..." 
                           onkeyup="filterMCPToolsLarge(this.value)">
                </div>
                
                <div class="mcp-tools-list-large" id="mcp-tools-list-large">
                    ${tools.map(tool => {
                        const serverName = tool.name.split('.')[0];
                        const toolName = tool.name.split('.').slice(1).join('.');
                        return `
                            <div class="mcp-tool-item" data-tool-name="${tool.name.toLowerCase()}" data-server="${serverName.toLowerCase()}" data-desc="${(tool.description || '').toLowerCase()}">
                                <label class="mcp-tool-checkbox-label">
                                    <input type="checkbox" class="mcp-tool-checkbox mcp-tool-checkbox-large" value="${tool.name}">
                                    <div class="mcp-tool-content">
                                        <div class="mcp-tool-name">
                                            <i class="fas fa-cog"></i>
                                            ${toolName}
                                            <span style="opacity: 0.6; font-size: 0.85em; margin-left: 8px;">(${serverName})</span>
                                        </div>
                                        </div>
                                </label>
                            </div>
                        `;
                    }).join('')}
                </div>
                
                <div class="mcp-tools-actions-large">
                    <button class="mcp-tools-select-all" onclick="selectAllToolsLarge()">
                        <i class="fas fa-check-double"></i> 全选
                    </button>
                    <button class="mcp-tools-select-none" onclick="selectNoToolsLarge()">
                        <i class="fas fa-times"></i> 全不选
                    </button>
                    <button class="mcp-tools-select-all" onclick="syncToolsSelection()" style="background: rgba(161, 196, 253, 0.2); border-color: rgba(161, 196, 253, 0.4);">
                        <i class="fas fa-sync"></i> 同步选择
                    </button>
                </div>
            </div>
        </div>
    `;
    
    // 更新大面板选中工具计数（默认全不选）
    updateSelectedToolsCountLarge();
}

// 大面板搜索功能
function filterMCPToolsLarge(searchTerm) {
    const toolItems = document.querySelectorAll('#mcp-tools-list-large .mcp-tool-item');
    const term = searchTerm.toLowerCase().trim();
    let visibleCount = 0;
    
    toolItems.forEach(item => {
        const toolName = item.dataset.toolName || '';
        const server = item.dataset.server || '';
        const desc = item.dataset.desc || '';
        
        const matches = toolName.includes(term) || 
                       server.includes(term) || 
                       desc.includes(term);
        
        if (matches || term === '') {
            item.style.display = 'flex';
            visibleCount++;
        } else {
            item.style.display = 'none';
        }
    });
    
    // 更新工具部分标题
    const toolsSection = document.querySelector('.mcp-tools-section h4');
    if (toolsSection) {
        const totalCount = toolItems.length;
        if (term === '') {
            toolsSection.innerHTML = `<i class="fas fa-list"></i> 工具管理`;
        } else {
            toolsSection.innerHTML = `<i class="fas fa-search"></i> 搜索结果 (${visibleCount}/${totalCount})`;
        }
    }
}

// 大面板工具选择函数
function selectAllToolsLarge() {
    const checkboxes = document.querySelectorAll('.mcp-tool-checkbox-large');
    checkboxes.forEach(checkbox => {
        checkbox.checked = true;
    });
    updateSelectedToolsCountLarge();
}

function selectNoToolsLarge() {
    const checkboxes = document.querySelectorAll('.mcp-tool-checkbox-large');
    checkboxes.forEach(checkbox => {
        checkbox.checked = false;
    });
    updateSelectedToolsCountLarge();
}

// 同步大面板和小面板的选择
function syncToolsSelection() {
    const largeCheckboxes = document.querySelectorAll('.mcp-tool-checkbox-large');
    const smallCheckboxes = document.querySelectorAll('.mcp-tool-checkbox:not(.mcp-tool-checkbox-large)');
    
    // 从大面板同步到小面板
    largeCheckboxes.forEach(largeCheckbox => {
        const toolName = largeCheckbox.value;
        const smallCheckbox = Array.from(smallCheckboxes).find(cb => cb.value === toolName);
        if (smallCheckbox) {
            smallCheckbox.checked = largeCheckbox.checked;
        }
    });
    
    updateSelectedToolsCount();
    updateSelectedToolsCountLarge();
}

// 更新大面板选中工具计数
function updateSelectedToolsCountLarge() {
    const selectedCheckboxes = document.querySelectorAll('.mcp-tool-checkbox-large:checked');
    const countElement = document.getElementById('selected-tools-count-large');
    if (countElement) {
        countElement.textContent = selectedCheckboxes.length;
    }
}

// 切换大面板展开/收缩
function toggleMCPPanel() {
    const panel = document.querySelector('.mcp-tools-panel');
    const toggleButton = document.querySelector('.mcp-panel-toggle i');
    
    if (panel.classList.contains('collapsed')) {
        panel.classList.remove('collapsed');
        toggleButton.className = 'fas fa-chevron-up';
    } else {
        panel.classList.add('collapsed');
        toggleButton.className = 'fas fa-chevron-down';
    }
}

// 添加工具选择变化监听
document.addEventListener('change', function(e) {
    if (e.target.classList.contains('mcp-tool-checkbox')) {
        updateSelectedToolsCount();
        if (e.target.classList.contains('mcp-tool-checkbox-large')) {
            updateSelectedToolsCountLarge();
        }
    }
});

// 导出功能供外部使用
window.AssistantApp = {
    sendMessage,
    addMessage,
    refreshData,
    handleError,
    loadMCPTools,
    showMCPToolsDialog,
    filterMCPTools,
    filterMCPToolsLarge,
    toggleMCPToolsExpanded,
    toggleMCPPanel,
    selectAllTools,
    selectNoTools,
    selectAllToolsLarge,
    selectNoToolsLarge,
    syncToolsSelection
};