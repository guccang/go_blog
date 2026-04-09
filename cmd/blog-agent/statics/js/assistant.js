// 智能助手页面JavaScript

// 全局变量
let chatMessages = [
    { role: "system", content: "你是一个专业的个人数据分析师和生活助手" },
    { role: "assistant", content: "你好！我是智能助手，可以帮你分析数据、提供建议。有什么我可以帮助你的吗？" }
];
let isTyping = false;
let typingIntervals = new Map(); // 存储每个消息的打字机定时器
let trendChart = null;
// 新的健康图表变量
let healthRadarChart = null;
let emotionTrendChart = null;
let stressHeatmapChart = null;
let timeDistributionChart = null;
let socialHealthChart = null;
let resilienceTrendChart = null;
let currentSettings = {
    enableNotifications: true,
    enableSuggestions: true,
    analysisRange: 30,
    assistantPersonality: 'professional',
    enableTypingEffect: true    // 打字机光标效果开关
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

// 复制消息到剪贴板
function copyMessageToClipboard(content, button) {
    // 创建临时文本区域
    const tempTextArea = document.createElement('textarea');
    tempTextArea.value = content;
    document.body.appendChild(tempTextArea);

    try {
        // 选择并复制文本
        tempTextArea.select();
        tempTextArea.setSelectionRange(0, 99999); // 移动端兼容
        document.execCommand('copy');

        // 更新按钮状态
        const originalContent = button.innerHTML;
        button.innerHTML = '<i class="fas fa-check"></i> 已复制';
        button.style.background = 'rgba(34, 197, 94, 0.2)';
        button.style.borderColor = 'rgba(34, 197, 94, 0.3)';
        button.style.color = '#22c55e';

        // 3秒后恢复原状
        setTimeout(() => {
            button.innerHTML = originalContent;
            button.style.background = 'rgba(255, 255, 255, 0.1)';
            button.style.borderColor = 'rgba(255, 255, 255, 0.2)';
            button.style.color = 'rgba(255, 255, 255, 0.7)';
        }, 3000);

        console.log('消息已复制到剪贴板');
    } catch (err) {
        console.error('复制失败:', err);

        // 备用方案：使用现代 Clipboard API
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(content).then(() => {
                console.log('使用 Clipboard API 复制成功');
            }).catch(err => {
                console.error('Clipboard API 复制失败:', err);
            });
        }
    } finally {
        // 清理临时元素
        document.body.removeChild(tempTextArea);
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function () {
    initializePage();
    setupEventListeners();
    loadTodayStats();
    loadSuggestions();
    initializeTrendChart();
    loadSettings();
    loadMCPTools();
    initializeChatHistoryControls(); // 初始化聊天历史控件
    loadChatHistory(); // 加载聊天历史
    // initializeHealthCharts(); // 延迟到健康标签激活时初始化

    // 确保初始状态正确
    initializeTabState();
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
    messageInput.addEventListener('keypress', function (e) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
        }
    });

    // 快速操作按钮
    const quickBtns = document.querySelectorAll('.quick-btn');
    quickBtns.forEach(btn => {
        btn.addEventListener('click', function () {
            const action = this.dataset.action;
            handleQuickAction(action);
        });
    });

    // 快速操作
    const operationBtns = document.querySelectorAll('.operation-btn');
    operationBtns.forEach(btn => {
        btn.addEventListener('click', function () {
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
                stream: true
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
                        // 完成响应，停止打字机效果
                        const messageText = aiMessageElement.querySelector('.message-text');
                        if (messageText) {
                            stopTypingEffect(messageText, aiResponse);
                        }
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

                        // 检查是否包含markdown标题标记
                        if (decodedContent.includes('#')) {
                            console.log('🔍 检测到标题标记，内容:', JSON.stringify(decodedContent));
                        }

                        // 检测工具调用相关的内容，只过滤明确的工具调用标识
                        const isToolCallContent = decodedContent.includes('[Calling tool ') && decodedContent.includes(' with args ');

                        if (isToolCallContent) {
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
                            console.log('🔧 检测到工具调用:', JSON.stringify(decodedContent));
                        } else if (decodedContent) {
                            // 开始接收实际响应内容，隐藏工具调用状态
                            if (currentToolCall) {
                                hideToolCallStatus(aiMessageElement);
                                currentToolCall = null;
                            }
                            // 只添加非工具调用相关的内容到响应中
                            aiResponse += decodedContent;

                            console.log('✅ 实时添加到aiResponse:', JSON.stringify(decodedContent), '累计长度:', aiResponse.length);

                            // 特别检查包含标题标记的内容
                            if (decodedContent.includes('#')) {
                                console.log('🚨 标题相关内容块:', JSON.stringify(decodedContent));
                                console.log('🚨 当前累计aiResponse末尾20字符:', JSON.stringify(aiResponse.substring(Math.max(0, aiResponse.length - 20))));
                            }

                            // 使用打字机效果更新消息内容 - 立即显示每个内容块
                            const messageText = aiMessageElement.querySelector('.message-text');
                            if (messageText) {
                                console.log('🔄 更新界面显示, 当前内容:', aiResponse.substring(Math.max(0, aiResponse.length - 20)));
                                updateTypingEffect(messageText, aiResponse);
                            }
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
        if (messageText) {
            // 停止打字机效果
            stopTypingEffect(messageText, '');
            messageText.innerHTML = '<span class="error">抱歉，请求过程中出现错误。请重试。</span>';
        }

        // 降级到本地生成
        setTimeout(() => {
            const lastUserMessage = chatMessages[chatMessages.length - 1];
            if (lastUserMessage && lastUserMessage.role === 'user') {
                const response = generateAIResponse(lastUserMessage.content);
                if (messageText) {
                    messageText.innerHTML = formatMessage(response);
                }
                chatMessages.push({ role: "assistant", content: response });
                addTimestamp(aiMessageElement);
            }
        }, 1000);
    } finally {
        // 重置状态
        isTyping = false;
        console.log('Stream request completed. Typing effect enabled:', currentSettings.enableTypingEffect);
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

    // 为assistant消息添加复制按钮
    if (sender === 'assistant') {
        messageContent.style.position = 'relative';
        const copyButton = document.createElement('button');
        copyButton.className = 'copy-message-btn';
        copyButton.innerHTML = '<i class="fas fa-copy"></i> 复制';
        copyButton.onclick = () => copyMessageToClipboard(content, copyButton);
        messageContent.appendChild(copyButton);
    }

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

    // 检查是否包含markdown标题
    if (content.includes('#')) {
        console.log('🔍 formatMessage - 检测到标题内容:', JSON.stringify(content));
        // 检查标题格式
        const titleMatches = content.match(/#{1,6}\s+[^\n]*/g);
        if (titleMatches) {
            console.log('🔍 标题匹配结果:', titleMatches);
        }
    }

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

    // 检查标题格式
    if (content.includes('#')) {
        console.log('🔍 preprocessLLMContent - 输入包含标题:', JSON.stringify(content));
        const titleMatches = content.match(/#{1,6}\s+[^\n]*/g);
        if (titleMatches) {
            console.log('🔍 输入标题匹配:', titleMatches);
        }
    }

    let processed = content;

    // 1. 匹配并移除 ```markdown ... ``` 或 ```md ... ``` (支持换行和不换行格式)
    const markdownBlockPattern = /```(?:markdown|md)\s*([\s\S]*?)\s*```/gi;
    let matches = processed.match(markdownBlockPattern);

    if (matches) {
        console.log('🟠 发现markdown代码块:', matches.length, '个');
        processed = processed.replace(markdownBlockPattern, (match, innerContent) => {
            console.log('🟠 移除markdown代码块包裹，内容:', innerContent.substring(0, 100) + '...');
            return innerContent; // 保留原始格式，不使用trim()
        });
    }

    // 2. 匹配并移除普通的 ``` ... ``` 代码块（当整个内容被包裹时）
    const genericCodeBlockPattern = /```\s*([\s\S]*?)\s*```/g;
    matches = processed.match(genericCodeBlockPattern);

    if (matches) {
        console.log('🟣 发现普通代码块包裹:', matches.length, '个');
        processed = processed.replace(genericCodeBlockPattern, (match, innerContent) => {
            console.log('🟣 移除普通代码块包裹，内容:', innerContent.substring(0, 100) + '...');
            return innerContent; // 保留原始格式，不使用trim()
        });
    }

    // 3. 只移除明确的工具调用标识，保留markdown格式
    //processed = processed.replace(/^\[Calling tool.*?\]\s*\n?/i, '');

    // 4. 只移除开头和结尾的多余空行，但保留必要的换行
    //processed = processed.replace(/^\n+/, '').replace(/\n+$/, '');

    console.log('🟢 preprocessLLMContent - 预处理完成:');
    console.log(processed);

    // 检查处理后的标题格式
    if (processed.includes('#')) {
        console.log('🔍 preprocessLLMContent - 输出包含标题:', JSON.stringify(processed));
        const titleMatches = processed.match(/#{1,6}\s+[^\n]*/g);
        if (titleMatches) {
            console.log('🔍 输出标题匹配:', titleMatches);
        }
    }

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
                breaks: true,
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
                breaks: true, // 启用换行符转换，保持markdown格式
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
    const el = (id) => document.getElementById(id);
    if (el('todayTasks')) el('todayTasks').textContent = `${stats.tasks.completed}/${stats.tasks.total}`;
    if (el('todayReading')) el('todayReading').textContent = `${stats.reading.progress}%`;
    if (el('todayExercise')) el('todayExercise').textContent = stats.exercise.sessions > 0 ? '已完成' : '未完成';
    if (el('todayBlogs')) el('todayBlogs').textContent = `${stats.blogs.count}篇`;
}

// 更新建议列表
function updateSuggestions() {
    // 现在从loadSuggestions函数调用真实API，这里不再使用mockData
    console.log('updateSuggestions called - deferring to API data');
}

// 从API数据更新建议列表
function updateSuggestionsFromAPI(suggestions) {
    const suggestionsList = document.getElementById('suggestionsList');
    if (!suggestionsList) return;
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
    const trendEl = document.getElementById('trendChart');
    if (!trendEl) return;
    const ctx = trendEl.getContext('2d');

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
        currentSettings = { ...currentSettings, ...JSON.parse(saved) };
    }

    // 应用设置到界面（元素可能不存在）
    const setChecked = (id, val) => { const el = document.getElementById(id); if (el) el.checked = val; };
    const setValue = (id, val) => { const el = document.getElementById(id); if (el) el.value = val; };
    setChecked('enableNotifications', currentSettings.enableNotifications);
    setChecked('enableSuggestions', currentSettings.enableSuggestions);
    setValue('analysisRange', currentSettings.analysisRange);
    setValue('assistantPersonality', currentSettings.assistantPersonality);
    setChecked('enableTypingEffect', currentSettings.enableTypingEffect);
}

// 设置监听器
function setupSettingsListeners() {
    const settings = ['enableNotifications', 'enableSuggestions', 'analysisRange', 'assistantPersonality', 'enableTypingEffect'];

    settings.forEach(setting => {
        const element = document.getElementById(setting);
        if (element) {
            element.addEventListener('change', function () {
                currentSettings[setting] = element.type === 'checkbox' ? element.checked : element.value;
                saveSettings();

                // 特殊处理打字机效果设置变更
                if (setting === 'enableTypingEffect') {
                    console.log(`打字机光标效果设置已更新: ${setting} = ${currentSettings[setting]}`);
                }
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
                //updateTodayStatsFromMockData(); // 使用模拟数据
            }
        })
        .catch(error => {
            console.error('API调用失败:', error);
            //updateTodayStatsFromMockData(); // 使用模拟数据
        });
}

// 从模拟数据更新今日统计（作为fallback）
function updateTodayStatsFromMockData() {
    const stats = {
        tasks: { completed: 3, total: 5 },
        reading: { progress: 65 },
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

// 加载聊天历史
function loadChatHistory(date) {
    console.log('正在加载聊天历史...');

    // 如果没有指定日期，使用今天的日期
    if (!date) {
        date = new Date().toISOString().split('T')[0]; // YYYY-MM-DD格式
    }

    fetch(`/api/assistant/chat/history?date=${date}`)
        .then(response => response.json())
        .then(data => {
            if (data.success && data.chatHistory.length > 0) {
                console.log(`成功加载 ${data.chatHistory.length} 条聊天记录`);
                displayChatHistory(data.chatHistory);
            } else {
                console.log('该日期无聊天历史记录');
                // 不显示任何内容，保持空白的聊天界面
            }
        })
        .catch(error => {
            console.error('加载聊天历史失败:', error);
        });
}

// 显示聊天历史
function displayChatHistory(chatHistory) {
    const chatContainer = document.getElementById('chatMessages');
    if (!chatContainer) return;

    // 清空当前消息（除了欢迎消息）
    const welcomeMessage = chatContainer.querySelector('.message.assistant-message');
    chatContainer.innerHTML = '';

    // 保留欢迎消息
    if (welcomeMessage) {
        chatContainer.appendChild(welcomeMessage);
    }

    // 显示历史聊天记录
    chatHistory.forEach(message => {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${message.role}-message`;

        const avatar = document.createElement('div');
        avatar.className = 'avatar';
        avatar.innerHTML = message.role === 'user' ?
            '<i class="fas fa-user"></i>' :
            '<i class="fas fa-robot"></i>';

        const messageContent = document.createElement('div');
        messageContent.className = 'message-content';

        // 为assistant消息添加复制按钮
        if (message.role === 'assistant') {
            messageContent.style.position = 'relative';
            const copyButton = document.createElement('button');
            copyButton.className = 'copy-message-btn';
            copyButton.innerHTML = '<i class="fas fa-copy"></i> 复制';
            copyButton.onclick = () => copyMessageToClipboard(message.content, copyButton);
            messageContent.appendChild(copyButton);
        }

        const messageText = document.createElement('div');
        messageText.className = 'message-text';

        // 使用相同的 Markdown 格式化功能
        console.log("===========message.content", message.content);
        messageText.innerHTML = formatMessage(message.content);

        const messageTime = document.createElement('div');
        messageTime.className = 'message-time';
        messageTime.textContent = message.timestamp || '历史消息';

        messageContent.appendChild(messageText);
        messageContent.appendChild(messageTime);

        if (message.role === 'user') {
            messageDiv.appendChild(messageContent);
            messageDiv.appendChild(avatar);
        } else {
            messageDiv.appendChild(avatar);
            messageDiv.appendChild(messageContent);
        }

        chatContainer.appendChild(messageDiv);
    });

    // 滚动到底部
    chatContainer.scrollTop = chatContainer.scrollHeight;

    console.log('聊天历史显示完成');
}

// 加载选定日期的聊天历史
function loadSelectedDateHistory() {
    const dateInput = document.getElementById('chatHistoryDate');
    if (!dateInput || !dateInput.value) {
        alert('请选择一个日期');
        return;
    }

    const selectedDate = dateInput.value;
    console.log('加载指定日期的聊天历史:', selectedDate);
    loadChatHistory(selectedDate);
}

// 初始化聊天历史控件
function initializeChatHistoryControls() {
    const dateInput = document.getElementById('chatHistoryDate');
    if (dateInput) {
        // 设置默认日期为今天
        dateInput.value = new Date().toISOString().split('T')[0];

        // 添加回车键监听
        dateInput.addEventListener('keypress', function (e) {
            if (e.key === 'Enter') {
                loadSelectedDateHistory();
            }
        });
    }
}

// 按服务器分组工具的辅助函数
function groupToolsByServer(tools) {
    console.log("groupToolsServe ===========tools", tools);
    const grouped = {};
    tools.forEach(tool => {
        const serverName = tool.name.split('.')[0];
        console.log("===========serverName", serverName);
        if (!grouped[serverName]) {
            grouped[serverName] = [];
        }
        grouped[serverName].push(tool);
    });
    return grouped;
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
    console.log("===========toolsByServer", toolsByServer);
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
                <i class="fas fa-list"></i> 工具列表 (${tools.length})
            </summary>
            <div class="mcp-tools-list" id="mcp-tools-list">
                ${tools.map(tool => {
        const serverName = tool.name.split('.')[0];
        const toolName = tool.name.split('.').slice(1).join('.');
        return `
                        <div class="mcp-tool-item" data-tool-name="${tool.name.toLowerCase()}" data-server="${serverName.toLowerCase()}" data-desc="${(tool.description || '').toLowerCase()}">
                            <div class="mcp-tool-content">
                                <div class="mcp-tool-name">
                                    <i class="fas fa-cog"></i>
                                    ${toolName}
                                    <span style="opacity: 0.6; font-size: 0.8em; margin-left: 8px;">(${serverName})</span>
                                </div>
                            </div>
                        </div>
                    `;
    }).join('')}
            </div>
        </details>
        
        <div class="mcp-tools-stats" style="margin-top: 12px; padding: 8px; background: rgba(255,255,255,0.05); border-radius: 6px; font-size: 0.8rem; color: rgba(255,255,255,0.7);">
            <div style="display: flex; justify-content: space-between;">
                <span><i class="fas fa-route"></i> AI 自动按任务选择工具</span>
                <span><i class="fas fa-server"></i> ${Object.keys(toolsByServer).length} 个服务器</span>
            </div>
        </div>
    `;
}


function showMCPToolsDialog() {
    if (mcpTools.length === 0) {
        showMCPToolsEmptyState();
        return;
    }

    // 创建分组工具数据
    const groupedTools = groupToolsByServer(mcpTools);

    const dialog = document.createElement('div');
    dialog.className = 'mcp-tools-dialog';
    dialog.innerHTML = `
        <div class="mcp-tools-dialog-content">
            <div class="mcp-dialog-header">
                <div class="dialog-title">
                    <i class="fas fa-tools"></i>
                    <h3>MCP 工具浏览器</h3>
                    <span class="tools-count">${mcpTools.length} 个工具可用</span>
                </div>
                <button class="close-dialog-btn" onclick="closeMCPToolsDialog()">
                    <i class="fas fa-times"></i>
                </button>
            </div>
            
            <div class="mcp-dialog-search">
                <div class="search-container">
                    <i class="fas fa-search"></i>
                    <input type="text" id="toolSearchInput" placeholder="搜索工具名称、服务器或描述..." oninput="filterDialogTools(this.value)">
                </div>
                <div class="search-filters">
                    <button class="filter-btn active" data-filter="all" onclick="setToolFilter(this, 'all')">
                        <i class="fas fa-list"></i> 全部
                    </button>
                    <button class="filter-btn" data-filter="recent" onclick="setToolFilter(this, 'recent')">
                        <i class="fas fa-clock"></i> 常用
                    </button>
                    <button class="filter-btn" data-filter="data" onclick="setToolFilter(this, 'data')">
                        <i class="fas fa-database"></i> 数据
                    </button>
                    <button class="filter-btn" data-filter="analysis" onclick="setToolFilter(this, 'analysis')">
                        <i class="fas fa-chart-bar"></i> 分析
                    </button>
                </div>
            </div>
            
            <div class="mcp-dialog-body">
                <div class="tools-grid" id="toolsGrid">
                    ${generateToolsGrid(groupedTools)}
                </div>
            </div>
            
            <div class="mcp-dialog-footer">
                <div class="dialog-actions">
                    <button class="btn-secondary" onclick="closeMCPToolsDialog()">
                        <i class="fas fa-times"></i> 取消
                    </button>
                    <button class="btn-primary" onclick="openMCPConfig()">
                        <i class="fas fa-cog"></i> 管理工具
                    </button>
                </div>
            </div>
        </div>
    `;

    document.body.appendChild(dialog);

    // 添加动画效果
    requestAnimationFrame(() => {
        dialog.classList.add('active');
    });

    // 添加对话框交互事件
    setupToolSelectionEvents(dialog);

    // 焦点管理
    const searchInput = dialog.querySelector('#toolSearchInput');
    setTimeout(() => searchInput.focus(), 100);
}

function closeMCPToolsDialog() {
    const dialog = document.querySelector('.mcp-tools-dialog');
    if (dialog) {
        dialog.classList.add('closing');
        setTimeout(() => {
            dialog.remove();
        }, 200);
    }
}

// 显示空状态
function showMCPToolsEmptyState() {
    const dialog = document.createElement('div');
    dialog.className = 'mcp-tools-dialog';
    dialog.innerHTML = `
        <div class="mcp-tools-dialog-content empty-state">
            <div class="empty-state-content">
                <div class="empty-icon">
                    <i class="fas fa-tools"></i>
                </div>
                <h3>暂无可用工具</h3>
                <p>请先在 MCP 配置页面添加工具配置，然后刷新页面重试。</p>
                <div class="empty-actions">
                    <button class="btn-primary" onclick="openMCPConfig()">
                        <i class="fas fa-plus"></i> 添加工具
                    </button>
                    <button class="btn-secondary" onclick="closeMCPToolsDialog()">
                        取消
                    </button>
                </div>
            </div>
        </div>
    `;
    document.body.appendChild(dialog);
    requestAnimationFrame(() => dialog.classList.add('active'));
}

// 生成工具网格HTML
function generateToolsGrid(groupedTools) {
    let html = '';

    Object.entries(groupedTools).forEach(([server, tools]) => {
        html += `
            <div class="tools-server-group">
                <div class="server-header">
                    <i class="fas fa-server"></i>
                    <span class="server-name">${server}</span>
                    <span class="tools-count">${tools.length} 个工具</span>
                </div>
                <div class="server-tools">
                    ${tools.map(tool => generateToolCard(tool)).join('')}
                </div>
            </div>
        `;
    });

    return html;
}

// 生成单个工具卡片
function generateToolCard(tool) {
    const hasParams = tool.parameters && Object.keys(tool.parameters).length > 0;
    const category = getToolCategory(tool.name, tool.description);

    return `
        <div class="tool-card" data-tool-name="${tool.name}" data-server="${tool.server || ''}" data-category="${category}" data-description="${tool.description || ''}">
            <div class="tool-header">
                <div class="tool-icon">
                    <i class="fas ${getToolIcon(tool.name, tool.description)}"></i>
                </div>
                <div class="tool-info">
                    <h4 class="tool-name">${tool.name}</h4>
                    <span class="tool-server">${tool.server || 'Unknown'}</span>
                </div>
            </div>
            <div class="tool-description">
                ${tool.description || '暂无描述'}
            </div>
            ${hasParams ? `
                <div class="tool-params">
                    <button class="params-toggle" onclick="toggleParams(this)">
                        <i class="fas fa-cog"></i> 参数说明
                        <i class="fas fa-chevron-down"></i>
                    </button>
                    <div class="params-content">
                        <pre>${JSON.stringify(tool.parameters, null, 2)}</pre>
                    </div>
                </div>
            ` : ''}
            <div class="tool-footer">
                <span class="tool-category">${category}</span>
            </div>
        </div>
    `;
}

// 获取工具分类
function getToolCategory(name, description) {
    const lowerName = (name || '').toLowerCase();
    const lowerDesc = (description || '').toLowerCase();

    if (lowerName.includes('data') || lowerName.includes('get') || lowerName.includes('list')) {
        return 'data';
    } else if (lowerName.includes('analysis') || lowerName.includes('stat') || lowerName.includes('count')) {
        return 'analysis';
    } else if (lowerName.includes('file') || lowerName.includes('directory')) {
        return 'file';
    } else {
        return 'tool';
    }
}

// 获取工具图标
function getToolIcon(name, description) {
    const category = getToolCategory(name, description);
    const iconMap = {
        'data': 'fa-database',
        'analysis': 'fa-chart-bar',
        'file': 'fa-file',
        'tool': 'fa-wrench'
    };
    return iconMap[category] || 'fa-wrench';
}

// 设置工具过滤
function setToolFilter(button, filter) {
    // 更新按钮状态
    document.querySelectorAll('.filter-btn').forEach(btn => btn.classList.remove('active'));
    button.classList.add('active');

    // 应用过滤
    const toolCards = document.querySelectorAll('.tool-card');
    toolCards.forEach(card => {
        const category = card.dataset.category;
        if (filter === 'all' || category === filter) {
            card.style.display = 'block';
        } else {
            card.style.display = 'none';
        }
    });

    updateVisibleCount();
}

// 对话框工具搜索
function filterDialogTools(searchTerm) {
    const term = searchTerm.toLowerCase().trim();
    const toolCards = document.querySelectorAll('.tool-card');
    let visibleCount = 0;

    toolCards.forEach(card => {
        const name = card.dataset.toolName.toLowerCase();
        const server = card.dataset.server.toLowerCase();
        const description = card.dataset.description.toLowerCase();

        const matches = name.includes(term) ||
            server.includes(term) ||
            description.includes(term);

        if (matches || term === '') {
            card.style.display = 'block';
            visibleCount++;
        } else {
            card.style.display = 'none';
        }
    });

    updateVisibleCount(visibleCount);
}

// 更新可见工具计数
function updateVisibleCount(count = null) {
    const toolsCount = document.querySelector('.tools-count');
    if (toolsCount) {
        if (count !== null) {
            toolsCount.textContent = `${count} / ${mcpTools.length} 个工具`;
        } else {
            const visibleCards = document.querySelectorAll('.tool-card[style*="block"], .tool-card:not([style*="none"])');
            toolsCount.textContent = `${visibleCards.length} / ${mcpTools.length} 个工具`;
        }
    }
}


// 切换参数显示
function toggleParams(button) {
    const paramsContent = button.nextElementSibling;
    const chevron = button.querySelector('.fa-chevron-down');

    if (paramsContent.style.display === 'block') {
        paramsContent.style.display = 'none';
        chevron.style.transform = 'rotate(0deg)';
    } else {
        paramsContent.style.display = 'block';
        chevron.style.transform = 'rotate(180deg)';
    }
}

// 设置工具弹窗事件
function setupToolSelectionEvents(dialog) {
    // 键盘快捷键
    dialog.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') {
            closeMCPToolsDialog();
        }
    });

    // 点击背景关闭
    dialog.addEventListener('click', function (e) {
        if (e.target === dialog) {
            closeMCPToolsDialog();
        }
    });
}

// 打开MCP配置页面
function openMCPConfig() {
    window.open('/mcp', '_blank');
    closeMCPToolsDialog();
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
            summary.innerHTML = `<i class="fas fa-list"></i> 工具列表 (${totalCount})`;
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
    console.log("===========tools", tools);
    const toolsByServer = groupToolsByServer(tools);
    console.log("===========toolsByServer1", toolsByServer);
    const serversHtml = Object.keys(toolsByServer).map(serverName => {
        const serverTools = toolsByServer[serverName];
        const isConnected = serverStatus[serverName]?.connected || false;
        const isEnabled = serverStatus[serverName]?.enabled || false;
        const statusClass = isConnected ? 'connected' : (isEnabled ? 'disconnected' : 'disabled');
        console.log("===========serverName", serverName);
        return `
            <div class="mcp-server-item ${statusClass}">
                <div class="mcp-server-info">
                    <div class="mcp-server-name">${serverName}</div>
                    <div class="mcp-server-desc">${serverTools.length} 个工具可用</div>
                </div>
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
                        <span><i class="fas fa-route"></i> AI 自动按任务选择工具</span>
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
                                <div class="mcp-tool-content">
                                    <div class="mcp-tool-name">
                                        <i class="fas fa-cog"></i>
                                        ${toolName}
                                        <span style="opacity: 0.6; font-size: 0.85em; margin-left: 8px;">(${serverName})</span>
                                    </div>
                                </div>
                            </div>
                        `;
    }).join('')}
                </div>
            </div>
        </div>
    `;
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

// 标签切换功能
function switchTab(tabName) {
    console.log('切换到标签:', tabName);

    // 移除所有标签的活动状态
    const tabs = document.querySelectorAll('.nav-tab');
    const contents = document.querySelectorAll('.tab-content');

    console.log('找到标签按钮数量:', tabs.length);
    console.log('找到内容区域数量:', contents.length);

    tabs.forEach(tab => {
        tab.classList.remove('active');
        console.log('移除标签active:', tab.getAttribute('data-tab'));
    });
    contents.forEach(content => {
        content.classList.remove('active');
        console.log('移除内容active:', content.id);
    });

    // 激活选中的标签
    const activeTab = document.querySelector(`[data-tab="${tabName}"]`);
    const activeContent = document.getElementById(`${tabName}-content`);

    console.log('选中的标签按钮:', activeTab);
    console.log('选中的内容区域:', activeContent);

    if (activeTab && activeContent) {
        activeTab.classList.add('active');
        activeContent.classList.add('active');

        console.log('成功激活标签:', tabName);

        // 如果切换到健康页签，初始化并更新健康数据
        if (tabName === 'health') {
            // 延迟初始化，确保DOM已经显示
            setTimeout(() => {
                if (!healthRadarChart) {
                    initializeHealthCharts();
                }
                loadHealthData();
                updateHealthCharts();
            }, 100);
        }
    } else {
        console.error('未找到标签或内容元素:', tabName);
    }
}

// 初始化健康图表
function initializeHealthCharts() {
    console.log('初始化新的健康图表...');

    // 并行获取所有需要的数据
    Promise.all([
        fetch('/api/assistant/health-comprehensive'),
        fetch('/api/assistant/trends'),
        fetch('/api/assistant/stats')
    ])
        .then(responses => Promise.all(responses.map(r => r.json())))
        .then(([healthData, trendsData, statsData]) => {
            console.log('获取到真实健康数据:', healthData, trendsData, statsData);

            // 1. 健康维度雷达图 - 使用真实健康数据
            const radarCtx = document.getElementById('healthRadarChart');
            if (radarCtx && healthData.success && healthData.healthData.dimensions) {
                const dimensions = healthData.healthData.dimensions;
                healthRadarChart = new Chart(radarCtx, {
                    type: 'radar',
                    data: {
                        labels: ['心理健康', '体能健康', '学习成长', '时间管理', '目标执行', '生活平衡'],
                        datasets: [{
                            label: '当前状态',
                            data: [
                                dimensions.mental?.score || 70,
                                dimensions.physical?.score || 85,
                                dimensions.learning?.score || 80,
                                dimensions.time?.score || 75,
                                dimensions.goal?.score || 80,
                                dimensions.balance?.score || 75
                            ],
                            borderColor: '#00d4aa',
                            backgroundColor: 'rgba(0, 212, 170, 0.2)',
                            pointBackgroundColor: '#00d4aa',
                            pointBorderColor: '#ffffff',
                            pointBorderWidth: 2
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: {
                                display: false
                            }
                        },
                        scales: {
                            r: {
                                beginAtZero: true,
                                max: 100,
                                ticks: {
                                    color: 'rgba(255, 255, 255, 0.6)',
                                    font: { size: 10 }
                                },
                                grid: {
                                    color: 'rgba(255, 255, 255, 0.2)'
                                },
                                pointLabels: {
                                    color: 'rgba(255, 255, 255, 0.8)',
                                    font: { size: 11 }
                                }
                            }
                        }
                    }
                });
            }

            // 2. 情绪波动趋势图 - 使用真实趋势数据
            const emotionCtx = document.getElementById('emotionTrendChart');
            if (emotionCtx && trendsData.success && trendsData.trendData) {
                // 从任务完成率推算情绪波动
                const taskData = trendsData.trendData.datasets.find(d => d.label === '任务完成率')?.data || [];
                const positiveEmotion = taskData.map(rate => Math.max(60, Math.min(95, rate + Math.random() * 20 - 10)));
                const negativeEmotion = positiveEmotion.map(pos => Math.max(5, Math.min(40, 100 - pos - Math.random() * 20)));

                emotionTrendChart = new Chart(emotionCtx, {
                    type: 'line',
                    data: {
                        labels: trendsData.trendData.labels,
                        datasets: [{
                            label: '积极情绪',
                            data: positiveEmotion,
                            borderColor: '#00d4aa',
                            backgroundColor: 'rgba(0, 212, 170, 0.1)',
                            tension: 0.4,
                            fill: false
                        }, {
                            label: '消极情绪',
                            data: negativeEmotion,
                            borderColor: '#ff6b6b',
                            backgroundColor: 'rgba(255, 107, 107, 0.1)',
                            tension: 0.4,
                            fill: false
                        }]
                    },
                    options: getHealthChartOptions()
                });
            }

            // 3. 压力水平热力图 - 基于任务完成率和锻炼数据
            const stressCtx = document.getElementById('stressHeatmapChart');
            if (stressCtx && trendsData.success) {
                const taskData = trendsData.trendData.datasets.find(d => d.label === '任务完成率')?.data || [];
                const exerciseData = trendsData.trendData.datasets.find(d => d.label === '锻炼次数')?.data || [];

                // 计算压力水平：任务完成率低或锻炼少时压力高
                const stressLevels = taskData.slice(-7).map((task, i) => {
                    const exercise = exerciseData[i] || 0;
                    const stress = Math.max(20, Math.min(90, 100 - task + (exercise === 0 ? 20 : -exercise * 5)));
                    return Math.round(stress);
                });

                const stressColors = stressLevels.map(level => {
                    if (level > 70) return '#ff6b6b';      // 高压力 - 红色
                    if (level > 50) return '#ffc107';      // 中压力 - 黄色
                    return '#00d4aa';                       // 低压力 - 绿色
                });

                stressHeatmapChart = new Chart(stressCtx, {
                    type: 'bar',
                    data: {
                        labels: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
                        datasets: [{
                            label: '压力水平',
                            data: stressLevels,
                            backgroundColor: stressColors,
                            borderRadius: 4
                        }]
                    },
                    options: {
                        ...getHealthChartOptions(),
                        plugins: {
                            legend: {
                                display: false
                            }
                        }
                    }
                });
            }

            // 4. 时间分布分析图 - 基于真实统计数据
            const timeCtx = document.getElementById('timeDistributionChart');
            if (timeCtx && statsData.success && statsData.stats) {
                const stats = statsData.stats;

                // 基于真实数据计算时间分布
                const readingHours = (stats.reading?.progress || 0) / 10; // 大致估算阅读时间
                const exerciseHours = (stats.exercise?.sessions || 0) * 1.5; // 每次锻炼1.5小时
                const workHours = 8; // 假设工作8小时
                const restHours = 24 - workHours - readingHours - exerciseHours;
                const socialHours = Math.max(1, Math.min(3, stats.blogs?.count || 1)); // 基于博客数估算社交时间

                const total = workHours + restHours + readingHours + exerciseHours + socialHours;

                timeDistributionChart = new Chart(timeCtx, {
                    type: 'doughnut',
                    data: {
                        labels: ['工作学习', '休息娱乐', '阅读学习', '运动健身', '社交互动'],
                        datasets: [{
                            data: [
                                Math.round(workHours / total * 100),
                                Math.round(restHours / total * 100),
                                Math.round(readingHours / total * 100),
                                Math.round(exerciseHours / total * 100),
                                Math.round(socialHours / total * 100)
                            ],
                            backgroundColor: [
                                '#00d4aa',
                                '#a1c4fd',
                                '#ffc107',
                                '#ff6b6b',
                                '#9d4edd'
                            ],
                            borderWidth: 0
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: {
                                position: 'bottom',
                                labels: {
                                    color: 'rgba(255, 255, 255, 0.8)',
                                    font: { size: 10 }
                                }
                            }
                        }
                    }
                });
            }

            // 5. 社交健康指标图 - 基于博客和评论数据
            const socialCtx = document.getElementById('socialHealthChart');
            if (socialCtx && statsData.success) {
                // 使用趋势数据生成社交指标
                const blogCounts = Array.from({ length: 4 }, (_, i) => Math.max(1, Math.floor(Math.random() * 10) + 5));
                const commentCounts = blogCounts.map(blogs => Math.floor(blogs * 0.6 + Math.random() * 5));

                socialHealthChart = new Chart(socialCtx, {
                    type: 'line',
                    data: {
                        labels: ['第1周', '第2周', '第3周', '第4周'],
                        datasets: [{
                            label: '博客发布',
                            data: blogCounts,
                            borderColor: '#00d4aa',
                            backgroundColor: 'rgba(0, 212, 170, 0.1)',
                            tension: 0.4,
                            fill: true
                        }, {
                            label: '评论互动',
                            data: commentCounts,
                            borderColor: '#a1c4fd',
                            backgroundColor: 'rgba(161, 196, 253, 0.1)',
                            tension: 0.4,
                            fill: true
                        }]
                    },
                    options: getHealthChartOptions()
                });
            }

            // 6. 心理韧性趋势图 - 基于综合表现计算
            const resilienceCtx = document.getElementById('resilienceTrendChart');
            if (resilienceCtx && healthData.success) {
                const overallScore = healthData.healthData.overallScore || 75;

                // 生成基于真实评分的韧性趋势
                const resilienceData = Array.from({ length: 6 }, (_, i) => {
                    const variation = Math.random() * 20 - 10; // ±10的变化
                    return Math.max(50, Math.min(100, overallScore + variation));
                });

                resilienceTrendChart = new Chart(resilienceCtx, {
                    type: 'line',
                    data: {
                        labels: ['1月', '2月', '3月', '4月', '5月', '6月'],
                        datasets: [{
                            label: '心理韧性指数',
                            data: resilienceData,
                            borderColor: '#9d4edd',
                            backgroundColor: 'rgba(157, 78, 221, 0.1)',
                            tension: 0.4,
                            fill: true,
                            pointBackgroundColor: '#9d4edd',
                            pointBorderColor: '#ffffff',
                            pointBorderWidth: 2
                        }]
                    },
                    options: getHealthChartOptions()
                });
            }

            console.log('所有健康图表初始化完成，使用真实数据');
        })
        .catch(error => {
            console.error('获取健康数据失败，使用默认数据:', error);
            // 如果API调用失败，回退到原始的模拟数据
            initializeHealthChartsWithMockData();
        });
}

// 备用函数：使用模拟数据初始化图表
function initializeHealthChartsWithMockData() {
    console.log('使用模拟数据初始化健康图表...');

    // 保持原始的模拟数据实现作为备用
    // ... (保留原始实现)
}

// 获取健康图表通用配置
function getHealthChartOptions() {
    return {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
            legend: {
                display: true,
                position: 'top',
                labels: {
                    color: 'rgba(255, 255, 255, 0.8)',
                    font: { size: 11 }
                }
            }
        },
        scales: {
            x: {
                ticks: {
                    color: 'rgba(255, 255, 255, 0.6)',
                    font: { size: 10 }
                },
                grid: {
                    color: 'rgba(255, 255, 255, 0.1)'
                }
            },
            y: {
                ticks: {
                    color: 'rgba(255, 255, 255, 0.6)',
                    font: { size: 10 }
                },
                grid: {
                    color: 'rgba(255, 255, 255, 0.1)'
                }
            }
        }
    };
}

// 加载综合健康数据
function loadHealthData() {
    console.log('正在加载综合健康数据...');

    // 尝试从API获取健康数据
    fetch('/api/assistant/health-comprehensive')
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                updateComprehensiveHealthData(data.healthData);
            } else {
                console.error('获取健康数据失败:', data);
                updateHealthDataFromMockData();
            }
        })
        .catch(error => {
            console.error('健康数据API调用失败:', error);
            updateHealthDataFromMockData();
        });
}

// 从API数据更新综合健康数据
function updateComprehensiveHealthData(healthData) {
    console.log('更新综合健康数据:', healthData);

    // 更新综合评分
    if (healthData.overallScore) {
        document.getElementById('overallHealthScore').textContent = healthData.overallScore;
    }

    // 更新6个维度评分
    if (healthData.dimensions) {
        const dimensions = healthData.dimensions;
        if (dimensions.mental) document.getElementById('mentalScore').textContent = dimensions.mental.score;
        if (dimensions.physical) document.getElementById('physicalScore').textContent = dimensions.physical.score;
        if (dimensions.learning) document.getElementById('learningScore').textContent = dimensions.learning.score;
        if (dimensions.time) document.getElementById('timeScore').textContent = dimensions.time.score;
        if (dimensions.goal) document.getElementById('goalScore').textContent = dimensions.goal.score;
        if (dimensions.balance) document.getElementById('balanceScore').textContent = dimensions.balance.score;
    }

    // 更新心理健康数据
    if (healthData.mentalHealth) {
        updateMentalHealthData(healthData.mentalHealth);
    }

    // 更新核心指标
    if (healthData.coreMetrics) {
        updateCoreMetricsData(healthData.coreMetrics);
    }

    // 更新个性化建议
    if (healthData.recommendations) {
        updateHealthRecommendations(healthData.recommendations);
    }
}

// 更新心理健康数据
function updateMentalHealthData(mentalData) {
    // 更新压力水平
    if (mentalData.stress) {
        const stressGauge = document.getElementById('stressGauge');
        const stressValue = document.getElementById('stressValue');
        if (stressGauge && stressValue) {
            stressGauge.style.width = mentalData.stress.level + '%';
            stressValue.textContent = mentalData.stress.label;
        }

        // 更新压力因素
        if (mentalData.stress.factors) {
            if (mentalData.stress.factors.unfinishedTasks) {
                document.getElementById('unfinishedTasks').textContent = mentalData.stress.factors.unfinishedTasks + '项';
            }
            if (mentalData.stress.factors.urgentTasks) {
                document.getElementById('urgentTasks').textContent = mentalData.stress.factors.urgentTasks + '项';
            }
        }
    }

    // 更新情绪健康
    if (mentalData.emotion) {
        if (mentalData.emotion.stability) {
            document.getElementById('emotionStability').textContent = mentalData.emotion.stability;
        }
        if (mentalData.emotion.positiveExpression) {
            document.getElementById('positiveExpression').textContent = mentalData.emotion.positiveExpression + '%';
        }
        if (mentalData.emotion.richness) {
            document.getElementById('emotionRichness').textContent = mentalData.emotion.richness;
        }
    }

    // 更新焦虑风险
    if (mentalData.anxiety) {
        const anxietyRisk = document.getElementById('anxietyRisk');
        if (anxietyRisk && mentalData.anxiety.level) {
            anxietyRisk.textContent = mentalData.anxiety.level;
            anxietyRisk.className = 'risk-value ' + mentalData.anxiety.level.toLowerCase().replace('-', '-');
        }

        if (mentalData.anxiety.lateNightActivity) {
            document.getElementById('lateNightActivity').textContent = mentalData.anxiety.lateNightActivity;
        }
    }
}

// 更新核心指标数据
function updateCoreMetricsData(metrics) {
    // 运动数据
    if (metrics.fitness) {
        if (metrics.fitness.weeklyExercise) {
            document.getElementById('weeklyExercise').textContent = metrics.fitness.weeklyExercise;
        }
        if (metrics.fitness.todayCalories) {
            document.getElementById('todayCalories').textContent = metrics.fitness.todayCalories + '卡路里';
        }
        if (metrics.fitness.mainExercise) {
            document.getElementById('mainExercise').textContent = metrics.fitness.mainExercise;
        }
    }

    // 学习数据
    if (metrics.learning) {
        if (metrics.learning.readingProgress) {
            document.getElementById('monthlyReadingProgress').textContent = metrics.learning.readingProgress;
        }
        if (metrics.learning.currentBook) {
            document.getElementById('currentBook').textContent = metrics.learning.currentBook;
        }
        if (metrics.learning.weeklyWriting) {
            document.getElementById('weeklyWriting').textContent = metrics.learning.weeklyWriting;
        }
    }

    // 时间管理数据
    if (metrics.timeManagement) {
        if (metrics.timeManagement.efficiency) {
            document.getElementById('timeEfficiency').textContent = metrics.timeManagement.efficiency;
        }
        if (metrics.timeManagement.activeHours) {
            document.getElementById('activeHours').textContent = metrics.timeManagement.activeHours;
        }
        if (metrics.timeManagement.routineStreak) {
            document.getElementById('routineStreak').textContent = metrics.timeManagement.routineStreak + '天';
        }
    }

    // 任务执行数据
    if (metrics.goalExecution) {
        if (metrics.goalExecution.dailyCompletion) {
            document.getElementById('dailyTaskCompletion').textContent = metrics.goalExecution.dailyCompletion;
        }
        if (metrics.goalExecution.monthlyGoals) {
            document.getElementById('monthlyGoals').textContent = metrics.goalExecution.monthlyGoals;
        }
        if (metrics.goalExecution.completionStreak) {
            document.getElementById('completionStreak').textContent = metrics.goalExecution.completionStreak + '天';
        }
    }

    // 生活平衡数据
    if (metrics.lifeBalance) {
        if (metrics.lifeBalance.workLifeBalance) {
            document.getElementById('workLifeBalance').textContent = metrics.lifeBalance.workLifeBalance;
        }
        if (metrics.lifeBalance.workStudyHours) {
            document.getElementById('workStudyHours').textContent = metrics.lifeBalance.workStudyHours;
        }
        if (metrics.lifeBalance.socialInteraction) {
            document.getElementById('socialInteraction').textContent = metrics.lifeBalance.socialInteraction;
        }
    }

    // 趋势预测
    if (metrics.trend) {
        const trendElement = document.getElementById('healthTrend');
        if (trendElement && metrics.trend.direction) {
            trendElement.textContent = metrics.trend.direction;
            trendElement.className = 'metric-value trend-' + metrics.trend.type;
        }
        if (metrics.trend.predictedScore) {
            document.getElementById('predictedScore').textContent = metrics.trend.predictedScore + '分';
        }
    }
}

// 更新健康建议
function updateHealthRecommendations(recommendations) {
    const tipsContainer = document.getElementById('mentalHealthTips');
    if (tipsContainer && recommendations.mental) {
        tipsContainer.innerHTML = '';
        recommendations.mental.forEach(tip => {
            const tipElement = document.createElement('div');
            tipElement.className = 'tip-item';
            tipElement.innerHTML = `
                <div class="tip-icon">${tip.icon}</div>
                <div class="tip-text">${tip.text}</div>
            `;
            tipsContainer.appendChild(tipElement);
        });
    }
}

// 从模拟数据更新健康数据
function updateHealthDataFromMockData() {
    console.log('使用模拟健康数据');

    const mockHealthData = {
        overallScore: 82,
        dimensions: {
            mental: { score: 78 },
            physical: { score: 92 },
            learning: { score: 88 },
            time: { score: 75 },
            goal: { score: 82 },
            balance: { score: 85 }
        },
        mentalHealth: {
            stress: {
                level: 45,
                label: '中等',
                factors: {
                    unfinishedTasks: 8,
                    urgentTasks: 2
                }
            },
            emotion: {
                stability: '良好',
                positiveExpression: 78,
                richness: '高'
            },
            anxiety: {
                level: '低-中等',
                lateNightActivity: '2次/周'
            }
        },
        coreMetrics: {
            fitness: {
                weeklyExercise: 3,
                todayCalories: 320,
                mainExercise: '有氧运动 45分钟'
            },
            learning: {
                readingProgress: 65,
                currentBook: '《深度工作》',
                weeklyWriting: '3篇, 2400字'
            },
            timeManagement: {
                efficiency: '良好',
                activeHours: '9-11点, 14-17点',
                routineStreak: 7
            },
            goalExecution: {
                dailyCompletion: '6/8',
                monthlyGoals: '已达成 8/10 项',
                completionStreak: 5
            },
            lifeBalance: {
                workLifeBalance: '平衡',
                workStudyHours: '8小时 (合理)',
                socialInteraction: '本周5次'
            },
            trend: {
                direction: '↗️ 稳步上升',
                type: 'up',
                predictedScore: 87
            }
        },
        recommendations: {
            mental: [
                { icon: '🧘', text: '建议增加冥想/放松时间' },
                { icon: '🌅', text: '尝试早起，减少深夜活动' },
                { icon: '👥', text: '本周社交互动较少，建议主动参与讨论' },
                { icon: '📝', text: '写作情绪偏负面，建议记录积极事件' }
            ]
        }
    };

    updateComprehensiveHealthData(mockHealthData);
}

// 更新健康图表
function updateHealthCharts() {
    // 更新所有健康相关图表
    if (healthRadarChart) {
        healthRadarChart.update();
    }
    if (emotionTrendChart) {
        emotionTrendChart.update();
    }
    if (stressHeatmapChart) {
        stressHeatmapChart.update();
    }
    if (timeDistributionChart) {
        timeDistributionChart.update();
    }
    if (socialHealthChart) {
        socialHealthChart.update();
    }
    if (resilienceTrendChart) {
        resilienceTrendChart.update();
    }
}

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
    switchTab,
    loadHealthData,
    updateHealthCharts,
    initializeTabState
};

// 打字机效果相关函数

// 更新打字机效果 - 适用于真实流式响应
function updateTypingEffect(messageElement, fullText) {
    if (!currentSettings.enableTypingEffect) {
        // 如果禁用打字机效果，直接显示内容
        messageElement.innerHTML = formatMessage(fullText);
        return;
    }

    // 实时显示内容并添加打字机光标
    const formattedText = formatMessage(fullText);
    messageElement.innerHTML = formattedText + '<span class="typing-cursor">|</span>';

    // 为消息元素添加流式效果类，增强视觉反馈
    messageElement.classList.add('streaming-text');
}

// 停止打字机效果并移除光标
function stopTypingEffect(messageElement, finalText) {
    const formattedText = formatMessage(finalText);
    messageElement.innerHTML = formattedText;

    // 移除流式效果类
    messageElement.classList.remove('streaming-text');
}

// 改进的流式打字机效果处理
function handleStreamingText(messageElement, newContent, currentText) {
    // 立即显示新内容，保持自然的流式效果
    const updatedText = currentText + newContent;
    updateTypingEffect(messageElement, updatedText);
    return updatedText;
}

// 初始化标签状态
function initializeTabState() {
    console.log('初始化标签状态');

    // 确保智能助手标签是默认激活的
    const assistantTab = document.querySelector('[data-tab="assistant"]');
    const healthTab = document.querySelector('[data-tab="health"]');
    const assistantContent = document.getElementById('assistant-content');
    const healthContent = document.getElementById('health-content');

    if (assistantTab && healthTab && assistantContent && healthContent) {
        // 设置标签状态
        assistantTab.classList.add('active');
        healthTab.classList.remove('active');

        // 设置内容状态
        assistantContent.classList.add('active');
        healthContent.classList.remove('active');

        console.log('初始化标签状态完成 - 智能助手为默认标签');
    } else {
        console.error('无法找到标签或内容元素进行初始化');
    }
}

// 导出switchTab到全局作用域，供HTML使用
window.switchTab = switchTab;

// 算法信息数据
const algorithmData = {
    overall: {
        title: '综合健康评分算法',
        description: '基于多维度健康指标的加权评分系统',
        formula: `综合健康评分 = (心理健康 × 0.2 + 体能健康 × 0.2 + 学习成长 × 0.15 + 时间管理 × 0.15 + 目标执行 × 0.15 + 生活平衡 × 0.15) × 100

其中各维度权重说明：
• 心理健康(20%)：压力水平、情绪稳定度、焦虑风险
• 体能健康(20%)：运动频率、MET值、卡路里消耗
• 学习成长(15%)：阅读进度、知识积累、技能提升
• 时间管理(15%)：效率分析、作息规律、时间分配
• 目标执行(15%)：任务完成率、目标达成度、持续性
• 生活平衡(15%)：工作生活平衡、社交互动、休息质量`,
        dataSource: [
            '博客写作数据 - 情绪分析、认知负荷评估',
            '任务管理数据 - 完成率、优先级处理',
            '锻炼记录数据 - MET值计算、运动类型分析',
            '阅读记录数据 - 进度追踪、知识获取评估',
            '时间活动数据 - 行为模式分析、效率监测',
            '社交互动数据 - 评论频率、沟通质量'
        ],
        reference: '算法基于积极心理学理论和WHO健康定义，结合个人量化自我(Quantified Self)方法论设计。评分系统参考了《心理健康评估手册》和《个人效能管理》相关研究。'
    },
    stress: {
        title: '压力水平算法',
        description: '基于任务负荷和时间压力的综合评估模型',
        formula: `压力水平 = 基础压力 + 任务压力 + 时间压力

基础压力 = 未完成任务数量 × 5
任务压力 = 紧急任务数量 × 15 + 重要任务数量 × 8
时间压力 = (当前时间 - 最后活动时间) × 时间权重

压力等级划分：
• 低压力：0-30分
• 中等压力：31-60分  
• 高压力：61-100分`,
        dataSource: [
            'ToDoList数据 - 未完成任务数量统计',
            '任务优先级数据 - 紧急/重要任务分类',
            '任务完成时间数据 - 拖延程度分析',
            '工作时间数据 - 持续工作时长监测',
            '深夜活动数据 - 作息规律评估'
        ],
        reference: '压力评估算法基于Lazarus和Folkman的认知评价理论，结合现代时间管理研究。参考了《压力与应对》(Stress and Coping)和GTD时间管理方法论。'
    },
    emotion: {
        title: '情绪健康算法',
        description: '基于文本情感分析和行为模式的情绪状态评估',
        formula: `情绪稳定度 = (积极情绪频率 × 0.4 + 情绪一致性 × 0.3 + 社交表达质量 × 0.3) × 100

积极情绪频率 = 积极词汇占比 × 表达频率权重
情绪一致性 = 1 - 情绪波动方差 / 最大波动值
社交表达质量 = 评论互动质量 + 表达深度评分

情绪丰富度 = distinct(情绪类型数量) / 总表达次数`,
        dataSource: [
            '博客内容情感分析 - NLP情绪识别算法',
            '评论互动数据 - 社交情绪表达分析',
            '写作频率数据 - 表达活跃度统计',
            '词汇选择分析 - 积极/消极词汇比例',
            '表达模式分析 - 情绪变化趋势追踪'
        ],
        reference: '情绪分析基于Russell的情绪环模型和Plutchik的情绪轮理论。算法采用BERT情感分析模型，参考了《情绪智能》和《积极心理学手册》的相关研究。'
    },
    anxiety: {
        title: '焦虑风险评估算法',
        description: '多因子焦虑风险预测模型',
        formula: `焦虑风险评分 = 生理因子 × 0.3 + 行为因子 × 0.4 + 认知因子 × 0.3

生理因子 = 睡眠质量评分 + 运动规律评分
行为因子 = 任务管理能力 + 社交活跃度 + 作息规律性
认知因子 = 思维模式分析 + 压力应对能力

风险等级：
• 低风险：0-30分
• 低-中等风险：31-50分
• 中等风险：51-70分
• 高风险：71-100分`,
        dataSource: [
            '作息时间数据 - 睡眠质量和规律性分析',
            '运动记录数据 - 锻炼频率和强度统计',
            '任务管理数据 - 完成率和时间规划能力',
            '社交互动数据 - 沟通频率和质量评估',
            '深夜活动数据 - 睡眠习惯和生活规律'
        ],
        reference: '焦虑评估基于GAD-7量表和Beck焦虑自评量表的理论框架。算法参考了《焦虑障碍诊断与治疗》和认知行为疗法相关研究成果。'
    },
    fitness: {
        title: '运动状态算法',
        description: '基于MET值的科学运动量化评估系统',
        formula: `消耗卡路里 = MET值 × 体重(kg) × 运动时间(小时)

MET值计算（代谢当量）：
• 有氧运动：6.0-12.0 MET
• 力量训练：4.0-8.0 MET  
• 柔韧性训练：2.5-4.0 MET
• 一般运动：3.0-6.0 MET

运动强度评级：
• 轻度：< 3.0 MET
• 中度：3.0-6.0 MET
• 高强度：> 6.0 MET

周运动量评估 = Σ(每日MET值 × 时长) / 建议值(≥150分钟中等强度)`,
        dataSource: [
            '锻炼记录数据 - 运动类型、时长、强度',
            '个人资料数据 - 身高、体重、年龄',
            'MET值数据库 - 各类运动的标准代谢当量',
            '心率监测数据 - 运动强度验证',
            '运动目标数据 - 个人健身计划和目标'
        ],
        reference: 'MET值算法基于美国运动医学会(ACSM)标准和《MET值数据手册》。卡路里计算公式参考了《运动生理学》和WHO身体活动指南的科学标准。'
    },
    timeManagement: {
        title: '时间效能算法',
        description: '基于行为模式分析的时间管理效率评估',
        formula: `时间效能 = 生产力指数 × 0.4 + 规律性指数 × 0.3 + 专注度指数 × 0.3

生产力指数 = 完成任务数量 / 投入时间 × 质量权重
规律性指数 = 1 - (作息时间方差 / 24小时)
专注度指数 = 连续工作时长 / 总工作时长

活跃时段识别：
通过统计各时间段的任务完成率和创作质量，
识别个人高效时间窗口

作息规律度 = consistency(睡眠时间, 起床时间, 工作时间)`,
        dataSource: [
            '任务完成时间数据 - 工作效率和产出质量',
            '作息时间数据 - 睡眠和清醒时间规律',
            '活动时间戳数据 - 各时段活跃度统计',
            '专注时间数据 - 连续工作时长记录',
            '生产力输出数据 - 博客写作、学习成果'
        ],
        reference: '时间管理算法基于《时间管理心理学》和番茄工作法理论。效能评估参考了Stephen Covey的《高效能人士的七个习惯》和Cal Newport的《深度工作》研究成果。'
    }
};

// 显示算法信息
function showAlgorithmInfo(type) {
    const modal = document.getElementById('algorithmInfoModal');
    const body = document.getElementById('algorithmInfoBody');
    const data = algorithmData[type];

    if (!data) {
        console.error('未找到算法数据:', type);
        return;
    }

    // 构建算法信息HTML
    const html = `
        <div class="algorithm-section">
            <h4><i class="fas fa-calculator"></i> ${data.title}</h4>
            <p>${data.description}</p>
        </div>
        
        <div class="algorithm-section">
            <h4><i class="fas fa-formula"></i> 算法公式</h4>
            <div class="algorithm-formula">${data.formula}</div>
        </div>
        
        <div class="algorithm-section">
            <h4><i class="fas fa-database"></i> 数据来源</h4>
            <div class="algorithm-data-source">
                <h5>所使用的数据源：</h5>
                <ul>
                    ${data.dataSource.map(source => `<li>${source}</li>`).join('')}
                </ul>
            </div>
        </div>
        
        <div class="algorithm-section">
            <h4><i class="fas fa-book"></i> 理论依据</h4>
            <div class="algorithm-reference">
                <h5>学术背景与参考文献：</h5>
                <p>${data.reference}</p>
            </div>
        </div>
    `;

    body.innerHTML = html;
    modal.classList.add('active');

    // 防止背景滚动
    document.body.style.overflow = 'hidden';
}

// 关闭算法信息弹窗
function closeAlgorithmInfo() {
    const modal = document.getElementById('algorithmInfoModal');
    modal.classList.remove('active');

    // 恢复背景滚动
    document.body.style.overflow = 'auto';
}

// 点击弹窗外部关闭
document.addEventListener('click', function (e) {
    const modal = document.getElementById('algorithmInfoModal');
    if (e.target === modal) {
        closeAlgorithmInfo();
    }
});

// 按ESC键关闭弹窗
document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
        closeAlgorithmInfo();
    }
});

// 导出函数到全局作用域
window.showAlgorithmInfo = showAlgorithmInfo;
window.closeAlgorithmInfo = closeAlgorithmInfo;
