// English Learning Tracker - Main JavaScript
(function () {
    'use strict';

    // ==================== State Management ====================
    const STORAGE_KEY = 'english_learning_tracker';
    const DEFAULT_STATE = {
        streak: { current: 0, best: 0, lastDate: null },
        vocabulary: { total: 0, daily: [], words: [] },
        xp: { total: 0, daily: [] },
        listening: { rate: 0, history: [] },
        dailyProgress: {
            vocab: 0, listen: 0, speak: 0, read: 0, grammar: 0
        },
        dailyGoals: {
            vocab: 20, listen: 30, speak: 15, read: 30, grammar: 20
        },
        reminders: [],
        history: [],
        currentPhase: 1,
        phaseProgress: [0, 0, 0, 0],
        resources: [],
        settings: { dailyGoalMinutes: 30, targetLevel: 'B2' }
    };

    let state = loadState();
    let xpChart = null;
    let vocabChart = null;
    let listeningChart = null;

    function loadState() {
        try {
            const saved = localStorage.getItem(STORAGE_KEY);
            if (saved) {
                return { ...DEFAULT_STATE, ...JSON.parse(saved) };
            }
        } catch (e) {
            console.error('Failed to load state:', e);
        }
        return { ...DEFAULT_STATE };
    }

    function saveState() {
        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
        } catch (e) {
            console.error('Failed to save state:', e);
        }
    }

    // ==================== Initialization ====================
    document.addEventListener('DOMContentLoaded', function () {
        initApp();
    });

    function initApp() {
        checkStreak();
        renderAll();
        initCharts();
        showView('dashboard');

        // Check and load from blog API
        loadFromAPI();
    }

    function renderAll() {
        renderGoalBanner();
        renderRoadmap();
        renderStreak();
        renderDailyProgress();
        renderDashboardStats();
        renderReminders();
        renderResources();
        renderHistory();
        renderHeatmap();
    }

    // ==================== View Switching ====================
    function showView(viewName) {
        document.querySelectorAll('.content-view').forEach(function (v) {
            v.classList.remove('active');
        });
        var el = document.getElementById(viewName + 'View');
        if (el) el.classList.add('active');

        document.querySelectorAll('.nav-btn').forEach(function (btn) {
            btn.classList.remove('active');
        });
        var activeBtn = document.querySelector('.nav-btn[data-view="' + viewName + '"]');
        if (activeBtn) activeBtn.classList.add('active');
    }
    window.showView = showView;

    // ==================== Goal Banner ====================
    function renderGoalBanner() {
        var el = document.getElementById('overallProgressFill');
        if (!el) return;

        var totalProgress = calculateOverallProgress();
        el.style.width = totalProgress + '%';

        var daysEl = document.getElementById('goalStatDays');
        var vocabEl = document.getElementById('goalStatVocab');
        var xpEl = document.getElementById('goalStatXP');

        if (daysEl) daysEl.textContent = state.streak.current;
        if (vocabEl) vocabEl.textContent = state.vocabulary.total;
        if (xpEl) xpEl.textContent = state.xp.total;
    }

    function calculateOverallProgress() {
        var phaseWeight = 25;
        var total = 0;
        for (var i = 0; i < state.phaseProgress.length; i++) {
            total += (state.phaseProgress[i] / 100) * phaseWeight;
        }
        return Math.min(100, Math.round(total));
    }

    // ==================== Roadmap ====================
    function renderRoadmap() {
        var phases = [
            { name: '基础入门', desc: '字母、发音、基础词汇 500词', phase: 'phase1' },
            { name: '初级进阶', desc: '日常对话、语法基础、1500词', phase: 'phase2' },
            { name: '中级突破', desc: '阅读理解、听力训练、3000词', phase: 'phase3' },
            { name: '高级精通', desc: '流利表达、写作能力、5000+词', phase: 'phase4' }
        ];

        var list = document.getElementById('roadmapList');
        if (!list) return;
        list.innerHTML = '';

        phases.forEach(function (phase, index) {
            var progress = state.phaseProgress[index] || 0;
            var isCompleted = progress >= 100;
            var isCurrent = index + 1 === state.currentPhase;

            var li = document.createElement('li');
            li.className = 'roadmap-item';
            li.onclick = function () { openPhaseDetail(index); };

            var dotClasses = 'roadmap-dot ' + phase.phase;
            if (isCompleted) dotClasses += ' completed';
            if (isCurrent) dotClasses += ' active';

            var colors = ['var(--phase1-color)', 'var(--phase2-color)', 'var(--phase3-color)', 'var(--phase4-color)'];
            var fillColor = isCompleted ? 'var(--success-color)' : colors[index];

            li.innerHTML =
                '<div class="' + dotClasses + '">' + (isCompleted ? '<i class="fas fa-check"></i>' : (index + 1)) + '</div>' +
                '<div class="roadmap-content">' +
                    '<h4>' + (isCurrent ? '>>> ' : '') + '阶段' + (index + 1) + ': ' + phase.name + '</h4>' +
                    '<p>' + phase.desc + '</p>' +
                    '<div class="phase-progress"><div class="phase-progress-fill" style="width:' + progress + '%;background:' + fillColor + '"></div></div>' +
                    '<p style="font-size:11px;margin-top:4px;opacity:0.6">' + progress + '% 完成</p>' +
                '</div>';
            list.appendChild(li);
        });
    }

    function openPhaseDetail(phaseIndex) {
        var names = ['基础入门', '初级进阶', '中级突破', '高级精通'];
        var tasks = [
            ['学习26个字母及发音', '掌握音标系统', '学习基础问候语', '掌握数字和日期表达', '学习500个核心词汇'],
            ['学习基础语法结构', '掌握日常对话场景', '学习常用句型50个', '扩展词汇到1500', '通过A2水平测试'],
            ['阅读简短文章', '听力理解练习', '学习复合句和从句', '扩展词汇到3000', '通过B1水平测试'],
            ['阅读英文原著', '流利日常对话', '英文写作练习', '扩展词汇到5000+', '通过B2水平测试']
        ];

        var title = document.getElementById('modalTitle');
        var body = document.getElementById('modalBody');
        if (!title || !body) return;

        title.textContent = '阶段' + (phaseIndex + 1) + ': ' + names[phaseIndex];
        var html = '<div style="margin-bottom:16px">' +
            '<div class="phase-progress" style="height:10px;background:var(--lighter-bg);border-radius:5px;overflow:hidden">' +
            '<div class="phase-progress-fill" style="width:' + state.phaseProgress[phaseIndex] + '%;height:100%;background:var(--accent-color);border-radius:5px"></div>' +
            '</div>' +
            '<p style="font-size:12px;margin-top:4px;opacity:0.6">' + state.phaseProgress[phaseIndex] + '% 完成</p></div>';

        html += '<h4 style="margin-bottom:12px">学习任务：</h4>';
        tasks[phaseIndex].forEach(function (task, i) {
            var checked = (state.phaseProgress[phaseIndex] / 100) * tasks[phaseIndex].length > i;
            html += '<label style="display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:1px solid var(--border-color);cursor:pointer">' +
                '<input type="checkbox" ' + (checked ? 'checked' : '') +
                ' onchange="window.togglePhaseTask(' + phaseIndex + ',' + i + ',' + tasks[phaseIndex].length + ')">' +
                '<span>' + task + '</span></label>';
        });

        openModal();
    }

    window.togglePhaseTask = function (phaseIndex, taskIndex, totalTasks) {
        var checkboxes = document.querySelectorAll('#modalBody input[type="checkbox"]');
        var completed = 0;
        checkboxes.forEach(function (cb) { if (cb.checked) completed++; });
        state.phaseProgress[phaseIndex] = Math.round((completed / totalTasks) * 100);

        if (state.phaseProgress[phaseIndex] >= 100 && state.currentPhase === phaseIndex + 1) {
            if (phaseIndex < 3) state.currentPhase = phaseIndex + 2;
        }

        saveState();
        renderRoadmap();
        renderGoalBanner();
    };

    // ==================== Streak Counter ====================
    function checkStreak() {
        var today = getDateStr(new Date());
        if (state.streak.lastDate === today) return;

        var yesterday = getDateStr(new Date(Date.now() - 86400000));
        if (state.streak.lastDate === yesterday) {
            // streak continues, will be incremented when user logs activity
        } else if (state.streak.lastDate && state.streak.lastDate !== today) {
            // streak broken
            state.streak.current = 0;
        }
        saveState();
    }

    function incrementStreak() {
        var today = getDateStr(new Date());
        if (state.streak.lastDate !== today) {
            state.streak.current++;
            state.streak.lastDate = today;
            if (state.streak.current > state.streak.best) {
                state.streak.best = state.streak.current;
            }
            saveState();
            renderStreak();
        }
    }

    function renderStreak() {
        var numEl = document.getElementById('streakNumber');
        var bestEl = document.getElementById('streakBest');
        var flameEl = document.getElementById('streakFlames');

        if (numEl) numEl.textContent = state.streak.current;
        if (bestEl) bestEl.textContent = '最佳记录: ' + state.streak.best + ' 天';

        if (flameEl) {
            var flames = '';
            var count = Math.min(state.streak.current, 7);
            for (var i = 0; i < count; i++) flames += '🔥';
            if (count === 0) flames = '💤';
            flameEl.textContent = flames;
        }
    }

    // ==================== Daily Progress ====================
    function renderDailyProgress() {
        var items = [
            { key: 'vocab', icon: '📝', label: '词汇学习', color: 'var(--phase1-color)', unit: '词' },
            { key: 'listen', icon: '🎧', label: '听力训练', color: 'var(--phase4-color)', unit: '分钟' },
            { key: 'speak', icon: '🗣️', label: '口语练习', color: 'var(--phase2-color)', unit: '分钟' },
            { key: 'read', icon: '📖', label: '阅读理解', color: 'var(--phase3-color)', unit: '分钟' },
            { key: 'grammar', icon: '✏️', label: '语法练习', color: '#6c757d', unit: '题' }
        ];

        var container = document.getElementById('dailyProgressList');
        if (!container) return;
        container.innerHTML = '';

        items.forEach(function (item) {
            var current = state.dailyProgress[item.key] || 0;
            var goal = state.dailyGoals[item.key] || 20;
            var pct = Math.min(100, Math.round((current / goal) * 100));

            var div = document.createElement('div');
            div.className = 'progress-item';
            div.innerHTML =
                '<div class="progress-item-icon ' + item.key + '">' + item.icon + '</div>' +
                '<div class="progress-item-info">' +
                    '<div class="item-title">' + item.label + '</div>' +
                    '<div class="item-bar"><div class="item-bar-fill" style="width:' + pct + '%;background:' + item.color + '"></div></div>' +
                '</div>' +
                '<div class="progress-item-value">' + current + '/' + goal + ' ' + item.unit + '</div>';
            container.appendChild(div);
        });
    }

    // ==================== Dashboard Stats ====================
    function renderDashboardStats() {
        updateStatCard('statVocabTotal', state.vocabulary.total);
        updateStatCard('statXPTotal', state.xp.total);
        updateStatCard('statListeningRate', state.listening.rate + '%');
        updateStatCard('statStreakDays', state.streak.current);
    }

    function updateStatCard(id, value) {
        var el = document.getElementById(id);
        if (el) el.textContent = value;
    }

    // ==================== Charts ====================
    function initCharts() {
        if (typeof Chart === 'undefined') return;

        initXPChart();
        initVocabChart();
        initListeningChart();
    }

    function initXPChart() {
        var ctx = document.getElementById('xpChart');
        if (!ctx) return;

        var labels = getLast7Days();
        var data = getDataForDays(state.xp.daily, labels);

        xpChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    label: '多邻国 XP',
                    data: data,
                    backgroundColor: 'rgba(231, 111, 81, 0.6)',
                    borderColor: 'var(--accent-color)',
                    borderWidth: 1,
                    borderRadius: 6
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: { beginAtZero: true, grid: { color: 'rgba(0,0,0,0.05)' } },
                    x: { grid: { display: false } }
                },
                plugins: {
                    legend: { display: false }
                }
            }
        });
    }

    function initVocabChart() {
        var ctx = document.getElementById('vocabChart');
        if (!ctx) return;

        var labels = getLast7Days();
        var data = getAccumulatedData(state.vocabulary.daily, labels, state.vocabulary.total);

        vocabChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: '词汇量',
                    data: data,
                    borderColor: 'var(--accent-color)',
                    backgroundColor: 'rgba(231, 111, 81, 0.1)',
                    fill: true,
                    tension: 0.4,
                    pointRadius: 4,
                    pointBackgroundColor: 'var(--accent-color)'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: { beginAtZero: false, grid: { color: 'rgba(0,0,0,0.05)' } },
                    x: { grid: { display: false } }
                },
                plugins: {
                    legend: { display: false }
                }
            }
        });
    }

    function initListeningChart() {
        var ctx = document.getElementById('listeningChart');
        if (!ctx) return;

        var labels = getLast7Days();
        var data = getDataForDays(state.listening.history, labels);

        listeningChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: '听力理解率 %',
                    data: data,
                    borderColor: 'var(--success-color)',
                    backgroundColor: 'rgba(107, 144, 128, 0.1)',
                    fill: true,
                    tension: 0.4,
                    pointRadius: 4,
                    pointBackgroundColor: 'var(--success-color)'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: { beginAtZero: true, max: 100, grid: { color: 'rgba(0,0,0,0.05)' } },
                    x: { grid: { display: false } }
                },
                plugins: {
                    legend: { display: false }
                }
            }
        });
    }

    function updateCharts() {
        if (xpChart) {
            var labels = getLast7Days();
            xpChart.data.labels = labels;
            xpChart.data.datasets[0].data = getDataForDays(state.xp.daily, labels);
            xpChart.update();
        }
        if (vocabChart) {
            var labels2 = getLast7Days();
            vocabChart.data.labels = labels2;
            vocabChart.data.datasets[0].data = getAccumulatedData(state.vocabulary.daily, labels2, state.vocabulary.total);
            vocabChart.update();
        }
        if (listeningChart) {
            var labels3 = getLast7Days();
            listeningChart.data.labels = labels3;
            listeningChart.data.datasets[0].data = getDataForDays(state.listening.history, labels3);
            listeningChart.update();
        }
    }

    // ==================== XP Logging ====================
    window.logXP = function () {
        var input = document.getElementById('xpInput');
        if (!input) return;

        var xp = parseInt(input.value, 10);
        if (isNaN(xp) || xp <= 0) {
            showToast('请输入有效的XP值', 'error');
            return;
        }

        var today = getDateStr(new Date());
        state.xp.total += xp;

        var existing = findDailyEntry(state.xp.daily, today);
        if (existing) {
            existing.value += xp;
        } else {
            state.xp.daily.push({ date: today, value: xp });
        }

        addHistoryEntry('多邻国XP', '+' + xp + ' XP', 'success');
        incrementStreak();
        saveState();
        updateCharts();
        renderDashboardStats();
        renderGoalBanner();
        input.value = '';
        showToast('已记录 ' + xp + ' XP!', 'success');
    };

    // ==================== Progress Logging ====================
    window.logProgress = function (type) {
        var inputMap = {
            vocab: 'vocabInput',
            listen: 'listenInput',
            speak: 'speakInput',
            read: 'readInput',
            grammar: 'grammarInput'
        };
        var labelMap = {
            vocab: '词汇学习',
            listen: '听力训练',
            speak: '口语练习',
            read: '阅读理解',
            grammar: '语法练习'
        };
        var unitMap = {
            vocab: '词',
            listen: '分钟',
            speak: '分钟',
            read: '分钟',
            grammar: '题'
        };

        var input = document.getElementById(inputMap[type]);
        if (!input) return;

        var val = parseInt(input.value, 10);
        if (isNaN(val) || val <= 0) {
            showToast('请输入有效的数值', 'error');
            return;
        }

        state.dailyProgress[type] = (state.dailyProgress[type] || 0) + val;

        if (type === 'vocab') {
            state.vocabulary.total += val;
            var today = getDateStr(new Date());
            var existing = findDailyEntry(state.vocabulary.daily, today);
            if (existing) {
                existing.value += val;
            } else {
                state.vocabulary.daily.push({ date: today, value: val });
            }
        }

        if (type === 'listen') {
            var rateInput = document.getElementById('listenRateInput');
            if (rateInput && rateInput.value) {
                var rate = parseInt(rateInput.value, 10);
                if (!isNaN(rate) && rate >= 0 && rate <= 100) {
                    state.listening.rate = rate;
                    var today2 = getDateStr(new Date());
                    var existingRate = findDailyEntry(state.listening.history, today2);
                    if (existingRate) {
                        existingRate.value = rate;
                    } else {
                        state.listening.history.push({ date: today2, value: rate });
                    }
                    rateInput.value = '';
                }
            }
        }

        addHistoryEntry(labelMap[type], '+' + val + ' ' + unitMap[type], 'success');
        incrementStreak();
        saveState();
        renderDailyProgress();
        renderDashboardStats();
        renderGoalBanner();
        updateCharts();
        input.value = '';
        showToast(labelMap[type] + ': +' + val + ' ' + unitMap[type], 'success');
    };

    // ==================== Vocabulary ====================
    window.addVocabWord = function () {
        var wordInput = document.getElementById('newVocabWord');
        var meaningInput = document.getElementById('newVocabMeaning');
        if (!wordInput || !meaningInput) return;

        var word = wordInput.value.trim();
        var meaning = meaningInput.value.trim();
        if (!word || !meaning) {
            showToast('请输入单词和释义', 'error');
            return;
        }

        state.vocabulary.words.push({
            word: word,
            meaning: meaning,
            date: getDateStr(new Date())
        });

        saveState();
        renderVocabLog();
        wordInput.value = '';
        meaningInput.value = '';
        showToast('已添加: ' + word, 'success');
    };

    function renderVocabLog() {
        var container = document.getElementById('vocabLogList');
        if (!container) return;

        var words = state.vocabulary.words.slice(-20).reverse();
        container.innerHTML = '';

        if (words.length === 0) {
            container.innerHTML = '<p style="text-align:center;opacity:0.5;padding:20px">暂无单词记录</p>';
            return;
        }

        words.forEach(function (entry) {
            var div = document.createElement('div');
            div.className = 'vocab-log-entry';
            div.innerHTML =
                '<span class="vocab-word">' + escapeHtml(entry.word) + '</span>' +
                '<span class="vocab-meaning">' + escapeHtml(entry.meaning) + '</span>' +
                '<span class="vocab-date">' + entry.date + '</span>';
            container.appendChild(div);
        });
    }

    // ==================== Reminders ====================
    function renderReminders() {
        var list = document.getElementById('reminderList');
        if (!list) return;
        list.innerHTML = '';

        if (state.reminders.length === 0) {
            list.innerHTML = '<li style="padding:12px 0;text-align:center;opacity:0.5">暂无提醒</li>';
            return;
        }

        state.reminders.forEach(function (reminder, index) {
            var li = document.createElement('li');
            li.className = 'reminder-item';
            li.innerHTML =
                '<span class="reminder-time">' + reminder.time + '</span>' +
                '<span class="reminder-text">' + escapeHtml(reminder.text) + '</span>' +
                '<button class="reminder-delete" onclick="window.deleteReminder(' + index + ')"><i class="fas fa-times"></i></button>';
            list.appendChild(li);
        });
    }

    window.addReminder = function () {
        var timeInput = document.getElementById('reminderTime');
        var textInput = document.getElementById('reminderText');
        var typeInput = document.getElementById('reminderType');
        if (!timeInput || !textInput) return;

        var time = timeInput.value;
        var text = textInput.value.trim();
        if (!time || !text) {
            showToast('请填写提醒时间和内容', 'error');
            return;
        }

        state.reminders.push({
            time: time,
            text: text,
            type: typeInput ? typeInput.value : 'general',
            enabled: true
        });

        // Try to integrate with blog reminder API
        sendReminderToAPI(time, text, typeInput ? typeInput.value : 'general');

        saveState();
        renderReminders();
        timeInput.value = '';
        textInput.value = '';
        showToast('提醒已添加', 'success');
    };

    window.deleteReminder = function (index) {
        state.reminders.splice(index, 1);
        saveState();
        renderReminders();
        showToast('提醒已删除', 'warning');
    };

    // ==================== Resources ====================
    function renderResources() {
        var defaultResources = [
            { name: '多邻国 Duolingo', type: 'APP', icon: '🦉', url: 'https://www.duolingo.com' },
            { name: 'BBC Learning English', type: '听力', icon: '🎧', url: 'https://www.bbc.co.uk/learningenglish' },
            { name: 'Quizlet 单词卡', type: '词汇', icon: '📇', url: 'https://quizlet.com' },
            { name: 'Cambridge Dictionary', type: '词典', icon: '📖', url: 'https://dictionary.cambridge.org' },
            { name: 'TED Talks', type: '听力/口语', icon: '🎤', url: 'https://www.ted.com' },
            { name: 'Grammarly', type: '语法/写作', icon: '✍️', url: 'https://www.grammarly.com' }
        ];

        var resources = state.resources.length > 0 ? state.resources : defaultResources;
        var container = document.getElementById('resourceList');
        if (!container) return;
        container.innerHTML = '';

        resources.forEach(function (res) {
            var div = document.createElement('div');
            div.className = 'resource-item';
            div.innerHTML =
                '<div class="resource-icon">' + res.icon + '</div>' +
                '<div class="resource-info">' +
                    '<div class="resource-name">' + escapeHtml(res.name) + '</div>' +
                    '<div class="resource-type">' + escapeHtml(res.type) + '</div>' +
                '</div>' +
                '<a href="' + escapeHtml(res.url) + '" target="_blank" class="resource-link"><i class="fas fa-external-link-alt"></i></a>';
            container.appendChild(div);
        });
    }

    // ==================== History ====================
    function addHistoryEntry(activity, detail, status) {
        state.history.unshift({
            date: getDateStr(new Date()),
            time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
            activity: activity,
            detail: detail,
            status: status
        });
        // Keep last 100 entries
        if (state.history.length > 100) state.history = state.history.slice(0, 100);
        saveState();
        renderHistory();
    }

    function renderHistory() {
        var tbody = document.getElementById('historyBody');
        if (!tbody) return;
        tbody.innerHTML = '';

        var entries = state.history.slice(0, 20);
        if (entries.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;padding:20px;opacity:0.5">暂无学习记录</td></tr>';
            return;
        }

        entries.forEach(function (entry) {
            var tr = document.createElement('tr');
            var badgeClass = entry.status === 'success' ? 'badge-success' : (entry.status === 'warning' ? 'badge-warning' : 'badge-danger');
            tr.innerHTML =
                '<td>' + entry.date + ' ' + (entry.time || '') + '</td>' +
                '<td>' + escapeHtml(entry.activity) + '</td>' +
                '<td>' + escapeHtml(entry.detail) + '</td>' +
                '<td><span class="badge ' + badgeClass + '">' + (entry.status === 'success' ? '完成' : '进行中') + '</span></td>';
            tbody.appendChild(tr);
        });
    }

    // ==================== Heatmap ====================
    function renderHeatmap() {
        var container = document.getElementById('heatmapGrid');
        if (!container) return;
        container.innerHTML = '';

        // Show last 28 days (4 weeks)
        for (var i = 27; i >= 0; i--) {
            var date = new Date(Date.now() - i * 86400000);
            var dateStr = getDateStr(date);

            var xpEntry = findDailyEntry(state.xp.daily, dateStr);
            var vocabEntry = findDailyEntry(state.vocabulary.daily, dateStr);
            var totalActivity = (xpEntry ? xpEntry.value : 0) + (vocabEntry ? vocabEntry.value * 5 : 0);

            var level = 0;
            if (totalActivity > 0) level = 1;
            if (totalActivity >= 30) level = 2;
            if (totalActivity >= 80) level = 3;
            if (totalActivity >= 150) level = 4;

            var cell = document.createElement('div');
            cell.className = 'heatmap-cell level-' + level;
            cell.title = dateStr + ': ' + totalActivity + ' 活跃度';
            cell.textContent = date.getDate();
            container.appendChild(cell);
        }
    }

    // ==================== Reports ====================
    window.generateReport = function (type) {
        document.querySelectorAll('.report-tab').forEach(function (t) { t.classList.remove('active'); });
        event.target.classList.add('active');

        var days = type === 'weekly' ? 7 : 30;
        var label = type === 'weekly' ? '周度' : '月度';

        var totalXP = 0, totalVocab = 0, studyDays = 0;
        var avgListening = 0, listenCount = 0;

        for (var i = 0; i < days; i++) {
            var dateStr = getDateStr(new Date(Date.now() - i * 86400000));
            var xpEntry = findDailyEntry(state.xp.daily, dateStr);
            var vocabEntry = findDailyEntry(state.vocabulary.daily, dateStr);
            var listenEntry = findDailyEntry(state.listening.history, dateStr);

            if (xpEntry) { totalXP += xpEntry.value; studyDays++; }
            if (vocabEntry) totalVocab += vocabEntry.value;
            if (listenEntry) { avgListening += listenEntry.value; listenCount++; }
        }

        if (listenCount > 0) avgListening = Math.round(avgListening / listenCount);

        var summaryEl = document.getElementById('reportSummary');
        if (summaryEl) {
            summaryEl.innerHTML =
                '<div class="report-summary-card"><div class="summary-value">' + totalXP + '</div><div class="summary-label">总XP</div></div>' +
                '<div class="report-summary-card"><div class="summary-value">' + totalVocab + '</div><div class="summary-label">新词汇</div></div>' +
                '<div class="report-summary-card"><div class="summary-value">' + studyDays + '/' + days + '</div><div class="summary-label">学习天数</div></div>' +
                '<div class="report-summary-card"><div class="summary-value">' + avgListening + '%</div><div class="summary-label">平均听力率</div></div>' +
                '<div class="report-summary-card"><div class="summary-value">' + state.streak.current + '</div><div class="summary-label">当前连续</div></div>' +
                '<div class="report-summary-card"><div class="summary-value">' + Math.round(totalXP / Math.max(studyDays, 1)) + '</div><div class="summary-label">日均XP</div></div>';
        }

        showToast(label + '报告已生成', 'success');
    };

    // ==================== Modal ====================
    function openModal() {
        var overlay = document.getElementById('modalOverlay');
        if (overlay) overlay.classList.add('active');
    }

    window.closeModal = function () {
        var overlay = document.getElementById('modalOverlay');
        if (overlay) overlay.classList.remove('active');
    };

    // ==================== Log Daily Progress Modal ====================
    window.openLogModal = function () {
        var title = document.getElementById('modalTitle');
        var body = document.getElementById('modalBody');
        if (!title || !body) return;

        title.textContent = '记录今日学习';
        body.innerHTML =
            '<div class="form-group"><label>词汇学习 (个数)</label><input type="number" id="vocabInput" min="0" placeholder="今日学习的词汇数"></div>' +
            '<div style="text-align:right"><button class="btn-primary btn-small" onclick="window.logProgress(\'vocab\')">记录</button></div>' +

            '<div class="form-group"><label>听力训练 (分钟)</label><input type="number" id="listenInput" min="0" placeholder="听力练习时长"></div>' +
            '<div class="form-group"><label>听力理解率 (%)</label><input type="number" id="listenRateInput" min="0" max="100" placeholder="可选"></div>' +
            '<div style="text-align:right"><button class="btn-primary btn-small" onclick="window.logProgress(\'listen\')">记录</button></div>' +

            '<div class="form-group"><label>口语练习 (分钟)</label><input type="number" id="speakInput" min="0" placeholder="口语练习时长"></div>' +
            '<div style="text-align:right"><button class="btn-primary btn-small" onclick="window.logProgress(\'speak\')">记录</button></div>' +

            '<div class="form-group"><label>阅读理解 (分钟)</label><input type="number" id="readInput" min="0" placeholder="阅读时长"></div>' +
            '<div style="text-align:right"><button class="btn-primary btn-small" onclick="window.logProgress(\'read\')">记录</button></div>' +

            '<div class="form-group"><label>语法练习 (题数)</label><input type="number" id="grammarInput" min="0" placeholder="完成的语法练习题数"></div>' +
            '<div style="text-align:right"><button class="btn-primary btn-small" onclick="window.logProgress(\'grammar\')">记录</button></div>';

        openModal();
    };

    // ==================== Vocab Modal ====================
    window.openVocabModal = function () {
        var title = document.getElementById('modalTitle');
        var body = document.getElementById('modalBody');
        if (!title || !body) return;

        title.textContent = '添加新单词';
        body.innerHTML =
            '<div class="form-group"><label>英文单词</label><input type="text" id="newVocabWord" placeholder="English word"></div>' +
            '<div class="form-group"><label>中文释义</label><input type="text" id="newVocabMeaning" placeholder="中文意思"></div>' +
            '<button class="btn-primary" onclick="window.addVocabWord()" style="width:100%">添加</button>' +
            '<div id="vocabLogList" style="margin-top:16px"></div>';

        openModal();
        renderVocabLog();
    };

    // ==================== API Integration ====================
    function loadFromAPI() {
        // Try loading from blog system
        fetch('/api/blog/search?tag=english-learning', { credentials: 'same-origin' })
            .then(function (res) { return res.ok ? res.json() : null; })
            .then(function (data) {
                if (data && data.blogs) {
                    // Process blog entries for learning data
                    console.log('Loaded English learning blogs:', data.blogs.length);
                }
            })
            .catch(function () {
                // API not available, use local data
            });
    }

    function sendReminderToAPI(time, text, type) {
        // Integrate with blog system reminder API
        fetch('/api/reminder/add', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ time: time, text: text, type: type, category: 'english-learning' })
        }).catch(function () {
            // API not available, local only
        });
    }

    window.syncWithTodolist = function () {
        // Create a todolist task for today's English learning
        var tasks = [];
        var items = [
            { key: 'vocab', label: '词汇学习', goal: state.dailyGoals.vocab, unit: '词' },
            { key: 'listen', label: '听力训练', goal: state.dailyGoals.listen, unit: '分钟' },
            { key: 'speak', label: '口语练习', goal: state.dailyGoals.speak, unit: '分钟' },
            { key: 'read', label: '阅读理解', goal: state.dailyGoals.read, unit: '分钟' },
            { key: 'grammar', label: '语法练习', goal: state.dailyGoals.grammar, unit: '题' }
        ];

        items.forEach(function (item) {
            var current = state.dailyProgress[item.key] || 0;
            if (current < item.goal) {
                tasks.push('英语' + item.label + ': ' + (item.goal - current) + item.unit + '待完成');
            }
        });

        if (tasks.length === 0) {
            showToast('今日英语学习任务已全部完成!', 'success');
            return;
        }

        fetch('/api/todolist/add', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tasks: tasks, category: 'english-learning' })
        })
        .then(function () { showToast('已同步到待办任务', 'success'); })
        .catch(function () { showToast('同步失败，请手动添加到待办', 'warning'); });
    };

    // ==================== Data Export ====================
    window.exportData = function () {
        var dataStr = JSON.stringify(state, null, 2);
        var blob = new Blob([dataStr], { type: 'application/json' });
        var url = URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = url;
        a.download = 'english-learning-data-' + getDateStr(new Date()) + '.json';
        a.click();
        URL.revokeObjectURL(url);
        showToast('数据已导出', 'success');
    };

    window.importData = function () {
        var input = document.createElement('input');
        input.type = 'file';
        input.accept = '.json';
        input.onchange = function (e) {
            var file = e.target.files[0];
            if (!file) return;
            var reader = new FileReader();
            reader.onload = function (ev) {
                try {
                    var imported = JSON.parse(ev.target.result);
                    state = { ...DEFAULT_STATE, ...imported };
                    saveState();
                    renderAll();
                    updateCharts();
                    showToast('数据已导入', 'success');
                } catch (err) {
                    showToast('导入失败: 文件格式错误', 'error');
                }
            };
            reader.readAsText(file);
        };
        input.click();
    };

    // ==================== Reset Daily Progress ====================
    window.resetDailyProgress = function () {
        if (!confirm('确定要重置今日进度吗？')) return;
        state.dailyProgress = { vocab: 0, listen: 0, speak: 0, read: 0, grammar: 0 };
        saveState();
        renderDailyProgress();
        showToast('今日进度已重置', 'warning');
    };

    // ==================== Helpers ====================
    function getDateStr(date) {
        var y = date.getFullYear();
        var m = String(date.getMonth() + 1).padStart(2, '0');
        var d = String(date.getDate()).padStart(2, '0');
        return y + '-' + m + '-' + d;
    }

    function getLast7Days() {
        var days = [];
        for (var i = 6; i >= 0; i--) {
            var d = new Date(Date.now() - i * 86400000);
            days.push((d.getMonth() + 1) + '/' + d.getDate());
        }
        return days;
    }

    function getDataForDays(entries, labels) {
        return labels.map(function (label) {
            var parts = label.split('/');
            var month = parseInt(parts[0], 10);
            var day = parseInt(parts[1], 10);
            var now = new Date();
            var year = now.getFullYear();
            var dateStr = year + '-' + String(month).padStart(2, '0') + '-' + String(day).padStart(2, '0');
            var entry = findDailyEntry(entries, dateStr);
            return entry ? entry.value : 0;
        });
    }

    function getAccumulatedData(entries, labels, currentTotal) {
        var data = getDataForDays(entries, labels);
        // Calculate accumulated from end
        var result = [];
        var acc = currentTotal;
        for (var i = data.length - 1; i >= 0; i--) {
            result[i] = acc;
            acc -= data[i];
        }
        return result;
    }

    function findDailyEntry(entries, dateStr) {
        if (!entries) return null;
        for (var i = 0; i < entries.length; i++) {
            if (entries[i].date === dateStr) return entries[i];
        }
        return null;
    }

    function escapeHtml(str) {
        var div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    function showToast(message, type) {
        var container = document.getElementById('toastContainer');
        if (!container) {
            container = document.createElement('div');
            container.id = 'toastContainer';
            container.className = 'toast-container';
            document.body.appendChild(container);
        }

        var toast = document.createElement('div');
        toast.className = 'toast ' + (type || 'success');
        toast.textContent = message;
        container.appendChild(toast);

        setTimeout(function () {
            toast.style.opacity = '0';
            setTimeout(function () { toast.remove(); }, 300);
        }, 3500);
    }
})();
