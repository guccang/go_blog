// 配置管理页面JavaScript

let allConfigs = {};
let configComments = {};
let originalConfigs = {};
let originalComments = {};
let filteredConfigs = {};

// 配置项元数据：分类、描述、排序
const CONFIG_METADATA = {
    // ─── 基础设置 ───
    port:           { category: '基础设置', icon: '🌐', order: 1, desc: 'HTTP 服务监听端口号，修改后需重启生效' },
    pwd:            { category: '基础设置', icon: '🌐', order: 2, desc: '管理员登录密码' },
    admin:          { category: '基础设置', icon: '🌐', order: 3, desc: '管理员账户名称' },
    logs_dir:       { category: '基础设置', icon: '🌐', order: 4, desc: '日志文件存放目录路径' },
    statics_path:   { category: '基础设置', icon: '🌐', order: 5, desc: '静态资源（CSS/JS/图片）目录路径' },
    templates_path: { category: '基础设置', icon: '🌐', order: 6, desc: 'HTML 模板文件目录路径' },
    download_path:  { category: '基础设置', icon: '🌐', order: 7, desc: '文件下载保存目录路径' },
    recycle_path:   { category: '基础设置', icon: '🌐', order: 8, desc: '删除博客的回收站目录路径' },

    // ─── Redis 配置 ───
    redis_ip:   { category: 'Redis 缓存', icon: '🗄️', order: 1, desc: 'Redis 服务器 IP 地址，默认 127.0.0.1' },
    redis_port: { category: 'Redis 缓存', icon: '🗄️', order: 2, desc: 'Redis 服务器端口号，默认 6666' },
    redis_pwd:  { category: 'Redis 缓存', icon: '🗄️', order: 3, desc: 'Redis 连接密码，留空表示无密码' },

    // ─── 博客设置 ───
    publictags:       { category: '博客设置', icon: '📝', order: 1, desc: '公开可见的标签列表，多个用 | 分隔（如 public|share|demo）' },
    sysfiles:         { category: '博客设置', icon: '📝', order: 2, desc: '系统文件名列表，这些文件不会在博客列表中显示' },
    main_show_blogs:  { category: '博客设置', icon: '📝', order: 3, desc: '主页默认显示的博客数量' },
    max_blog_comments:{ category: '博客设置', icon: '📝', order: 4, desc: '每篇博客允许的最大评论数' },
    share_days:       { category: '博客设置', icon: '📝', order: 5, desc: '分享链接的有效天数' },
    help_blog_name:   { category: '博客设置', icon: '📝', order: 6, desc: '帮助文档对应的博客标题名称' },

    // ─── 日记设置 ───
    title_auto_add_date_suffix: { category: '日记设置', icon: '📔', order: 1, desc: '标题包含该关键字时自动添加日期后缀，多个用 | 分隔' },
    diary_keywords:             { category: '日记设置', icon: '📔', order: 2, desc: '日记识别关键字，标题含此前缀会被标记为日记，多个用 | 分隔' },
    diary_password:             { category: '日记设置', icon: '📔', order: 3, desc: '日记加密密码，设置后日记内容需输入密码才能查看' },

    // ─── AI / LLM 配置 ───
    openai_api_key:           { category: 'AI / LLM', icon: '🤖', order: 1, desc: 'OpenAI API 密钥（用于智能助手和 Agent）' },
    openai_api_url:           { category: 'AI / LLM', icon: '🤖', order: 2, desc: 'OpenAI API 请求地址，可配置代理或自部署端点' },
    deepseek_api_key:         { category: 'AI / LLM', icon: '🤖', order: 3, desc: 'DeepSeek API 密钥' },
    deepseek_api_url:         { category: 'AI / LLM', icon: '🤖', order: 4, desc: 'DeepSeek API 请求地址' },
    qwen_api_key:             { category: 'AI / LLM', icon: '🤖', order: 5, desc: '通义千问(Qwen) API 密钥' },
    qwen_api_url:             { category: 'AI / LLM', icon: '🤖', order: 6, desc: '通义千问(Qwen) API 请求地址' },
    llm_fallback_models:      { category: 'AI / LLM', icon: '🤖', order: 7, desc: 'LLM 备用模型配置（JSON 格式），主模型失败时自动切换' },
    assistant_save_mcp_result: { category: 'AI / LLM', icon: '🤖', order: 8, desc: '是否保存 MCP 工具调用结果到博客（true/false）' },

    // ─── CodeGen 编码助手 ───
    codegen_workspace:    { category: 'CodeGen 编码', icon: '💻', order: 1, desc: '编码项目工作区目录，多个用逗号分隔，默认 ./codegen' },
    codegen_max_turns:    { category: 'CodeGen 编码', icon: '💻', order: 2, desc: 'Claude 单次会话最大交互轮数，默认 20' },
    codegen_agent_token:  { category: 'CodeGen 编码', icon: '💻', order: 3, desc: '远程 CodeGen Agent 认证 Token' },

    // ─── 企业微信 ───
    wechat_corp_id:          { category: '企业微信', icon: '💬', order: 1, desc: '企业微信 Corp ID（企业ID）' },
    wechat_secret:           { category: '企业微信', icon: '💬', order: 2, desc: '企业微信应用 Secret' },
    wechat_agent_id:         { category: '企业微信', icon: '💬', order: 3, desc: '企业微信应用 Agent ID' },
    wechat_token:            { category: '企业微信', icon: '💬', order: 4, desc: '企业微信回调 Token（用于验证消息来源）' },
    wechat_encoding_aes_key: { category: '企业微信', icon: '💬', order: 5, desc: '企业微信消息加密 AES Key（43位字符）' },
    wechat_webhook:          { category: '企业微信', icon: '💬', order: 6, desc: '企业微信群机器人 Webhook 地址' },

    // ─── 邮件 / 通知 ───
    smtp_host:      { category: '邮件通知', icon: '📧', order: 1, desc: 'SMTP 邮件服务器地址（如 smtp.qq.com）' },
    smtp_port:      { category: '邮件通知', icon: '📧', order: 2, desc: 'SMTP 服务器端口号（如 465 或 587）' },
    email_from:     { category: '邮件通知', icon: '📧', order: 3, desc: '发件人邮箱地址' },
    email_password: { category: '邮件通知', icon: '📧', order: 4, desc: '发件人邮箱密码或授权码' },
    email_to:       { category: '邮件通知', icon: '📧', order: 5, desc: '默认收件人邮箱地址' },
    sms_phone:      { category: '邮件通知', icon: '📧', order: 6, desc: '短信通知接收手机号' },
    sms_send_url:   { category: '邮件通知', icon: '📧', order: 7, desc: '短信发送接口 URL' },
};

// 分类显示顺序
const CATEGORY_ORDER = [
    '基础设置', 'Redis 缓存', '博客设置', '日记设置',
    'AI / LLM', 'CodeGen 编码', '企业微信', '邮件通知'
];

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
            configComments = data.comments || {};
            originalConfigs = JSON.parse(JSON.stringify(allConfigs)); // 深拷贝
            originalComments = JSON.parse(JSON.stringify(configComments)); // 深拷贝
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

// 渲染配置列表（按分类分组）
function renderConfigs() {
    const configList = document.getElementById('configList');
    configList.innerHTML = '';

    const keys = Object.keys(filteredConfigs);

    if (keys.length === 0) {
        configList.innerHTML = `
            <div class="empty-state">
                <h3>没有找到配置项</h3>
                <p>您可以添加新的配置项或检查搜索条件</p>
                <button class="btn btn-primary" onclick="addNewConfig()">添加配置</button>
            </div>
        `;
        return;
    }

    // 按分类分组
    const groups = {};
    const ungrouped = [];

    keys.forEach(key => {
        const meta = CONFIG_METADATA[key];
        if (meta) {
            const cat = meta.category;
            if (!groups[cat]) groups[cat] = [];
            groups[cat].push(key);
        } else {
            ungrouped.push(key);
        }
    });

    // 每个分类内按 order 排序
    Object.keys(groups).forEach(cat => {
        groups[cat].sort((a, b) => {
            const oa = (CONFIG_METADATA[a] || {}).order || 999;
            const ob = (CONFIG_METADATA[b] || {}).order || 999;
            return oa - ob;
        });
    });

    // 按分类顺序渲染
    CATEGORY_ORDER.forEach(cat => {
        if (!groups[cat] || groups[cat].length === 0) return;
        const meta0 = CONFIG_METADATA[groups[cat][0]];
        const icon = meta0 ? meta0.icon : '📦';

        const section = document.createElement('div');
        section.className = 'config-category';
        section.innerHTML = `<div class="category-header" onclick="toggleCategory(this)">
            <span class="category-icon">${icon}</span>
            <span class="category-title">${cat}</span>
            <span class="category-count">${groups[cat].length} 项</span>
            <span class="category-toggle">▼</span>
        </div>`;

        const body = document.createElement('div');
        body.className = 'category-body';

        groups[cat].forEach(key => {
            body.appendChild(createConfigItem(key, filteredConfigs[key]));
        });

        section.appendChild(body);
        configList.appendChild(section);
    });

    // 未分类的配置项
    if (ungrouped.length > 0) {
        ungrouped.sort();
        const section = document.createElement('div');
        section.className = 'config-category';
        section.innerHTML = `<div class="category-header" onclick="toggleCategory(this)">
            <span class="category-icon">📦</span>
            <span class="category-title">其他配置</span>
            <span class="category-count">${ungrouped.length} 项</span>
            <span class="category-toggle">▼</span>
        </div>`;

        const body = document.createElement('div');
        body.className = 'category-body';

        ungrouped.forEach(key => {
            body.appendChild(createConfigItem(key, filteredConfigs[key]));
        });

        section.appendChild(body);
        configList.appendChild(section);
    }
}

// 折叠/展开分类
function toggleCategory(header) {
    const section = header.parentElement;
    section.classList.toggle('collapsed');
    const toggle = header.querySelector('.category-toggle');
    toggle.textContent = section.classList.contains('collapsed') ? '▶' : '▼';
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

    const comment = configComments[key] || '';
    const meta = CONFIG_METADATA[key];
    const description = meta ? meta.desc : '';

    item.innerHTML = `
        <div class="config-key">
            <div class="config-key-label">配置项名称</div>
            <input type="text" class="config-key-input" value="${escapeHtml(key)}"
                   onchange="updateConfigKey('${escapeHtml(key)}', this.value)"
                   title="${escapeHtml(description || key)}">
            ${description ? `<div class="config-desc">${escapeHtml(description)}</div>` : ''}
        </div>
        <div class="config-value">
            <div class="config-value-label">配置值</div>
            <input type="text" class="config-value-input" value="${escapeHtml(value)}"
                   onchange="updateConfigValue('${escapeHtml(key)}', this.value)"
                   title="配置项的值">
            <div class="config-type-hint">${getConfigTypeHint(value)}</div>
        </div>
        <div class="config-comment">
            <div class="config-comment-label">注释说明</div>
            <textarea class="config-comment-input"
                      onchange="updateConfigComment('${escapeHtml(key)}', this.value)"
                      placeholder="配置项注释说明">${escapeHtml(comment)}</textarea>
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
    
    // 更新配置和注释
    const value = allConfigs[oldKey];
    const comment = configComments[oldKey] || '';
    delete allConfigs[oldKey];
    delete configComments[oldKey];
    allConfigs[newKey] = value;
    configComments[newKey] = comment;
    
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
    updateConfigItemStatus(key);
}

// 更新配置项注释
function updateConfigComment(key, comment) {
    configComments[key] = comment;
    updateRawPreview();
    updateConfigItemStatus(key);
}

// 更新配置项状态样式
function updateConfigItemStatus(key) {
    const configItems = document.querySelectorAll('.config-item');
    configItems.forEach(item => {
        if (item.dataset.originalKey === key) {
            item.classList.remove('new', 'modified');
            const isNewConfig = !originalConfigs.hasOwnProperty(key);
            const isValueChanged = originalConfigs[key] !== allConfigs[key];
            const isCommentChanged = (originalComments[key] || '') !== (configComments[key] || '');
            
            if (isNewConfig) {
                item.classList.add('new');
            } else if (isValueChanged || isCommentChanged) {
                item.classList.add('modified');
            }
        }
    });
}

// 重置配置项
function resetConfig(key) {
    if (originalConfigs.hasOwnProperty(key)) {
        allConfigs[key] = originalConfigs[key];
        configComments[key] = originalComments[key] || '';
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
        delete configComments[key];
        delete filteredConfigs[key];
        renderConfigs();
        updateRawPreview();
        updateConfigCount();
        showToast('配置项已删除', 'success');
    }
}

// 过滤配置项（支持按名称、值、描述搜索）
function filterConfigs() {
    const searchText = document.getElementById('searchInput').value.toLowerCase();

    if (searchText === '') {
        filteredConfigs = JSON.parse(JSON.stringify(allConfigs));
    } else {
        filteredConfigs = {};
        Object.keys(allConfigs).forEach(key => {
            const value = allConfigs[key];
            const meta = CONFIG_METADATA[key];
            const desc = meta ? meta.desc : '';
            const cat = meta ? meta.category : '';
            if (key.toLowerCase().includes(searchText) ||
                value.toLowerCase().includes(searchText) ||
                desc.toLowerCase().includes(searchText) ||
                cat.toLowerCase().includes(searchText)) {
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
        const comment = configComments[key];
        if (key && value !== undefined) {
            if (comment && comment.trim()) {
                lines.push(`# ${comment}`);
            }
            lines.push(`${key}=${value}`);
            lines.push('');
        }
    });
    
    preview.value = lines.join('\n');
}

// 添加新配置
function addNewConfig() {
    document.getElementById('newConfigKey').value = '';
    document.getElementById('newConfigValue').value = '';
    document.getElementById('newConfigComment').value = '';
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
    const comment = document.getElementById('newConfigComment').value.trim();
    
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
    
    // 添加配置和注释
    allConfigs[key] = value;
    configComments[key] = comment;
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
                configs: allConfigs,
                comments: configComments
            })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`HTTP ${response.status}: ${errorText}`);
        }

        const data = await response.json();
        
        if (data.success) {
            originalConfigs = JSON.parse(JSON.stringify(allConfigs)); // 更新原始配置
            originalComments = JSON.parse(JSON.stringify(configComments)); // 更新原始注释
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
        const valueChanged = !originalConfigs.hasOwnProperty(key) || originalConfigs[key] !== allConfigs[key];
        const commentChanged = (originalComments[key] || '') !== (configComments[key] || '');
        if (valueChanged || commentChanged) {
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