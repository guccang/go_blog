/**
 * Agent Notifier - Handles WebSocket connection and notifications for Agent
 * Can be included in any page to receive agent notifications.
 */

(function() {
    // 防止重复初始化
    if (window.AgentNotifier) return;

    const AgentNotifier = {
        ws: null,
        reconnectTimer: null,
        listeners: [],

        init: function() {
            this.requestNotificationPermission();
            this.connect();
            this.injectStyles();
        },

        // 请求通知权限
        requestNotificationPermission: function() {
            if ("Notification" in window && Notification.permission !== "granted") {
                Notification.requestPermission();
            }
        },

        // 连接 WebSocket
        connect: function() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            this.ws = new WebSocket(`${protocol}//${window.location.host}/ws/agent/notifications`);

            this.ws.onopen = () => {
                console.log('[Agent] WebSocket connected');
                this.updateStatus(true);
                if (this.reconnectTimer) {
                    clearTimeout(this.reconnectTimer);
                    this.reconnectTimer = null;
                }
            };

            this.ws.onclose = () => {
                console.log('[Agent] WebSocket disconnected');
                this.updateStatus(false);
                // 5秒后重连
                this.reconnectTimer = setTimeout(() => this.connect(), 5000);
            };

            this.ws.onmessage = (event) => {
                try {
                    const notification = JSON.parse(event.data);
                    this.handleNotification(notification);
                } catch (e) {
                    console.error('[Agent] Failed to parse notification:', e);
                }
            };
        },

        // 处理通知
        handleNotification: function(notification) {
            // 触发所有监听器
            this.listeners.forEach(callback => callback(notification));

            // 处理提醒
            if (notification.type === 'reminder' || notification.type === 'notification') {
                this.showToast(notification.message);
                this.showSystemNotification(notification.message);
            }
        },

        // 显示页面内 Toast
        showToast: function(message) {
            let container = document.getElementById('agent-toast-container');
            if (!container) {
                container = document.createElement('div');
                container.id = 'agent-toast-container';
                container.style.cssText = `
                    position: fixed;
                    top: 80px;
                    right: 20px;
                    z-index: 9999;
                    display: flex;
                    flex-direction: column;
                    gap: 10px;
                    pointer-events: none;
                `;
                document.body.appendChild(container);
            }

            const toast = document.createElement('div');
            toast.style.cssText = `
                background: linear-gradient(135deg, #6366f1, #a855f7);
                color: white;
                padding: 16px 24px;
                border-radius: 12px;
                box-shadow: 0 10px 40px rgba(99, 102, 241, 0.5);
                animation: agentSlideIn 0.3s ease-out;
                max-width: 400px;
                cursor: pointer;
                pointer-events: auto;
                font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
                display: flex;
                align-items: center;
                justify-content: space-between;
            `;
            
            toast.innerHTML = `
                <div style="display: flex; align-items: center; gap: 10px;">
                    <span>🔔</span>
                    <span>${this.escapeHtml(message)}</span>
                </div>
                <span style="margin-left: 10px; font-size: 1.2em;">&times;</span>
            `;
            
            toast.onclick = () => {
                toast.style.animation = 'agentSlideOut 0.3s ease-in';
                setTimeout(() => {
                    toast.remove();
                    if (container.children.length === 0) {
                        container.remove();
                    }
                }, 300);
            };
            
            container.appendChild(toast);
        },

        // 显示系统通知
        showSystemNotification: function(message) {
            if ("Notification" in window && Notification.permission === "granted") {
                // 检查页面是否可见，如果可见则不一定需要系统通知（或者两者都显示）
                // 这里选择始终显示，确保用户不错过
                new Notification("Agent 提醒", {
                    body: message,
                    icon: '/statics/logo/favicon.ico' // 假设有这个图标
                });
            }
        },

        // 注册监听器
        addListener: function(callback) {
            this.listeners.push(callback);
        },

        // 移除监听器
        removeListener: function(callback) {
            this.listeners = this.listeners.filter(cb => cb !== callback);
        },

        // 更新连接状态 UI (如果存在)
        updateStatus: function(connected) {
            const indicator = document.getElementById('wsIndicator');
            const text = document.getElementById('wsStatusText');
            if (indicator && text) {
                if (connected) {
                    indicator.classList.add('connected');
                    text.textContent = '已连接';
                } else {
                    indicator.classList.remove('connected');
                    text.textContent = '未连接';
                }
            }
        },

        injectStyles: function() {
            const style = document.createElement('style');
            style.textContent = `
                @keyframes agentSlideIn {
                    from { transform: translateX(100%); opacity: 0; }
                    to { transform: translateX(0); opacity: 1; }
                }
                @keyframes agentSlideOut {
                    from { transform: translateX(0); opacity: 1; }
                    to { transform: translateX(100%); opacity: 0; }
                }
            `;
            document.head.appendChild(style);
        },

        escapeHtml: function(str) {
            const div = document.createElement('div');
            div.textContent = str;
            return div.innerHTML;
        }
    };

    window.AgentNotifier = AgentNotifier;
    AgentNotifier.init();
})();
