// MCP Management Center - Modern JavaScript

let currentEditingConfig = null;
let currentConfigs = [];
let searchTimeout = null;

// Load MCP configurations
function loadMCPConfigs() {
    fetch('/api/mcp?action=list')
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                updateConfigList(data.data || []);
                updateStats(data.data || []);
            } else {
                showNotification('加载配置失败: ' + data.message, 'error');
            }
        })
        .catch(error => {
            console.error('Error loading MCP configs:', error);
            showNotification('加载配置时发生错误', 'error');
        });
}

// Update configuration list display
function updateConfigList(configs) {
    currentConfigs = configs || [];
    const configList = document.getElementById('configList');
    const emptyState = document.getElementById('emptyState');
    
    if (!configList) {
        console.error('Config list element not found');
        return;
    }

    configList.innerHTML = '';

    if (!currentConfigs || currentConfigs.length === 0) {
        configList.style.display = 'none';
        if (emptyState) emptyState.style.display = 'block';
        return;
    }

    configList.style.display = 'grid';
    if (emptyState) emptyState.style.display = 'none';

    currentConfigs.forEach(config => {
        const configElement = createConfigElement(config);
        configList.appendChild(configElement);
    });
}

// Create config element
function createConfigElement(config) {
    const div = document.createElement('div');
    div.className = `config-card ${config.enabled ? 'enabled' : 'disabled'}`;
    div.setAttribute('data-name', config.name);

    const statusClass = config.enabled ? 'active' : 'inactive';
    const statusText = config.enabled ? '启用' : '禁用';
    const toggleText = config.enabled ? '禁用' : '启用';

    // Format args display
    let argsDisplay = '';
    if (config.args && config.args.length > 0) {
        argsDisplay = config.args.map(arg => `<span class="arg-tag">${escapeHtml(arg)}</span>`).join('');
    }

    // Format environment variables display
    let envDisplay = '';
    if (config.environment && Object.keys(config.environment).length > 0) {
        const envVars = Object.entries(config.environment)
            .map(([key, value]) => `
                <div class="env-var">
                    <span class="env-key">${escapeHtml(key)}</span>
                    <span class="env-value">${escapeHtml(value)}</span>
                </div>
            `).join('');
        envDisplay = `
            <div class="config-env">
                <div class="env-label">
                    <i class="fas fa-layer-group"></i> 环境变量
                </div>
                <div class="env-vars">
                    ${envVars}
                </div>
            </div>`;
    }

    // Format dates
    const createdAt = new Date(config.created_at).toLocaleString('zh-CN');
    const updatedAt = new Date(config.updated_at).toLocaleString('zh-CN');

    div.innerHTML = `
        <div class="config-card-header">
            <div class="config-info">
                <h3 class="config-name">${escapeHtml(config.name)}</h3>
                <div class="config-status">
                    <span class="status-dot ${statusClass}"></span>
                    <span class="status-text">${config.enabled ? '运行中' : '已停止'}</span>
                </div>
            </div>
            <div class="config-menu">
                <button class="menu-btn" onclick="toggleMenu('${escapeHtml(config.name)}')">
                    <i class="fas fa-ellipsis-v"></i>
                </button>
                <div class="menu-dropdown" id="menu-${escapeHtml(config.name)}">
                    <a href="#" onclick="editConfig('${escapeHtml(config.name)}')">
                        <i class="fas fa-edit"></i> 编辑
                    </a>
                    <a href="#" onclick="toggleConfig('${escapeHtml(config.name)}')">
                        <i class="fas fa-${config.enabled ? 'pause' : 'play'}"></i> 
                        ${config.enabled ? '停止' : '启动'}
                    </a>
                    <a href="#" onclick="deleteConfig('${escapeHtml(config.name)}')" class="danger">
                        <i class="fas fa-trash"></i> 删除
                    </a>
                </div>
            </div>
        </div>
        <div class="config-card-body">
            <div class="config-command">
                <div class="command-label">
                    <i class="fas fa-terminal"></i> 命令
                </div>
                <div class="command-text">${escapeHtml(config.command)}</div>
                ${argsDisplay ? `<div class="command-args">${argsDisplay}</div>` : ''}
            </div>
            ${envDisplay}
            <div class="config-meta">
                <div class="meta-item">
                    <i class="fas fa-calendar-plus"></i>
                    <span>${createdAt}</span>
                </div>
                <div class="meta-item">
                    <i class="fas fa-calendar-check"></i>
                    <span>${updatedAt}</span>
                </div>
            </div>
        </div>
        <div class="config-card-footer">
            <div class="config-actions">
                <button class="btn-action primary" onclick="editConfig('${escapeHtml(config.name)}')">
                    <i class="fas fa-edit"></i> 编辑
                </button>
                <button class="btn-action ${config.enabled ? 'warning' : 'success'}" onclick="toggleConfig('${escapeHtml(config.name)}')">
                    <i class="fas fa-${config.enabled ? 'pause' : 'play'}"></i>
                    ${config.enabled ? '停止' : '启动'}
                </button>
                <button class="btn-action danger" onclick="deleteConfig('${escapeHtml(config.name)}')">
                    <i class="fas fa-trash"></i>
                </button>
            </div>
        </div>
    `;

    return div;
}

// Update statistics
function updateStats(configs) {
    const totalConfigs = configs ? configs.length : 0;
    const enabledConfigs = configs ? configs.filter(config => config.enabled).length : 0;
    const disabledConfigs = totalConfigs - enabledConfigs;

    const totalElement = document.getElementById('totalConfigs');
    const enabledElement = document.getElementById('enabledConfigs');
    const disabledElement = document.getElementById('disabledConfigs');

    if (totalElement) {
        totalElement.textContent = totalConfigs;
        animateNumber(totalElement, totalConfigs);
    }
    if (enabledElement) {
        enabledElement.textContent = enabledConfigs;
        animateNumber(enabledElement, enabledConfigs);
    }
    if (disabledElement) {
        disabledElement.textContent = disabledConfigs;
        animateNumber(disabledElement, disabledConfigs);
    }
}

// Animate number changes
function animateNumber(element, targetNumber) {
    const startNumber = parseInt(element.textContent) || 0;
    const duration = 500;
    const startTime = Date.now();
    
    function updateNumber() {
        const now = Date.now();
        const progress = Math.min((now - startTime) / duration, 1);
        const currentNumber = Math.round(startNumber + (targetNumber - startNumber) * progress);
        element.textContent = currentNumber;
        
        if (progress < 1) {
            requestAnimationFrame(updateNumber);
        }
    }
    
    if (startNumber !== targetNumber) {
        requestAnimationFrame(updateNumber);
    }
}

// Toggle configuration
function toggleConfig(name) {
    fetch(`/api/mcp?action=toggle&name=${encodeURIComponent(name)}`, {
        method: 'PUT'
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            showNotification(`配置 "${name}" 已${data.data.enabled ? '启用' : '禁用'}`, 'success');
            loadMCPConfigs(); // Reload to update display
        } else {
            showNotification('切换状态失败: ' + data.message, 'error');
        }
    })
    .catch(error => {
        console.error('Error toggling config:', error);
        showNotification('切换状态时发生错误', 'error');
    });
}

// Edit configuration
function editConfig(name) {
    fetch(`/api/mcp?action=get&name=${encodeURIComponent(name)}`)
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                currentEditingConfig = name;
                populateConfigForm(data.data);
                document.getElementById('modalTitle').textContent = '编辑 MCP 配置';
                openModal();
            } else {
                showNotification('获取配置失败: ' + data.message, 'error');
            }
        })
        .catch(error => {
            console.error('Error fetching config:', error);
            showNotification('获取配置时发生错误', 'error');
        });
}

// Delete configuration
function deleteConfig(name) {
    if (!confirm(`确定要删除配置 "${name}" 吗？此操作无法撤销。`)) {
        return;
    }

    fetch(`/api/mcp?name=${encodeURIComponent(name)}`, {
        method: 'DELETE'
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            showNotification(`配置 "${name}" 已删除`, 'success');
            loadMCPConfigs(); // Reload to update display
        } else {
            showNotification('删除配置失败: ' + data.message, 'error');
        }
    })
    .catch(error => {
        console.error('Error deleting config:', error);
        showNotification('删除配置时发生错误', 'error');
    });
}

// Initialize page when DOM is loaded
function initializeMCPPage() {
    console.log('Initializing MCP Management Center...');
    
    // Add config button
    const addConfigBtn = document.getElementById('addConfigBtn');
    if (addConfigBtn) {
        addConfigBtn.addEventListener('click', function() {
            currentEditingConfig = null;
            clearConfigForm();
            const modalTitle = document.getElementById('modalTitle');
            if (modalTitle) {
                modalTitle.innerHTML = '<i class="fas fa-plus-circle"></i> 新增MCP配置';
            }
            openModal();
        });
    }
    
    // Search functionality
    const searchInput = document.getElementById('searchInput');
    if (searchInput) {
        searchInput.addEventListener('input', function() {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                filterConfigs(this.value);
            }, 300);
        });
    }
    
    // View toggle functionality
    const viewButtons = document.querySelectorAll('.view-btn');
    viewButtons.forEach(btn => {
        btn.addEventListener('click', function() {
            viewButtons.forEach(b => b.classList.remove('active'));
            this.classList.add('active');
            
            const view = this.dataset.view;
            const container = document.getElementById('configsContainer');
            if (container) {
                if (view === 'list') {
                    container.classList.add('list-view');
                } else {
                    container.classList.remove('list-view');
                }
            }
        });
    });
    
    // Load MCP configurations
    loadMCPConfigs();
    
    // Close dropdowns when clicking outside
    document.addEventListener('click', function(event) {
        if (!event.target.closest('.config-menu')) {
            document.querySelectorAll('.menu-dropdown').forEach(menu => {
                menu.classList.remove('show');
            });
        }
    });
}

// Open modal
function openModal() {
    document.getElementById('configModal').style.display = 'block';
}

// Close modal
function closeModal() {
    document.getElementById('configModal').style.display = 'none';
}

// Clear configuration form
function clearConfigForm() {
    document.getElementById('configForm').reset();
    // Set default enabled state for new configurations to false (unselected)
    document.getElementById('configEnabled').checked = false;
}

// Populate configuration form with data
function populateConfigForm(config) {
    document.getElementById('configName').value = config.name || '';
    document.getElementById('configCommand').value = config.command || '';
    document.getElementById('configEnabled').checked = config.enabled || false;

    // Set args (join array with newlines)
    if (config.args && Array.isArray(config.args)) {
        document.getElementById('configArgs').value = config.args.join('\n');
    } else {
        document.getElementById('configArgs').value = '';
    }

    // Set environment variables (format as KEY=VALUE lines)
    if (config.environment && typeof config.environment === 'object') {
        const envLines = Object.entries(config.environment)
            .map(([key, value]) => `${key}=${value}`)
            .join('\n');
        document.getElementById('configEnv').value = envLines;
    } else {
        document.getElementById('configEnv').value = '';
    }
}

// Save configuration (add or update)
function saveConfig() {
    const form = document.getElementById('configForm');
    const formData = new FormData(form);

    // Parse args (split by newlines, filter empty)
    const argsText = formData.get('args') || '';
    const args = argsText.split('\n')
        .map(line => line.trim())
        .filter(line => line.length > 0);

    // Parse environment variables
    const envText = formData.get('environment') || '';
    const environment = {};
    envText.split('\n')
        .map(line => line.trim())
        .filter(line => line.length > 0 && line.includes('='))
        .forEach(line => {
            const [key, ...valueParts] = line.split('=');
            const value = valueParts.join('=');
            if (key.trim() && value.trim()) {
                environment[key.trim()] = value.trim();
            }
        });

    const config = {
        name: formData.get('name'),
        command: formData.get('command'),
        args: args,
        environment: environment,
        enabled: formData.get('enabled') === 'on'
    };

    // Validate required fields
    if (!config.name || !config.command) {
        showNotification('请填写配置名称和命令', 'error');
        return;
    }

    const isEditing = currentEditingConfig !== null;
    const url = isEditing ? `/api/mcp?name=${encodeURIComponent(currentEditingConfig)}` : '/api/mcp';
    const method = isEditing ? 'PUT' : 'POST';

    fetch(url, {
        method: method,
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(config)
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            showNotification(`配置 "${config.name}" ${isEditing ? '更新' : '添加'}成功`, 'success');
            closeModal();
            loadMCPConfigs(); // Reload to update display
        } else {
            showNotification(`${isEditing ? '更新' : '添加'}配置失败: ` + data.message, 'error');
        }
    })
    .catch(error => {
        console.error('Error saving config:', error);
        showNotification(`${isEditing ? '更新' : '添加'}配置时发生错误`, 'error');
    });
}

// Show notification
function showNotification(message, type = 'info') {
    const container = document.getElementById('notifications') || document.body;
    
    // Create notification element
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    
    // Add icon based on type
    const icons = {
        success: 'fas fa-check-circle',
        error: 'fas fa-exclamation-circle', 
        warning: 'fas fa-exclamation-triangle',
        info: 'fas fa-info-circle'
    };
    
    notification.innerHTML = `
        <div style="display: flex; align-items: center; gap: 8px;">
            <i class="${icons[type] || icons.info}"></i>
            <span>${message}</span>
        </div>
    `;

    // Add to container
    container.appendChild(notification);

    // Auto remove after 4 seconds
    setTimeout(() => {
        if (notification.parentNode) {
            notification.style.animation = 'notificationSlideOut 0.3s ease forwards';
            setTimeout(() => {
                if (notification.parentNode) {
                    notification.parentNode.removeChild(notification);
                }
            }, 300);
        }
    }, 4000);
}

// Search and filter configurations
function filterConfigs(searchTerm) {
    if (!currentConfigs) return;
    
    const filtered = currentConfigs.filter(config => {
        return config.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
               config.command.toLowerCase().includes(searchTerm.toLowerCase());
    });
    
    updateConfigList(filtered);
}

// Toggle menu dropdown
function toggleMenu(configName) {
    const menu = document.getElementById(`menu-${configName}`);
    if (menu) {
        // Close other menus
        document.querySelectorAll('.menu-dropdown').forEach(m => {
            if (m !== menu) m.classList.remove('show');
        });
        
        // Toggle current menu
        menu.classList.toggle('show');
    }
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Close modal when clicking outside
window.addEventListener('click', function(event) {
    const modal = document.getElementById('configModal');
    if (event.target === modal) {
        closeModal();
    }
});

// Close modal with Escape key
document.addEventListener('keydown', function(event) {
    if (event.key === 'Escape') {
        closeModal();
    }
});