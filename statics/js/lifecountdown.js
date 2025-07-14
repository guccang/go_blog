// 人生倒计时页面JavaScript - 优化版

// 全局变量
let currentConfig = {
    currentAge: 25,
    expectedLifespan: 80,
    dailySleepHours: 8.0,
    dailyStudyHours: 2.0,
    dailyReadingHours: 1.0,
    dailyWorkHours: 8.0,
    averageBookWords: 150000
};

let currentData = null;
let timeChart = null;
let booksChart = null;
let availableBooks = []; // 存储从API获取的书籍列表

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 先加载保存的配置
    loadSavedConfig().then(() => {
        initializePage();
        setupEventListeners();
        
        // 异步加载书籍列表，加载完成后会自动更新可视化
        loadBooksList().then(() => {
            console.log('书籍列表加载完成，当前书籍数量:', availableBooks.length);
        });
        
        loadData();
    });
});

// 初始化页面
function initializePage() {
    updateSliderValues();
    initializeCharts();
    loadData();
}

// 初始化图表
function initializeCharts() {
    // 初始化时间分配图表
    const timeCtx = document.getElementById('timeChart').getContext('2d');
    timeChart = new Chart(timeCtx, {
        type: 'doughnut',
        data: {
            labels: ['睡眠 (33.3%)', '休息 (50%)', '学习 (8%)', '黄金时间 (33.7%)'],
            datasets: [{
                data: [9740, 14651, 2323, 9862],
                backgroundColor: [
                    'rgba(161, 196, 253, 0.8)',
                    'rgba(122, 231, 191, 0.8)',
                    'rgba(253, 203, 110, 0.8)',
                    'rgba(255, 107, 107, 0.8)'
                ],
                borderColor: [
                    'rgba(161, 196, 253, 1)',
                    'rgba(122, 231, 191, 1)',
                    'rgba(253, 203, 110, 1)',
                    'rgba(255, 107, 107, 1)'
                ],
                borderWidth: 1
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
                        font: {
                            size: 13
                        },
                        padding: 20
                    }
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            const label = context.label || '';
                            const value = context.parsed || 0;
                            return `${label}: ${value.toLocaleString()} 天`;
                        }
                    }
                }
            }
        }
    });
    
    // 初始化阅读图表
    const booksCtx = document.getElementById('booksChart').getContext('2d');
    booksChart = new Chart(booksCtx, {
        type: 'bar',
        data: {
            labels: ['快速 (周/本)', '中等 (月/2本)', '慢速 (3月/本)'],
            datasets: [{
                label: '一生可读书籍数量',
                data: [3640, 1680, 280],
                backgroundColor: 'rgba(255, 154, 158, 0.7)',
                borderColor: 'rgba(255, 154, 158, 1)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.8)'
                    },
                    grid: {
                        color: 'rgba(255, 255, 255, 0.1)'
                    }
                },
                x: {
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.8)'
                    },
                    grid: {
                        display: false
                    }
                }
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `约 ${context.parsed.y} 本书`;
                        }
                    }
                }
            }
        }
    });
}

// 设置事件监听器
function setupEventListeners() {
    // 滑块事件
    document.getElementById('age-slider').addEventListener('input', handleAgeChange);
    document.getElementById('lifespan-slider').addEventListener('input', handleLifespanChange);
    document.getElementById('sleep-slider').addEventListener('input', handleSleepChange);
    document.getElementById('reading-slider').addEventListener('input', handleReadingChange);
    
    // 按钮事件
    document.getElementById('btn-fast').addEventListener('click', function() {
        // 快速阅读场景：每周一本，70年 * 52周 = 3640本
        const fastReadingBooks = 3640;
        updateBooksVisualization(fastReadingBooks);
        updateBooksChart([fastReadingBooks, 1680, 280]);
    });
    
    document.getElementById('btn-slow').addEventListener('click', function() {
        // 慢速阅读场景：每3个月一本，70年 * 4本/年 = 280本
        const slowReadingBooks = 280;
        updateBooksVisualization(slowReadingBooks);
        updateBooksChart([280, 1680, slowReadingBooks]);
    });
}

// 加载数据
function loadData() {
    loadMockData();
}

// 加载模拟数据
function loadMockData() {
    const totalDays = currentConfig.expectedLifespan * 365;
    const passedDays = currentConfig.currentAge * 365;
    const remainingDays = totalDays - passedDays;
    const sleepDays = Math.floor(totalDays * currentConfig.dailySleepHours / 24);
    const studyDays = Math.floor(totalDays * currentConfig.dailyStudyHours / 24);
    const workDays = Math.floor(totalDays * currentConfig.dailyWorkHours / 24);
    const restDays = Math.floor(totalDays * (24 - currentConfig.dailySleepHours - currentConfig.dailyStudyHours - currentConfig.dailyWorkHours) / 24);
    const goldenDays = Math.max(0, Math.min(45, currentConfig.expectedLifespan) - Math.max(18, currentConfig.currentAge)) * 365;
    const totalReadingHours = Math.floor(remainingDays * currentConfig.dailyReadingHours);
    // 使用固定的阅读速度：300字/分钟，平均每本书15万字
    const fixedReadingSpeed = 300; // 字/分钟
    const booksCanRead = Math.floor(totalReadingHours * 60 / (currentConfig.averageBookWords / fixedReadingSpeed));
    
    currentData = {
        currentAge: currentConfig.currentAge,
        expectedLifespan: currentConfig.expectedLifespan,
        totalDays: totalDays,
        passedDays: passedDays,
        remainingDays: remainingDays,
        passedPercent: (passedDays / totalDays) * 100,
        remainingPercent: (remainingDays / totalDays) * 100,
        sleepDays: sleepDays,
        studyDays: studyDays,
        workDays: workDays,
        restDays: restDays,
        goldenDays: goldenDays,
        totalReadingHours: totalReadingHours,
        booksCanRead: booksCanRead,
        readingSpeed: fixedReadingSpeed,
        averageBookWords: currentConfig.averageBookWords,
        dailySleepHours: currentConfig.dailySleepHours,
        dailyStudyHours: currentConfig.dailyStudyHours,
        dailyReadingHours: currentConfig.dailyReadingHours,
        dailyWorkHours: currentConfig.dailyWorkHours
    };
    
    updateUI();
}

// 更新UI
function updateUI() {
    if (!currentData) return;
    
    updateBasicInfo();
    updateTimeChart();
    updateBooksChart([currentData.booksCanRead, Math.floor(currentData.booksCanRead * 0.6), Math.floor(currentData.booksCanRead * 0.15)]);
    updateBooksVisualization(currentData.booksCanRead);
    updateGoldenTime();
    updateDataTable();
    updateFooter();
}

// 更新基础信息
function updateBasicInfo() {
    document.getElementById('total-days').textContent = currentData.totalDays.toLocaleString();
    document.getElementById('days-lived').textContent = currentData.passedDays.toLocaleString();
    document.getElementById('days-left').textContent = currentData.remainingDays.toLocaleString();
    document.getElementById('age-info').textContent = `（${currentData.currentAge}岁）`;
    document.getElementById('years-left').textContent = `（${currentData.expectedLifespan - currentData.currentAge}年）`;
}

// 更新时间图表
function updateTimeChart() {
    if (!timeChart) return;
    
    const sleepPercent = (currentData.sleepDays / currentData.totalDays) * 100;
    const restPercent = (currentData.restDays / currentData.totalDays) * 100;
    const studyPercent = (currentData.studyDays / currentData.totalDays) * 100;
    const goldenPercent = (currentData.goldenDays / currentData.totalDays) * 100;
    
    timeChart.data.labels = [
        `睡眠 (${sleepPercent.toFixed(1)}%)`,
        `休息 (${restPercent.toFixed(1)}%)`,
        `学习 (${studyPercent.toFixed(1)}%)`,
        `黄金时间 (${goldenPercent.toFixed(1)}%)`
    ];
    timeChart.data.datasets[0].data = [
        currentData.sleepDays,
        currentData.restDays,
        currentData.studyDays,
        currentData.goldenDays
    ];
    timeChart.update();
}

// 更新书籍图表
function updateBooksChart(data) {
    if (!booksChart) return;
    
    booksChart.data.datasets[0].data = data;
    booksChart.update();
}

// 从API获取书籍列表
function loadBooksList() {
    return fetch('/api/lifecountdown')
        .then(response => response.json())
        .then(data => {
            if (data.success && data.books && data.books.length > 0) {
                availableBooks = data.books;
                console.log('成功加载书籍列表:', availableBooks.length, '本书');
            } else {
                console.log('API返回空书籍列表，使用默认书籍列表');
                availableBooks = getDefaultBooks();
            }
            // 书籍列表加载完成后，重新更新可视化
            if (currentData) {
                updateBooksVisualization(currentData.booksCanRead);
            }
        })
        .catch(error => {
            console.error('获取书籍列表失败:', error);
            availableBooks = getDefaultBooks();
            // 即使失败也要更新可视化
            if (currentData) {
                updateBooksVisualization(currentData.booksCanRead);
            }
        });
}

// 获取默认书籍列表
function getDefaultBooks() {
    return [
        "时间简史", "活着", "百年孤独", "思考快与慢", "人类简史", 
        "原则", "三体", "1984", "深度工作", "认知觉醒", "心流", 
        "经济学原理", "创新者", "未来简史", "影响力", "黑天鹅",
        "毛泽东传", "邓小平传", "红楼梦", "西游记", "水浒传",
        "三国演义", "论语", "孟子", "老子", "庄子", "史记",
        "资治通鉴", "明朝那些事儿", "万历十五年", "中国哲学简史"
    ];
}

// 更新书籍可视化
function updateBooksVisualization(bookCount) {
    const booksContainer = document.getElementById('books-container');
    booksContainer.innerHTML = '';
    
    // 使用从API获取的书籍列表，如果没有则使用默认列表
    const bookTitles = availableBooks.length > 0 ? availableBooks : getDefaultBooks();
    
    // 显示逻辑：
    // 1. 如果使用真实书籍数据，最多显示实际拥有的书籍数量
    // 2. 如果使用默认列表，考虑性能限制最多显示200本
    let maxDisplayCount;
    if (availableBooks.length > 0) {
        // 使用真实书籍数据：不超过实际拥有的书籍数量
        maxDisplayCount = availableBooks.length;
    } else {
        // 使用默认列表：考虑性能，最多显示200本
        maxDisplayCount = 200;
    }
    
    const displayCount = Math.min(bookCount, maxDisplayCount);
    
    for (let i = 0; i < displayCount; i++) {
        const book = document.createElement('div');
        book.className = 'book';
        
        const title = document.createElement('div');
        title.className = 'book-title';
        // 使用真实的书籍标题，按顺序循环显示
        const bookTitle = bookTitles[i % bookTitles.length];
        title.textContent = bookTitle;
        
        // 添加点击事件，跳转到书籍详情页面
        book.addEventListener('click', function() {
            // 跳转到reading页面，并传递书名参数
            window.location.href = `/reading?book=${encodeURIComponent(bookTitle)}`;
        });
        
        // 添加鼠标悬停效果提示
        book.title = `点击查看《${bookTitle}》详情`;
        book.style.cursor = 'pointer';
        
        book.appendChild(title);
        booksContainer.appendChild(book);
    }
    
    // 添加统计文本
    const counter = document.createElement('div');
    counter.style.width = '100%';
    counter.style.textAlign = 'center';
    counter.style.marginTop = '15px';
    counter.style.fontWeight = 'bold';
    
    // 显示更详细的信息
    let limitInfo = '';
    if (availableBooks.length > 0 && bookCount > availableBooks.length) {
        // 真实书籍数据，但可读数量超过拥有数量
        limitInfo = ` (实际拥有${availableBooks.length}本，无法显示更多)`;
    } else if (availableBooks.length === 0 && bookCount > 200) {
        // 默认列表，受性能限制
        limitInfo = ` (受性能限制，最多显示200本)`;
    }
    
    counter.innerHTML = `已显示: ${displayCount} 本书 | 一生可读: ${bookCount} 本书${limitInfo}<br/>
        <small style="color: rgba(255,255,255,0.7)">书籍数据来源: ${availableBooks.length > 0 ? 'Reading模块 (' + availableBooks.length + '本)' : '默认列表'}</small>`;
    booksContainer.appendChild(counter);
}

// 更新黄金时间
function updateGoldenTime() {
    const goldenYears = Math.max(0, Math.min(45, currentData.expectedLifespan) - Math.max(18, currentData.currentAge));
    document.getElementById('golden-years').textContent = goldenYears;
    document.getElementById('golden-days').textContent = currentData.goldenDays.toLocaleString();
}

// 更新详细数据表格
function updateDataTable() {
    const tableBody = document.getElementById('time-table-body');
    tableBody.innerHTML = '';
    
    const timeData = [
        { name: '总寿命', days: currentData.totalDays, percent: 100, desc: '预期寿命总天数' },
        { name: '已过时间', days: currentData.passedDays, percent: currentData.passedPercent, desc: '已经度过的时间' },
        { name: '剩余时间', days: currentData.remainingDays, percent: currentData.remainingPercent, desc: '还有多少时间' },
        { name: '睡眠时间', days: currentData.sleepDays, percent: (currentData.sleepDays / currentData.totalDays) * 100, desc: '一生中用于睡眠的时间' },
        { name: '学习时间', days: currentData.studyDays, percent: (currentData.studyDays / currentData.totalDays) * 100, desc: '用于学习和教育的时间' },
        { name: '工作时间', days: currentData.workDays, percent: (currentData.workDays / currentData.totalDays) * 100, desc: '用于工作的时间' },
        { name: '休息时间', days: currentData.restDays, percent: (currentData.restDays / currentData.totalDays) * 100, desc: '用于休息和娱乐的时间' },
        { name: '黄金时间', days: currentData.goldenDays, percent: (currentData.goldenDays / currentData.totalDays) * 100, desc: '18-45岁的黄金时期' }
    ];
    
    timeData.forEach(item => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${item.name}</td>
            <td>${item.days.toLocaleString()}天</td>
            <td>${item.percent.toFixed(1)}%</td>
            <td>${item.desc}</td>
        `;
        tableBody.appendChild(row);
    });
}

// 更新页脚
function updateFooter() {
    document.getElementById('footer-lifespan').textContent = currentData.expectedLifespan;
    document.getElementById('footer-total-days').textContent = currentData.totalDays.toLocaleString();
    document.getElementById('footer-sleep').textContent = currentData.dailySleepHours;
}

// 滑块事件处理函数
function handleAgeChange(event) {
    const value = parseInt(event.target.value);
    document.getElementById('age-value').textContent = `${value}岁`;
    currentConfig.currentAge = value;
    loadData();
    saveConfig();
}

function handleLifespanChange(event) {
    const value = parseInt(event.target.value);
    document.getElementById('lifespan-value').textContent = `${value}岁`;
    currentConfig.expectedLifespan = value;
    loadData();
    saveConfig();
}

function handleSleepChange(event) {
    const value = parseFloat(event.target.value);
    document.getElementById('sleep-value').textContent = `${value}小时`;
    currentConfig.dailySleepHours = value;
    loadData();
    saveConfig();
}

function handleReadingChange(event) {
    const value = parseFloat(event.target.value);
    document.getElementById('reading-value').textContent = `${value}小时`;
    currentConfig.dailyReadingHours = value;
    loadData();
    saveConfig();
}



// 更新滑块值显示
function updateSliderValues() {
    document.getElementById('age-value').textContent = `${currentConfig.currentAge}岁`;
    document.getElementById('lifespan-value').textContent = `${currentConfig.expectedLifespan}岁`;
    document.getElementById('sleep-value').textContent = `${currentConfig.dailySleepHours}小时`;
    document.getElementById('reading-value').textContent = `${currentConfig.dailyReadingHours}小时`;
    
    // 更新滑块位置
    document.getElementById('age-slider').value = currentConfig.currentAge;
    document.getElementById('lifespan-slider').value = currentConfig.expectedLifespan;
    document.getElementById('sleep-slider').value = currentConfig.dailySleepHours;
    document.getElementById('reading-slider').value = currentConfig.dailyReadingHours;
}

// 保存配置到服务器
function saveConfig() {
    const configData = {
        title: 'lifecountdown.md',
        content: JSON.stringify(currentConfig, null, 2),
        authtype: 'private',
        tags: 'lifecountdown,config',
        encrypt: ''
    };
    
    if (configExists) {
        // 文件存在，使用更新接口
        updateExistingConfig(configData);
    } else {
        // 文件不存在，使用新建接口
        createNewConfig(configData);
    }
}

// 更新已存在的配置文件
function updateExistingConfig(configData) {
    fetch('/modify', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: new URLSearchParams(configData)
    })
    .then(response => response.text())
    .then(data => {
        if (data.includes('successfully')) {
            console.log('✅ 配置更新成功');
            showSaveNotification('设置已保存', 'success');
        } else if (data.includes('not found') || data.includes('不存在')) {
            // 如果博客不存在，改为新建
            console.log('🔄 配置文件不存在，改为新建');
            configExists = false;
            createNewConfig(configData);
        } else {
            console.log('⚠️ 配置更新失败:', data);
            showSaveNotification('更新失败', 'warning');
        }
    })
    .catch(error => {
        console.error('❌ 配置更新失败:', error);
        showSaveNotification('更新失败', 'error');
    });
}

// 创建新的配置文件
function createNewConfig(configData) {
    fetch('/save', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: new URLSearchParams(configData)
    })
    .then(response => response.text())
    .then(data => {
        if (data.includes('successfully')) {
            console.log('✅ 配置新建成功');
            configExists = true; // 标记文件已存在
            showSaveNotification('设置已保存', 'success');
        } else if (data.includes('same title')) {
            // 如果提示已有相同标题，说明文件已存在，改为更新
            console.log('🔄 配置文件已存在，改为更新');
            configExists = true;
            updateExistingConfig(configData);
        } else {
            console.log('⚠️ 配置新建失败:', data);
            showSaveNotification('新建失败', 'warning');
        }
    })
    .catch(error => {
        console.error('❌ 配置新建失败:', error);
        showSaveNotification('新建失败', 'error');
    });
}

// 显示保存通知
function showSaveNotification(message, type = 'success') {
    // 移除已存在的通知
    const existingNotification = document.querySelector('.save-notification');
    if (existingNotification) {
        existingNotification.remove();
    }
    
    // 创建通知元素
    const notification = document.createElement('div');
    notification.className = `save-notification ${type}`;
    notification.textContent = message;
    
    // 添加样式
    notification.style.cssText = `
        position: fixed;
        top: 80px;
        right: 20px;
        padding: 12px 20px;
        border-radius: 8px;
        color: white;
        font-weight: 600;
        z-index: 1001;
        transform: translateX(100%);
        transition: transform 0.3s ease;
        ${type === 'success' ? 'background: linear-gradient(45deg, #4CAF50, #45a049);' : ''}
        ${type === 'warning' ? 'background: linear-gradient(45deg, #ff9800, #f57c00);' : ''}
        ${type === 'error' ? 'background: linear-gradient(45deg, #f44336, #d32f2f);' : ''}
        box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    `;
    
    document.body.appendChild(notification);
    
    // 显示动画
    setTimeout(() => {
        notification.style.transform = 'translateX(0)';
    }, 100);
    
    // 3秒后自动隐藏
    setTimeout(() => {
        notification.style.transform = 'translateX(100%)';
        setTimeout(() => {
            if (notification.parentNode) {
                notification.remove();
            }
        }, 300);
    }, 3000);
}

// 全局变量记录配置文件是否存在
let configExists = false;

// 从服务器加载保存的配置
function loadSavedConfig() {
    return fetch('/api/lifecountdown/config')
        .then(response => response.json())
        .then(data => {
            if (data.success && data.config) {
                // 合并保存的配置到当前配置
                Object.assign(currentConfig, data.config);
                configExists = !data.isDefault; // 如果不是默认配置，说明文件存在
                console.log('成功加载配置:', currentConfig, data.isDefault ? '(默认配置)' : '(保存的配置)');
            } else {
                console.log('加载配置失败，使用默认配置');
                configExists = false;
            }
        })
        .catch(error => {
            console.error('加载配置失败:', error);
            console.log('使用默认配置');
            configExists = false;
        });
} 