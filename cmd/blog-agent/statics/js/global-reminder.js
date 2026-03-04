/* 全局提醒组件 - 在所有页面显示 */

(function() {
    'use strict';

    // 提醒容器
    let reminderContainer = null;
    // WebSocket 连接
    let ws = null;
    // 当前显示的提醒 (按类型去重)
    let activeReminders = {};
    // 重连定时器
    let reconnectTimer = null;

    // 初始化
    function init() {
        createContainer();
        connectWebSocket();
    }

    // 创建提醒容器
    function createContainer() {
        if (document.getElementById('global-reminder-container')) return;

        reminderContainer = document.createElement('div');
        reminderContainer.id = 'global-reminder-container';
        reminderContainer.innerHTML = '';
        document.body.appendChild(reminderContainer);

        // 添加样式
        const style = document.createElement('style');
        style.textContent = `
            #global-reminder-container {
                position: fixed;
                top: 20px;
                right: 20px;
                z-index: 10000;
                display: flex;
                flex-direction: column;
                gap: 10px;
                max-width: 400px;
                pointer-events: none;
            }

            .global-reminder {
                background: linear-gradient(135deg, #6366f1, #a855f7);
                color: white;
                padding: 16px 20px;
                border-radius: 12px;
                box-shadow: 0 10px 40px rgba(99, 102, 241, 0.5);
                display: flex;
                align-items: center;
                gap: 12px;
                cursor: pointer;
                pointer-events: auto;
                animation: slideIn 0.3s ease-out;
                transition: all 0.3s ease;
                position: relative;
                overflow: hidden;
            }

            .global-reminder:hover {
                transform: translateX(-5px);
                box-shadow: 0 15px 50px rgba(99, 102, 241, 0.7);
            }

            .global-reminder::before {
                content: '';
                position: absolute;
                top: 0;
                left: 0;
                width: 4px;
                height: 100%;
                background: #fbbf24;
            }

            .global-reminder .icon {
                font-size: 1.5rem;
                flex-shrink: 0;
            }

            .global-reminder .content {
                flex: 1;
            }

            .global-reminder .title {
                font-weight: 600;
                font-size: 0.9rem;
                margin-bottom: 4px;
            }

            .global-reminder .message {
                font-size: 0.85rem;
                opacity: 0.9;
            }

            .global-reminder .close-hint {
                font-size: 0.75rem;
                opacity: 0.6;
                margin-top: 6px;
            }

            .global-reminder .time {
                font-size: 0.75rem;
                opacity: 0.7;
                position: absolute;
                top: 8px;
                right: 12px;
            }

            .global-reminder.closing {
                animation: slideOut 0.3s ease-in forwards;
            }

            @keyframes slideIn {
                from {
                    transform: translateX(120%);
                    opacity: 0;
                }
                to {
                    transform: translateX(0);
                    opacity: 1;
                }
            }

            @keyframes slideOut {
                from {
                    transform: translateX(0);
                    opacity: 1;
                }
                to {
                    transform: translateX(120%);
                    opacity: 0;
                }
            }

            /* 提醒类型样式 */
            .global-reminder.reminder {
                background: linear-gradient(135deg, #6366f1, #a855f7);
            }

            .global-reminder.notification {
                background: linear-gradient(135deg, #10b981, #059669);
            }

            .global-reminder.warning {
                background: linear-gradient(135deg, #f59e0b, #d97706);
            }

            .global-reminder.error {
                background: linear-gradient(135deg, #ef4444, #dc2626);
            }
        `;
        document.head.appendChild(style);
    }

    // 连接 WebSocket
    function connectWebSocket() {
        if (ws && ws.readyState === WebSocket.OPEN) return;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/agent/notifications`;

        try {
            ws = new WebSocket(wsUrl);

            ws.onopen = function() {
                console.log('[GlobalReminder] WebSocket connected');
                if (reconnectTimer) {
                    clearTimeout(reconnectTimer);
                    reconnectTimer = null;
                }
            };

            ws.onclose = function() {
                console.log('[GlobalReminder] WebSocket disconnected');
                scheduleReconnect();
            };

            ws.onerror = function(error) {
                console.error('[GlobalReminder] WebSocket error:', error);
            };

            ws.onmessage = function(event) {
                try {
                    const data = JSON.parse(event.data);
                    handleNotification(data);
                } catch (e) {
                    console.error('[GlobalReminder] Parse error:', e);
                }
            };
        } catch (e) {
            console.error('[GlobalReminder] Connection failed:', e);
            scheduleReconnect();
        }
    }

    // 重连计划
    function scheduleReconnect() {
        if (reconnectTimer) return;
        reconnectTimer = setTimeout(() => {
            reconnectTimer = null;
            connectWebSocket();
        }, 5000);
    }

    // 处理通知
    function handleNotification(data) {
        // 只处理提醒和通知类型
        const allowedTypes = ['reminder', 'notification', 'smart_reminder', 'report_generated'];
        if (!allowedTypes.includes(data.type)) {
            return;
        }

        // 使用 task_id 或 type 作为唯一标识
        const reminderId = data.task_id || data.type;

        // 如果同类型提醒已存在，先移除旧的
        if (activeReminders[reminderId]) {
            removeReminder(reminderId, false);
        }

        // 显示新提醒
        showReminder(reminderId, data);
    }

    // 显示提醒
    function showReminder(id, data) {
        const reminder = document.createElement('div');
        reminder.className = `global-reminder ${data.type || 'reminder'}`;
        reminder.dataset.id = id;

        const icon = data.type === 'notification' ? '📬' : '🔔';
        const title = data.type === 'notification' ? '通知' : '提醒';
        const time = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });

        reminder.innerHTML = `
            <span class="icon">${icon}</span>
            <div class="content">
                <div class="title">${title}</div>
                <div class="message">${escapeHtml(data.message || '')}</div>
                <div class="close-hint">点击关闭</div>
            </div>
            <span class="time">${time}</span>
        `;

        // 点击关闭
        reminder.addEventListener('click', function() {
            removeReminder(id, true);
        });

        reminderContainer.appendChild(reminder);
        activeReminders[id] = reminder;

        // 播放提示音（可选）
        playNotificationSound();
    }

    // 移除提醒
    function removeReminder(id, animate) {
        const reminder = activeReminders[id];
        if (!reminder) return;

        if (animate) {
            reminder.classList.add('closing');
            setTimeout(() => {
                if (reminder.parentNode) {
                    reminder.parentNode.removeChild(reminder);
                }
            }, 300);
        } else {
            if (reminder.parentNode) {
                reminder.parentNode.removeChild(reminder);
            }
        }

        delete activeReminders[id];
    }

    // 转义 HTML
    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    // 播放提示音
    function playNotificationSound() {
        try {
            // 使用 Web Audio API 创建简单提示音
            const audioContext = new (window.AudioContext || window.webkitAudioContext)();
            const oscillator = audioContext.createOscillator();
            const gainNode = audioContext.createGain();

            oscillator.connect(gainNode);
            gainNode.connect(audioContext.destination);

            oscillator.frequency.value = 800;
            oscillator.type = 'sine';
            gainNode.gain.setValueAtTime(0.1, audioContext.currentTime);
            gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.3);

            oscillator.start(audioContext.currentTime);
            oscillator.stop(audioContext.currentTime + 0.3);
        } catch (e) {
            // 忽略音频播放错误
        }
    }

    // 页面加载后初始化
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    // 暴露全局 API（可选）
    window.GlobalReminder = {
        show: function(message, type) {
            handleNotification({
                type: type || 'notification',
                message: message,
                task_id: 'manual_' + Date.now()
            });
        },
        clear: function() {
            Object.keys(activeReminders).forEach(id => removeReminder(id, true));
        }
    };

})();
