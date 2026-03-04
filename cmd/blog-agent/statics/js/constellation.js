// 星座占卜JavaScript交互文件

// 全局变量
let currentConstellation = '';
let currentSection = 'horoscope';
let selectedMethod = 'single_card';

// 星座数据
const constellationData = {
    'aries': { name: '白羊座', symbol: '♈', dateRange: '3.21-4.19' },
    'taurus': { name: '金牛座', symbol: '♉', dateRange: '4.20-5.20' },
    'gemini': { name: '双子座', symbol: '♊', dateRange: '5.21-6.21' },
    'cancer': { name: '巨蟹座', symbol: '♋', dateRange: '6.22-7.22' },
    'leo': { name: '狮子座', symbol: '♌', dateRange: '7.23-8.22' },
    'virgo': { name: '处女座', symbol: '♍', dateRange: '8.23-9.22' },
    'libra': { name: '天秤座', symbol: '♎', dateRange: '9.23-10.23' },
    'scorpio': { name: '天蝎座', symbol: '♏', dateRange: '10.24-11.22' },
    'sagittarius': { name: '射手座', symbol: '♐', dateRange: '11.23-12.21' },
    'capricorn': { name: '摩羯座', symbol: '♑', dateRange: '12.22-1.19' },
    'aquarius': { name: '水瓶座', symbol: '♒', dateRange: '1.20-2.18' },
    'pisces': { name: '双鱼座', symbol: '♓', dateRange: '2.19-3.20' }
};

// DOM加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeEventListeners();
    showBackToTopButton();
    createStarField();
});

// 初始化事件监听器
function initializeEventListeners() {
    // 星座选择事件
    document.querySelectorAll('.constellation-item').forEach(item => {
        item.addEventListener('click', function() {
            selectConstellation(this.dataset.sign);
        });
    });

    // 功能导航事件
    document.querySelectorAll('.nav-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            switchSection(this.dataset.section);
        });
    });

    // 占卜类型选择事件
    const divinationType = document.getElementById('divination-type');
    if (divinationType) {
        divinationType.addEventListener('change', function() {
            toggleMethodSelection(this.value);
        });
    }

    // 占卜方法选择事件
    document.querySelectorAll('.method-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            selectDivinationMethod(this.dataset.method);
        });
    });

    // 各种按钮事件
    bindButtonEvents();

    // 返回顶部按钮事件
    const backToTopBtn = document.getElementById('back-to-top');
    if (backToTopBtn) {
        backToTopBtn.addEventListener('click', scrollToTop);
    }
}

// 绑定按钮事件
function bindButtonEvents() {
    const buttons = {
        'start-divination': startDivination,
        'analyze-compatibility': analyzeCompatibility,
        'create-birthchart': createBirthChart
    };

    Object.entries(buttons).forEach(([id, handler]) => {
        const element = document.getElementById(id);
        if (element) {
            element.addEventListener('click', handler);
        }
    });
}

// 选择星座
function selectConstellation(sign) {
    // 移除之前的选中状态
    document.querySelectorAll('.constellation-item').forEach(item => {
        item.classList.remove('active');
    });

    // 添加选中状态
    const selectedItem = document.querySelector(`[data-sign="${sign}"]`);
    if (selectedItem) {
        selectedItem.classList.add('active');
    }

    currentConstellation = sign;
    
    // 加载每日运势
    loadDailyHoroscope(sign);
    
    // 显示运势区域
    const horoscopeSection = document.getElementById('daily-horoscope');
    if (horoscopeSection) {
        horoscopeSection.style.display = 'block';
        horoscopeSection.scrollIntoView({ behavior: 'smooth' });
    }
}

// 加载每日运势
async function loadDailyHoroscope(constellation) {
    if (!constellation) return;

    try {
        showLoading(true);
        
        const today = new Date().toISOString().split('T')[0];
        const response = await fetch(`/api/constellation/horoscope?constellation=${constellation}&date=${today}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const horoscope = await response.json();
        displayHoroscope(horoscope);
        
    } catch (error) {
        console.error('加载运势失败:', error);
        showMessage('加载运势失败，请稍后重试', 'error');
    } finally {
        showLoading(false);
    }
}

// 显示运势信息
function displayHoroscope(horoscope) {
    const constellationInfo = constellationData[horoscope.constellation];
    
    // 更新星座信息
    document.querySelector('.selected-symbol').textContent = constellationInfo.symbol;
    document.querySelector('.selected-name').textContent = constellationInfo.name;
    document.querySelector('.horoscope-date').textContent = formatDate(horoscope.date);
    
    // 更新运势评分
    updateStars('overall-stars', horoscope.overall);
    updateStars('love-stars', horoscope.love);
    updateStars('career-stars', horoscope.career);
    updateStars('money-stars', horoscope.money);
    updateStars('health-stars', horoscope.health);
    
    // 更新总分显示
    const overallScore = document.getElementById('overall-score');
    if (overallScore) {
        overallScore.textContent = getScoreText(horoscope.overall);
    }
    
    // 更新幸运元素
    document.getElementById('lucky-color').textContent = horoscope.lucky_color;
    document.getElementById('lucky-number').textContent = horoscope.lucky_number;
    
    // 更新运势描述和建议
    document.getElementById('horoscope-text').textContent = horoscope.description;
    document.getElementById('advice-text').textContent = horoscope.advice;
}

// 更新星星评分显示
function updateStars(elementId, rating) {
    const container = document.getElementById(elementId);
    if (!container) return;
    
    container.innerHTML = '';
    
    for (let i = 1; i <= 5; i++) {
        const star = document.createElement('span');
        star.className = i <= rating ? 'star filled' : 'star';
        star.textContent = '★';
        container.appendChild(star);
    }
}

// 获取评分文本
function getScoreText(score) {
    const scoreTexts = {
        1: '需要小心',
        2: '平平淡淡',
        3: '中规中矩',
        4: '运势不错',
        5: '运势爆棚'
    };
    return scoreTexts[score] || '未知';
}

// 切换功能区域
function switchSection(section) {
    // 更新导航按钮状态
    document.querySelectorAll('.nav-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    document.querySelector(`[data-section="${section}"]`).classList.add('active');
    
    // 隐藏所有区域
    const sections = ['horoscope-section', 'divination-section', 'compatibility-section', 'birthchart-section', 'history-section'];
    sections.forEach(sectionId => {
        const element = document.getElementById(sectionId);
        if (element) {
            element.style.display = 'none';
        }
    });
    
    // 显示当前区域
    const targetSection = document.getElementById(`${section}-section`);
    if (targetSection) {
        targetSection.style.display = 'block';
        targetSection.scrollIntoView({ behavior: 'smooth' });
    }
    
    // 显示运势区域（如果选择了星座且在运势页面）
    if (section === 'horoscope' && currentConstellation) {
        const horoscopeSection = document.getElementById('daily-horoscope');
        if (horoscopeSection) {
            horoscopeSection.style.display = 'block';
        }
    }
    
    // 加载区域特定数据
    loadSectionData(section);
    
    currentSection = section;
}

// 加载区域特定数据
function loadSectionData(section) {
    switch (section) {
        case 'history':
            loadDivinationHistory();
            loadDivinationStats();
            break;
        // 其他区域可以根据需要添加
    }
}

// 切换占卜方法选择
function toggleMethodSelection(type) {
    const methodsContainer = document.getElementById('tarot-methods');
    if (methodsContainer) {
        methodsContainer.style.display = type === 'tarot' ? 'block' : 'none';
    }
}

// 选择占卜方法
function selectDivinationMethod(method) {
    document.querySelectorAll('.method-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    document.querySelector(`[data-method="${method}"]`).classList.add('active');
    selectedMethod = method;
}

// 开始占卜
async function startDivination() {
    const type = document.getElementById('divination-type').value;
    const question = document.getElementById('divination-question').value.trim();
    
    if (!question) {
        showMessage('请输入你的问题', 'warning');
        return;
    }
    
    try {
        showLoading(true);
        
        const requestData = {
            user_name: getCurrentUser(),
            type: type,
            question: question,
            method: type === 'tarot' ? selectedMethod : type
        };
        
        const response = await fetch('/api/constellation/divination', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestData)
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const result = await response.json();
        displayDivinationResult(result);
        
    } catch (error) {
        console.error('占卜失败:', error);
        showMessage('占卜失败，请稍后重试', 'error');
    } finally {
        showLoading(false);
    }
}

// 显示占卜结果
function displayDivinationResult(result) {
    const resultContainer = document.getElementById('divination-result');
    if (!resultContainer) return;
    
    // 显示塔罗牌（如果有）
    const cardsContainer = document.getElementById('tarot-cards');
    if (cardsContainer && result.result.cards && result.result.cards.length > 0) {
        cardsContainer.innerHTML = '';
        result.result.cards.forEach(card => {
            const cardElement = createTarotCardElement(card);
            cardsContainer.appendChild(cardElement);
        });
    } else if (cardsContainer) {
        cardsContainer.style.display = 'none';
    }
    
    // 显示解读结果
    const interpretationElement = document.getElementById('divination-interpretation');
    if (interpretationElement) {
        interpretationElement.innerHTML = `
            <h4>占卜解读</h4>
            <div class="interpretation-content">${formatText(result.result.interpretation)}</div>
        `;
    }
    
    // 显示建议
    const adviceElement = document.getElementById('divination-advice');
    if (adviceElement) {
        adviceElement.innerHTML = `
            <h4>指导建议</h4>
            <div class="advice-content">${result.result.advice}</div>
            <div class="lucky-elements-divination">
                <p><strong>幸运数字:</strong> ${result.result.lucky_numbers.join(', ')}</p>
                <p><strong>幸运颜色:</strong> ${result.result.lucky_colors.join(', ')}</p>
                <p><strong>准确度预测:</strong> ${Math.round(result.result.probability * 100)}%</p>
            </div>
        `;
    }
    
    resultContainer.style.display = 'block';
    resultContainer.scrollIntoView({ behavior: 'smooth' });
}

// 创建塔罗牌元素
function createTarotCardElement(card) {
    const cardDiv = document.createElement('div');
    cardDiv.className = `tarot-card ${card.is_reversed ? 'reversed' : ''}`;
    
    cardDiv.innerHTML = `
        <div class="card-name">${card.name}</div>
        <div class="card-name-cn">${card.name_cn}</div>
    `;
    
    return cardDiv;
}

// 分析星座配对
async function analyzeCompatibility() {
    const sign1 = document.getElementById('compatibility-sign1').value;
    const sign2 = document.getElementById('compatibility-sign2').value;
    
    if (!sign1 || !sign2) {
        showMessage('请选择两个星座', 'warning');
        return;
    }
    
    if (sign1 === sign2) {
        showMessage('请选择不同的星座进行配对', 'warning');
        return;
    }
    
    try {
        showLoading(true);
        
        const response = await fetch(`/api/constellation/compatibility?sign1=${sign1}&sign2=${sign2}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const analysis = await response.json();
        displayCompatibilityResult(analysis);
        
    } catch (error) {
        console.error('配对分析失败:', error);
        showMessage('配对分析失败，请稍后重试', 'error');
    } finally {
        showLoading(false);
    }
}

// 显示配对分析结果
function displayCompatibilityResult(analysis) {
    // 更新综合匹配度圆形进度条
    updateCircleProgress('overall-compatibility', analysis.overall_score);
    document.getElementById('overall-score-value').textContent = Math.round(analysis.overall_score);
    
    // 更新详细分数进度条
    updateProgressBar('love-progress', 'love-score-text', analysis.love_score);
    updateProgressBar('friend-progress', 'friend-score-text', analysis.friend_score);
    updateProgressBar('work-progress', 'work-score-text', analysis.work_score);
    
    // 更新分析内容
    updateAnalysisSection('compatibility-advantages', analysis.advantages);
    updateAnalysisSection('compatibility-challenges', analysis.challenges);
    updateAnalysisSection('compatibility-suggestions', analysis.suggestions);
    
    // 显示详细分析
    const detailedAnalysis = document.getElementById('detailed-analysis');
    if (detailedAnalysis) {
        detailedAnalysis.innerHTML = `
            <h4>详细分析</h4>
            <div class="analysis-text">${formatText(analysis.analysis)}</div>
        `;
    }
    
    // 显示结果区域
    document.getElementById('compatibility-result').style.display = 'block';
    document.getElementById('compatibility-result').scrollIntoView({ behavior: 'smooth' });
}

// 更新圆形进度条
function updateCircleProgress(elementId, score) {
    const element = document.getElementById(elementId);
    if (!element) return;
    
    const percentage = Math.round(score);
    const degrees = (percentage / 100) * 360;
    
    element.style.background = `conic-gradient(var(--accent-color) ${degrees}deg, rgba(255,255,255,0.1) ${degrees}deg)`;
}

// 更新进度条
function updateProgressBar(progressId, textId, score) {
    const progressBar = document.getElementById(progressId);
    const progressText = document.getElementById(textId);
    
    if (progressBar) {
        progressBar.style.width = `${score}%`;
    }
    
    if (progressText) {
        progressText.textContent = `${Math.round(score)}%`;
    }
}

// 更新分析章节
function updateAnalysisSection(elementId, items) {
    const element = document.getElementById(elementId);
    if (!element || !items) return;
    
    element.innerHTML = '';
    items.forEach(item => {
        const li = document.createElement('li');
        li.textContent = item;
        element.appendChild(li);
    });
}

// 创建个人星盘
async function createBirthChart() {
    const username = document.getElementById('chart-username').value.trim();
    const birthdate = document.getElementById('chart-birthdate').value;
    const birthtime = document.getElementById('chart-birthtime').value;
    const birthplace = document.getElementById('chart-birthplace').value.trim();
    
    if (!username || !birthdate || !birthtime || !birthplace) {
        showMessage('请填写完整的出生信息', 'warning');
        return;
    }
    
    try {
        showLoading(true);
        
        const requestData = {
            user_name: username,
            birth_date: birthdate,
            birth_time: birthtime,
            birth_place: birthplace
        };
        
        const response = await fetch('/api/constellation/birthchart', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestData)
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const chart = await response.json();
        displayBirthChart(chart);
        
    } catch (error) {
        console.error('生成星盘失败:', error);
        showMessage('生成星盘失败，请稍后重试', 'error');
    } finally {
        showLoading(false);
    }
}

// 显示个人星盘
function displayBirthChart(chart) {
    // 显示主要星座
    document.getElementById('sun-sign').textContent = getConstellationName(chart.sun_sign);
    document.getElementById('moon-sign').textContent = getConstellationName(chart.moon_sign);
    document.getElementById('rising-sign').textContent = getConstellationName(chart.rising_sign);
    
    // 显示行星位置
    const planetaryContainer = document.getElementById('planetary-positions');
    if (planetaryContainer) {
        const planetsHtml = `
            <div class="planet-item">
                <span class="planet-name">☀️ 太阳</span>
                <span class="planet-position">${getConstellationName(chart.planetary.sun)}</span>
            </div>
            <div class="planet-item">
                <span class="planet-name">🌙 月亮</span>
                <span class="planet-position">${getConstellationName(chart.planetary.moon)}</span>
            </div>
            <div class="planet-item">
                <span class="planet-name">☿ 水星</span>
                <span class="planet-position">${getConstellationName(chart.planetary.mercury)}</span>
            </div>
            <div class="planet-item">
                <span class="planet-name">♀ 金星</span>
                <span class="planet-position">${getConstellationName(chart.planetary.venus)}</span>
            </div>
            <div class="planet-item">
                <span class="planet-name">♂ 火星</span>
                <span class="planet-position">${getConstellationName(chart.planetary.mars)}</span>
            </div>
        `;
        planetaryContainer.innerHTML = `<h4>行星位置</h4>${planetsHtml}`;
    }
    
    // 显示宫位信息
    const housesContainer = document.getElementById('houses-info');
    if (housesContainer && chart.houses) {
        let housesHtml = '<h4>十二宫位</h4>';
        chart.houses.forEach(house => {
            housesHtml += `
                <div class="house-item">
                    <span class="house-number">第${house.number}宫</span>
                    <span class="house-sign">${getConstellationName(house.sign)}</span>
                </div>
            `;
        });
        housesContainer.innerHTML = housesHtml;
    }
    
    // 显示结果区域
    document.getElementById('birthchart-result').style.display = 'block';
    document.getElementById('birthchart-result').scrollIntoView({ behavior: 'smooth' });
}

// 加载占卜历史
async function loadDivinationHistory() {
    try {
        const username = getCurrentUser();
        if (!username) return;
        
        const response = await fetch(`/api/constellation/history?user_name=${username}&limit=10`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const history = await response.json();
        displayDivinationHistory(history);
        
    } catch (error) {
        console.error('加载历史记录失败:', error);
    }
}

// 显示占卜历史
function displayDivinationHistory(history) {
    const historyContainer = document.getElementById('history-list');
    if (!historyContainer || !history || history.length === 0) {
        if (historyContainer) {
            historyContainer.innerHTML = '<p>暂无占卜记录</p>';
        }
        return;
    }
    
    let historyHtml = '';
    history.forEach(record => {
        historyHtml += `
            <div class="history-item">
                <div class="history-header">
                    <span class="history-type">${getTypeText(record.type)}</span>
                    <span class="history-date">${formatDateTime(record.create_time)}</span>
                </div>
                <div class="history-question">"${record.question}"</div>
                <div class="accuracy-rating">
                    <span>准确度评价:</span>
                    ${createStarsRating(record.accuracy)}
                </div>
            </div>
        `;
    });
    
    historyContainer.innerHTML = historyHtml;
}

// 加载占卜统计
async function loadDivinationStats() {
    try {
        const username = getCurrentUser();
        if (!username) return;
        
        const response = await fetch(`/api/constellation/statistics?user_name=${username}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const stats = await response.json();
        displayDivinationStats(stats);
        
    } catch (error) {
        console.error('加载统计数据失败:', error);
    }
}

// 显示占卜统计
function displayDivinationStats(stats) {
    document.getElementById('total-divinations').textContent = stats.total_count || 0;
    document.getElementById('average-accuracy').textContent = stats.accuracy_avg ? 
        `${stats.accuracy_avg.toFixed(1)}/5` : '0/5';
    document.getElementById('favorite-type').textContent = getTypeText(stats.favorite_type) || '-';
}

// 工具函数

// 获取星座中文名称
function getConstellationName(sign) {
    return constellationData[sign] ? constellationData[sign].name : sign;
}

// 获取占卜类型文本
function getTypeText(type) {
    const typeTexts = {
        'tarot': '塔罗占卜',
        'astrology': '星座占卜',
        'numerology': '数字占卜'
    };
    return typeTexts[type] || type;
}

// 格式化日期
function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: 'long',
        day: 'numeric'
    });
}

// 格式化日期时间
function formatDateTime(dateTimeString) {
    const date = new Date(dateTimeString);
    return date.toLocaleString('zh-CN');
}

// 格式化文本（处理换行）
function formatText(text) {
    return text.replace(/\n/g, '<br>');
}

// 创建星星评分
function createStarsRating(rating) {
    let stars = '';
    for (let i = 1; i <= 5; i++) {
        stars += i <= rating ? '★' : '☆';
    }
    return stars;
}

// 获取当前用户（简化版本，实际应该从session获取）
function getCurrentUser() {
    // 这里应该从实际的用户session获取用户名
    // 暂时返回一个默认值
    return 'user';
}

// 显示加载动画
function showLoading(show) {
    const loadingOverlay = document.getElementById('loading-overlay');
    if (loadingOverlay) {
        loadingOverlay.style.display = show ? 'flex' : 'none';
    }
}

// 显示消息提示
function showMessage(message, type = 'info') {
    // 创建消息元素
    const messageDiv = document.createElement('div');
    messageDiv.className = `message-toast message-${type}`;
    messageDiv.textContent = message;
    
    // 添加样式
    messageDiv.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 15px 20px;
        background: var(--card-bg);
        backdrop-filter: blur(10px);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        color: var(--text-color);
        z-index: 10000;
        max-width: 300px;
        transform: translateX(100%);
        transition: transform 0.3s ease;
    `;
    
    // 添加类型特定样式
    if (type === 'error') {
        messageDiv.style.borderColor = '#ff4757';
        messageDiv.style.background = 'rgba(255, 71, 87, 0.1)';
    } else if (type === 'warning') {
        messageDiv.style.borderColor = '#ffa502';
        messageDiv.style.background = 'rgba(255, 165, 2, 0.1)';
    } else if (type === 'success') {
        messageDiv.style.borderColor = '#2ed573';
        messageDiv.style.background = 'rgba(46, 213, 115, 0.1)';
    }
    
    document.body.appendChild(messageDiv);
    
    // 显示动画
    setTimeout(() => {
        messageDiv.style.transform = 'translateX(0)';
    }, 100);
    
    // 自动隐藏
    setTimeout(() => {
        messageDiv.style.transform = 'translateX(100%)';
        setTimeout(() => {
            document.body.removeChild(messageDiv);
        }, 300);
    }, 3000);
}

// 滚动到顶部
function scrollToTop() {
    window.scrollTo({
        top: 0,
        behavior: 'smooth'
    });
}

// 显示/隐藏返回顶部按钮
function showBackToTopButton() {
    const backToTopBtn = document.getElementById('back-to-top');
    if (!backToTopBtn) return;
    
    window.addEventListener('scroll', function() {
        if (window.pageYOffset > 300) {
            backToTopBtn.style.display = 'block';
        } else {
            backToTopBtn.style.display = 'none';
        }
    });
}

// 创建星星背景动画
function createStarField() {
    const starBackground = document.querySelector('.star-background');
    if (!starBackground) return;
    
    // 创建更多星星
    for (let i = 0; i < 50; i++) {
        const star = document.createElement('div');
        star.className = 'floating-star';
        star.style.cssText = `
            position: absolute;
            width: 2px;
            height: 2px;
            background: white;
            border-radius: 50%;
            top: ${Math.random() * 100}vh;
            left: ${Math.random() * 100}vw;
            animation: twinkle ${2 + Math.random() * 3}s infinite;
            opacity: ${0.3 + Math.random() * 0.7};
        `;
        starBackground.appendChild(star);
    }
}

// 添加闪烁动画CSS
const style = document.createElement('style');
style.textContent = `
    @keyframes twinkle {
        0%, 100% { opacity: 0.3; transform: scale(1); }
        50% { opacity: 1; transform: scale(1.2); }
    }
`;
document.head.appendChild(style);

// 页面可见性变化处理
document.addEventListener('visibilitychange', function() {
    if (document.visibilityState === 'visible' && currentConstellation) {
        // 页面重新可见时刷新运势
        loadDailyHoroscope(currentConstellation);
    }
});

// 错误处理
window.addEventListener('error', function(e) {
    console.error('JavaScript错误:', e.error);
    showMessage('页面出现错误，请刷新重试', 'error');
});

// 网络状态检测
window.addEventListener('online', function() {
    showMessage('网络连接已恢复', 'success');
});

window.addEventListener('offline', function() {
    showMessage('网络连接已断开', 'warning');
});