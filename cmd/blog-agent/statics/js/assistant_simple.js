// 智能助手页面JavaScript - 简化版

console.log('🚀 简化版assistant.js开始加载');

// 基础变量
let isTyping = false;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    console.log('🔧 DOMContentLoaded 事件触发');
    
    try {
        setupBasicEventListeners();
        console.log('✅ 基础事件监听器设置完成');
    } catch (error) {
        console.error('❌ 设置事件监听器出错:', error);
    }
});

// 设置基础事件监听器
function setupBasicEventListeners() {
    console.log('🎯 开始设置基础事件监听器');
    
    // 获取设置相关元素
    const settingsBtn = document.getElementById('settingsBtn');
    const settingsPanel = document.getElementById('settingsPanel');
    const closeSettings = document.getElementById('closeSettings');
    
    console.log('🎯 设置按钮:', settingsBtn ? '找到' : '未找到');
    console.log('🎯 设置面板:', settingsPanel ? '找到' : '未找到');
    console.log('🎯 关闭按钮:', closeSettings ? '找到' : '未找到');
    
    if (!settingsBtn || !settingsPanel) {
        console.error('❌ 关键元素缺失，无法设置事件');
        return;
    }
    
    // 简单直接的事件绑定
    console.log('🎯 开始绑定设置按钮事件');
    
    // 方法1: 使用onclick/ontouchend属性
    settingsBtn.ontouchend = function(e) {
        console.log('📱 设置按钮 ontouchend 触发');
        e.preventDefault();
        settingsPanel.classList.add('active');
        console.log('✅ 设置面板已打开');
        return false;
    };
    
    settingsBtn.onclick = function(e) {
        console.log('🖱️ 设置按钮 onclick 触发');
        e.preventDefault();
        settingsPanel.classList.add('active');
        console.log('✅ 设置面板已打开');
        return false;
    };
    
    // 关闭按钮事件
    if (closeSettings) {
        closeSettings.ontouchend = function(e) {
            console.log('📱 关闭按钮 ontouchend 触发');
            e.preventDefault();
            settingsPanel.classList.remove('active');
            console.log('✅ 设置面板已关闭');
            return false;
        };
        
        closeSettings.onclick = function(e) {
            console.log('🖱️ 关闭按钮 onclick 触发');
            e.preventDefault();
            settingsPanel.classList.remove('active');
            console.log('✅ 设置面板已关闭');
            return false;
        };
    }
    
    // 方法2: 使用addEventListener作为备用
    settingsBtn.addEventListener('touchstart', function(e) {
        console.log('📱 addEventListener touchstart');
        this.style.transform = 'scale(0.95)';
    }, {passive: true});
    
    settingsBtn.addEventListener('touchend', function(e) {
        console.log('📱 addEventListener touchend');
        this.style.transform = 'scale(1)';
        e.preventDefault();
        settingsPanel.classList.add('active');
        console.log('✅ 设置面板已打开 (addEventListener)');
    }, {passive: false});
    
    console.log('✅ 所有事件绑定完成');
}

// 全局测试函数
window.testOpen = function() {
    console.log('🧪 测试打开设置面板');
    const panel = document.getElementById('settingsPanel');
    if (panel) {
        panel.classList.add('active');
        console.log('✅ 设置面板已打开');
    } else {
        console.log('❌ 找不到设置面板');
    }
};

window.testClose = function() {
    console.log('🧪 测试关闭设置面板');
    const panel = document.getElementById('settingsPanel');
    if (panel) {
        panel.classList.remove('active');
        console.log('✅ 设置面板已关闭');
    } else {
        console.log('❌ 找不到设置面板');
    }
};

console.log('🚀 简化版assistant.js加载完成');