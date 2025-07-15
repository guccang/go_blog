// 配置管理页面JavaScript

let allConfigs = {};
let originalConfigs = {};
let filteredConfigs = {};

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    loadConfigs();
    initializeEventListeners();
});

// 初始化事件监听器
function initializeEventListeners() {
    // 添加键盘快捷键
    document.addEventListener('keydown', function(e) {
        if (e.ctrlKey && e.key === 's') {
            e.preventDefault();
            saveAllConfigs();
        }
        if (e.key === 'Escape') {
            closeAddModal();
        }
    });

    // 点击模态窗口外部关闭
    document.getElementById('addConfigModal').addEventListener('click', function(e) {
        if (e.target === this) {
            closeAddModal();
        }
    });
}

// 加载配置数据
async function loadConfigs() {
    try {
        showToast('正在加载配置...', 'info');
        
        const response = await fetch('/api/config', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }

        const data = await response.json();
        
        if (data.success) {
            allConfigs = data.configs || {};
            originalConfigs = JSON.parse(JSON.stringify(allConfigs)); // 深拷贝
            filteredConfigs = JSON.parse(JSON.stringify(allConfigs));
            
            renderConfigs();
            updateRawPreview();
            updateConfigCount();
            
            if (data.is_default) {
                showToast('配置文件不存在，已创建带详细注释的默认配置。请根据需要调整配置项。', 'warning');
            } else {
                showToast('配置加载成功', 'success');
            }
        } else {
            throw new Error('加载配置失败');
        }
    } catch (error) {
        console.error('加载配置失败:', error);
        showToast('加载配置失败: ' + error.message, 'error');
    }
}

// 渲染配置列表
function renderConfigs() {
    const configList = document.getElementById('configList');
    configList.innerHTML = '';

    const sortedKeys = Object.keys(filteredConfigs).sort();
    
    if (sortedKeys.length === 0) {
        configList.innerHTML = `
            <div class="empty-state">
                <h3>没有找到配置项</h3>
                <p>您可以添加新的配置项或检查搜索条件</p>
                <button class="btn btn-primary" onclick="addNewConfig()">添加配置</button>
            </div>
        `;
        return;
    }

    sortedKeys.forEach(key => {
        const value = filteredConfigs[key];
        const configItem = createConfigItem(key, value);
        configList.appendChild(configItem);
    });
}

// 创建配置项元素
function createConfigItem(key, value) {
    const item = document.createElement('div');
    item.className = 'config-item';
    item.dataset.originalKey = key;
    
    // 检查是否是新配置或修改的配置
    if (!originalConfigs.hasOwnProperty(key)) {
        item.classList.add('new');
    } else if (originalConfigs[key] !== value) {
        item.classList.add('modified');
    }

    item.innerHTML = `
        <div class="config-key">
            <div class="config-key-label">配置项名称</div>
            <input type="text" class="config-key-input" value="${escapeHtml(key)}" 
                   onchange="updateConfigKey('${escapeHtml(key)}', this.value)"
                   title="配置项名称，建议使用小写字母和下划线">
        </div>
        <div class="config-value">
            <div class="config-value-label">配置值</div>
            <input type="text" class="config-value-input" value="${escapeHtml(value)}" 
                   onchange="updateConfigValue('${escapeHtml(key)}', this.value)"
                   title="配置项的值">
            <div class="config-type-hint">${getConfigTypeHint(value)}</div>
        </div>
        <div class="config-actions">
            <button class="btn btn-warning" onclick="resetConfig('${escapeHtml(key)}')" 
                    title="重置为原始值">重置</button>
            <button class="btn btn-danger" onclick="deleteConfig('${escapeHtml(key)}')" 
                    title="删除此配置项">删除</button>
        </div>
    `;

    return item;
}

// 获取配置值类型提示
function getConfigTypeHint(value) {
    if (value === 'true' || value === 'false') {
        return '布尔值 (true/false)';
    }
    if (/^\d+$/.test(value)) {
        return '整数';
    }
    if (/^\d+\.\d+$/.test(value)) {
        return '小数';
    }
    if (value.includes('|')) {
        return '列表值 (用|分隔)';
    }
    if (value.includes('/') || value.includes('\\')) {
        return '路径';
    }
    return '字符串';
}

// 更新配置项键名
function updateConfigKey(oldKey, newKey) {
    newKey = newKey.trim();
    
    if (newKey === oldKey) {
        return;
    }
    
    if (newKey === '') {
        showToast('配置项名称不能为空', 'error');
        return;
    }
    
    if (allConfigs.hasOwnProperty(newKey) && newKey !== oldKey) {
        showToast('配置项名称已存在', 'error');
        return;
    }
    
    // 验证配置键名格式
    if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(newKey)) {
        showToast('配置项名称只能包含字母、数字和下划线，且必须以字母开头', 'error');
        return;
    }
    
    // 更新配置
    const value = allConfigs[oldKey];
    delete allConfigs[oldKey];
    allConfigs[newKey] = value;
    
    // 如果在筛选结果中，也要更新
    if (filteredConfigs.hasOwnProperty(oldKey)) {
        delete filteredConfigs[oldKey];
        filteredConfigs[newKey] = value;
    }
    
    renderConfigs();
    updateRawPreview();
    showToast('配置项名称已更新', 'success');
}

// 更新配置项值
function updateConfigValue(key, value) {
    allConfigs[key] = value;
    if (filteredConfigs.hasOwnProperty(key)) {
        filteredConfigs[key] = value;
    }
    
    updateRawPreview();
    
    // 更新配置项的状态样式
    const configItems = document.querySelectorAll('.config-item');
    configItems.forEach(item => {
        if (item.dataset.originalKey === key) {
            item.classList.remove('new', 'modified');
            if (!originalConfigs.hasOwnProperty(key)) {
                item.classList.add('new');
            } else if (originalConfigs[key] !== value) {
                item.classList.add('modified');
            }
        }
    });
}

// 重置配置项
function resetConfig(key) {
    if (originalConfigs.hasOwnProperty(key)) {
        allConfigs[key] = originalConfigs[key];
        if (filteredConfigs.hasOwnProperty(key)) {
            filteredConfigs[key] = originalConfigs[key];
        }
        renderConfigs();
        updateRawPreview();
        showToast('配置项已重置', 'success');
    } else {
        // 如果是新配置，直接删除
        deleteConfig(key);
    }
}

// 删除配置项
function deleteConfig(key) {
    if (confirm(`确定要删除配置项 "${key}" 吗？`)) {
        delete allConfigs[key];
        delete filteredConfigs[key];
        renderConfigs();
        updateRawPreview();
        updateConfigCount();
        showToast('配置项已删除', 'success');
    }
}

// 过滤配置项
function filterConfigs() {
    const searchText = document.getElementById('searchInput').value.toLowerCase();
    
    if (searchText === '') {
        filteredConfigs = JSON.parse(JSON.stringify(allConfigs));
    } else {
        filteredConfigs = {};
        Object.keys(allConfigs).forEach(key => {
            const value = allConfigs[key];
            if (key.toLowerCase().includes(searchText) || 
                value.toLowerCase().includes(searchText)) {
                filteredConfigs[key] = value;
            }
        });
    }
    
    renderConfigs();
    updateConfigCount();
}

// 更新配置项计数
function updateConfigCount() {
    const total = Object.keys(allConfigs).length;
    const filtered = Object.keys(filteredConfigs).length;
    const countElement = document.getElementById('configCount');
    
    if (total === filtered) {
        countElement.textContent = `配置项: ${total}`;
    } else {
        countElement.textContent = `配置项: ${filtered} / ${total}`;
    }
}

// 更新原始配置预览
function updateRawPreview() {
    const preview = document.getElementById('rawConfigPreview');
    const lines = [];
    
    lines.push('# 系统配置文件');
    lines.push('# 格式: key=value');
    lines.push('# 注释行以#开头');
    lines.push('');
    
    const sortedKeys = Object.keys(allConfigs).sort();
    sortedKeys.forEach(key => {
        const value = allConfigs[key];
        if (key && value !== undefined) {
            lines.push(`${key}=${value}`);
        }
    });
    
    preview.value = lines.join('\n');
}

// 添加新配置
function addNewConfig() {
    document.getElementById('newConfigKey').value = '';
    document.getElementById('newConfigValue').value = '';
    document.getElementById('newConfigDescription').value = '';
    document.getElementById('addConfigModal').style.display = 'block';
    document.getElementById('newConfigKey').focus();
}

// 关闭添加配置模态窗口
function closeAddModal() {
    document.getElementById('addConfigModal').style.display = 'none';
}

// 确认添加配置
function confirmAddConfig() {
    const key = document.getElementById('newConfigKey').value.trim();
    const value = document.getElementById('newConfigValue').value.trim();
    
    if (!key) {
        showToast('请输入配置项名称', 'error');
        return;
    }
    
    if (!value) {
        showToast('请输入配置值', 'error');
        return;
    }
    
    // 验证配置键名格式
    if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(key)) {
        showToast('配置项名称只能包含字母、数字和下划线，且必须以字母开头', 'error');
        return;
    }
    
    if (allConfigs.hasOwnProperty(key)) {
        showToast('配置项名称已存在', 'error');
        return;
    }
    
    // 添加配置
    allConfigs[key] = value;
    filteredConfigs[key] = value;
    
    closeAddModal();
    renderConfigs();
    updateRawPreview();
    updateConfigCount();
    showToast('配置项已添加', 'success');
}

// 保存所有配置
async function saveAllConfigs() {
    try {
        showToast('正在保存配置...', 'info');
        
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                configs: allConfigs
            })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`HTTP ${response.status}: ${errorText}`);
        }

        const data = await response.json();
        
        if (data.success) {
            originalConfigs = JSON.parse(JSON.stringify(allConfigs)); // 更新原始配置
            renderConfigs(); // 重新渲染以更新状态样式
            showToast('配置保存成功！系统配置已更新', 'success');
        } else {
            throw new Error(data.message || '保存失败');
        }
    } catch (error) {
        console.error('保存配置失败:', error);
        showToast('保存配置失败: ' + error.message, 'error');
    }
}

// 返回上一页
function goBack() {
    if (hasUnsavedChanges()) {
        if (confirm('您有未保存的更改，确定要离开吗？')) {
            window.history.back();
        }
    } else {
        window.history.back();
    }
}

// 检查是否有未保存的更改
function hasUnsavedChanges() {
    const currentKeys = Object.keys(allConfigs).sort();
    const originalKeys = Object.keys(originalConfigs).sort();
    
    if (currentKeys.length !== originalKeys.length) {
        return true;
    }
    
    for (let key of currentKeys) {
        if (!originalConfigs.hasOwnProperty(key) || originalConfigs[key] !== allConfigs[key]) {
            return true;
        }
    }
    
    return false;
}

// 显示提示消息
function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type}`;
    
    // 触发显示动画
    setTimeout(() => {
        toast.classList.add('show');
    }, 100);
    
    // 3秒后隐藏
    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

// HTML转义函数
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 页面卸载前检查未保存的更改
window.addEventListener('beforeunload', function(e) {
    if (hasUnsavedChanges()) {
        e.preventDefault();
        e.returnValue = '您有未保存的更改，确定要离开吗？';
        return e.returnValue;
    }
});

// 全局错误处理
window.addEventListener('error', function(e) {
    console.error('JavaScript错误:', e.error);
    showToast('页面发生错误: ' + e.message, 'error');
});

// 样式相关功能
function toggleTheme() {
    document.body.classList.toggle('dark-theme');
    localStorage.setItem('theme', document.body.classList.contains('dark-theme') ? 'dark' : 'light');
}

// 应用保存的主题
function applySavedTheme() {
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme === 'dark') {
        document.body.classList.add('dark-theme');
    }
}

// 页面加载时应用主题
document.addEventListener('DOMContentLoaded', applySavedTheme); 