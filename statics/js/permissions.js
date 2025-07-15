// 权限管理相关JavaScript功能

document.addEventListener('DOMContentLoaded', function() {
    initPermissionControls();
});

// Toast 通知函数
function showToast(message, type = 'info', duration = 3000) {
    // 创建 toast 容器（如果不存在）
    let toastContainer = document.getElementById('toast-container');
    if (!toastContainer) {
        toastContainer = document.createElement('div');
        toastContainer.id = 'toast-container';
        toastContainer.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 9999;
            pointer-events: none;
        `;
        document.body.appendChild(toastContainer);
    }
    
    // 创建 toast 元素
    const toast = document.createElement('div');
    toast.style.cssText = `
        background: ${type === 'error' ? '#f44336' : type === 'success' ? '#4CAF50' : '#2196F3'};
        color: white;
        padding: 12px 20px;
        border-radius: 6px;
        margin-bottom: 10px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.3);
        transform: translateX(100%);
        transition: transform 0.3s ease-in-out;
        pointer-events: auto;
        max-width: 300px;
        word-wrap: break-word;
        font-size: 14px;
        line-height: 1.4;
    `;
    toast.textContent = message;
    
    toastContainer.appendChild(toast);
    
    // 动画显示
    setTimeout(() => {
        toast.style.transform = 'translateX(0)';
    }, 10);
    
    // 自动隐藏
    setTimeout(() => {
        toast.style.transform = 'translateX(100%)';
        setTimeout(() => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        }, 300);
    }, duration);
}

function initPermissionControls() {
    const encryptCheckbox = document.getElementById('encrypt_permission');
    const encryptInput = document.getElementById('encrypt');
    const diaryCheckbox = document.getElementById('diary_permission');
    const cooperationCheckbox = document.getElementById('cooperation_permission');
    
    // 加密权限与密码输入框联动
    if (encryptCheckbox && encryptInput) {
        // 初始状态设置
        updateEncryptInputState();
        
        // 监听加密权限变化
        encryptCheckbox.addEventListener('change', function() {
            updateEncryptInputState();
            
            // 如果启用加密但没有密码，则聚焦到密码输入框
            if (this.checked && !encryptInput.value.trim()) {
                // 滚动到密码输入框
                encryptInput.scrollIntoView({ behavior: 'smooth', block: 'center' });
                
                // 延迟聚焦，确保滚动完成
                setTimeout(() => {
                    encryptInput.focus();
                    // 添加视觉提示
                    encryptInput.style.animation = 'passwordHighlight 2.5s ease-in-out';
                }, 300);
                
                showToast('🔐 内容加密已启用！请在下方密码区域设置加密密码', 'info');
            } else if (this.checked) {
                showToast('🔐 内容加密已启用！', 'success');
            }
        });
        
        // 监听密码输入框变化
        encryptInput.addEventListener('input', function() {
            // 如果输入了密码但没有启用加密权限，自动启用
            if (this.value.trim() && !encryptCheckbox.checked) {
                encryptCheckbox.checked = true;
                updateEncryptInputState();
                showToast('已自动启用内容加密', 'info');
            }
        });
    }
    
    // 日记权限提示
    if (diaryCheckbox) {
        diaryCheckbox.addEventListener('change', function() {
            if (this.checked) {
                showToast('📔 日记权限已启用！访问需要额外密码验证', 'info');
                
                // 显示简化说明
                setTimeout(() => {
                    showCustomConfirm(
                        '📔 日记权限说明',
                        '访问此博客需要输入系统日记密码\n默认密码：diary123',
                        '查看配置方法',
                        '知道了',
                        () => showDiaryPasswordHelp()
                    );
                }, 500);
            }
        });
    }
    
    // 协作权限提示
    if (cooperationCheckbox) {
        cooperationCheckbox.addEventListener('change', function() {
            if (this.checked) {
                showToast('🤝 协作权限已启用！协作用户可以访问此博客', 'info');
                
                // 显示协作权限说明
                setTimeout(() => {
                    showCustomConfirm(
                        '🤝 协作权限说明',
                        '协作用户可以访问此博客\n协作用户需要在系统中配置',
                        '查看配置方法',
                        '知道了',
                        () => showCooperationHelp()
                    );
                }, 500);
            }
        });
    }
    
    // 基础权限切换提示
    const baseAuthRadios = document.querySelectorAll('input[name="base_auth_type"]');
    baseAuthRadios.forEach(radio => {
        radio.addEventListener('change', function() {
            const permissionName = this.value === 'public' ? '公开' : '私有';
            showToast(`已切换到${permissionName}权限`, 'info');
        });
    });
}

function updateEncryptInputState() {
    const encryptCheckbox = document.getElementById('encrypt_permission');
    const encryptInput = document.getElementById('encrypt');
    const encryptSection = document.getElementById('encrypt-section');
    const encryptLabel = document.getElementById('encrypt-label');
    const encryptHint = document.getElementById('encrypt-hint');
    
    if (encryptCheckbox && encryptInput) {
        if (encryptCheckbox.checked) {
            // 启用加密时的样式
            encryptInput.style.borderColor = '#4CAF50';
            encryptInput.style.backgroundColor = 'rgba(76, 175, 80, 0.1)';
            encryptInput.placeholder = '🔐 请输入加密密码（必填）';
            encryptInput.required = true;
            
            if (encryptSection) {
                encryptSection.style.backgroundColor = 'rgba(76, 175, 80, 0.05)';
                encryptSection.style.border = '1px solid rgba(76, 175, 80, 0.3)';
                encryptSection.style.borderRadius = '6px';
                encryptSection.style.padding = '10px';
            }
            
            if (encryptLabel) {
                encryptLabel.textContent = '🔐 加密密码 (必填)';
                encryptLabel.style.color = '#4CAF50';
                encryptLabel.style.fontWeight = 'bold';
            }
            
            if (encryptHint) {
                encryptHint.textContent = '✅ 内容加密已启用 - 请设置一个安全的密码';
                encryptHint.style.color = '#4CAF50';
            }
        } else {
            // 未启用加密时的样式
            encryptInput.style.borderColor = '';
            encryptInput.style.backgroundColor = '';
            encryptInput.placeholder = '输入加密密码...';
            encryptInput.required = false;
            
            if (encryptSection) {
                encryptSection.style.backgroundColor = '';
                encryptSection.style.border = '';
                encryptSection.style.borderRadius = '';
                encryptSection.style.padding = '';
            }
            
            if (encryptLabel) {
                encryptLabel.textContent = '🔐 加密密码';
                encryptLabel.style.color = '';
                encryptLabel.style.fontWeight = '';
            }
            
            if (encryptHint) {
                encryptHint.textContent = '💡 启用"内容加密"权限时必须设置密码';
                encryptHint.style.color = '#888';
            }
        }
    }
}

// 获取当前权限设置摘要
function getPermissionSummary() {
    const baseAuth = document.querySelector('input[name="base_auth_type"]:checked');
    const diary = document.getElementById('diary_permission');
    const cooperation = document.getElementById('cooperation_permission');
    const encrypt = document.getElementById('encrypt_permission');
    
    let summary = [];
    
    if (baseAuth) {
        summary.push(baseAuth.value === 'public' ? '🌐 公开' : '🔒 私有');
    }
    
    if (diary && diary.checked) {
        summary.push('📔 日记');
    }
    
    if (cooperation && cooperation.checked) {
        summary.push('🤝 协作');
    }
    
    if (encrypt && encrypt.checked) {
        summary.push('🔐 加密');
    }
    
    return summary.join(' + ');
}

// 验证权限设置
function validatePermissions() {
    const encrypt = document.getElementById('encrypt_permission');
    
    // 检查加密设置
    if (encrypt && encrypt.checked) {
        // 检查加密设置密码输入框（创建页面和编辑页面）
        const encryptPasswordInput = document.getElementById('encrypt-password'); // 编辑页面
        const encryptInput = document.getElementById('encrypt');                 // 创建页面
        
        let passwordInput = null;
        let passwordValue = '';
        
        if (encryptPasswordInput) {
            // 编辑页面：使用专门的加密设置输入框
            passwordInput = encryptPasswordInput;
            passwordValue = encryptPasswordInput.value;
        } else if (encryptInput) {
            // 创建页面：使用加密密码输入框
            passwordInput = encryptInput;
            passwordValue = encryptInput.value;
        }
        
        // 检查是否已经是加密博客（通过解密密码输入框的存在判断）
        const decryptInput = document.getElementById('decrypt-password');
        const isAlreadyEncrypted = decryptInput !== null;
        
        // 如果不是已加密博客，则必须设置密码
        if (!isAlreadyEncrypted && (!passwordInput || !passwordValue.trim())) {
            showToast('启用内容加密时必须设置加密密码', 'error');
            if (passwordInput) passwordInput.focus();
            return false;
        }
        
        // 如果设置了密码，验证长度
        if (passwordValue && passwordValue.length < 6) {
            showToast('加密密码长度至少需要6个字符', 'error');
            passwordInput.focus();
            return false;
        }
        

    }
    
    return true;
}

// 显示权限设置帮助
function showPermissionHelp() {
    const helpText = `
博客权限设置说明：

🔒 私有权限：只有登录用户可以访问
🌐 公开权限：所有用户都可以访问
📔 日记权限：需要额外密码验证
🤝 协作权限：允许协作用户访问
🔐 内容加密：使用AES加密保护内容

权限可以组合使用，例如：
• 公开 + 加密：所有人可以看到博客，但需要密码解密内容
• 私有 + 协作：只有登录用户和授权协作用户可以访问
• 私有 + 日记：登录用户访问，但还需要额外的日记密码验证

注意事项：
• 日记权限使用系统配置的密码，不是博客个人密码
• 加密权限使用博客个人设置的密码
• 权限设置保存后立即生效
    `;
    
    alert(helpText);
}

// 自定义确认弹框
function showCustomConfirm(title, message, confirmText, cancelText, onConfirm) {
    // 创建遮罩层
    const overlay = document.createElement('div');
    overlay.className = 'custom-modal-overlay';
    overlay.style.cssText = `
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.5);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 10000;
        backdrop-filter: blur(2px);
    `;
    
    // 创建弹框
    const modal = document.createElement('div');
    modal.className = 'custom-modal';
    modal.style.cssText = `
        background: white;
        border-radius: 12px;
        padding: 24px;
        max-width: 400px;
        width: 90%;
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
        text-align: center;
        animation: modalSlideIn 0.3s ease-out;
    `;
    
    // 添加动画样式
    const style = document.createElement('style');
    style.textContent = `
        @keyframes modalSlideIn {
            from {
                transform: translateY(-20px);
                opacity: 0;
            }
            to {
                transform: translateY(0);
                opacity: 1;
            }
        }
    `;
    document.head.appendChild(style);
    
    // 创建标题
    const titleEl = document.createElement('h3');
    titleEl.textContent = title;
    titleEl.style.cssText = `
        margin: 0 0 16px 0;
        font-size: 18px;
        color: #433520;
        font-weight: 600;
    `;
    
    // 创建消息内容
    const messageEl = document.createElement('p');
    messageEl.textContent = message;
    messageEl.style.cssText = `
        margin: 0 0 24px 0;
        line-height: 1.6;
        color: #666;
        white-space: pre-line;
    `;
    
    // 创建按钮容器
    const buttonContainer = document.createElement('div');
    buttonContainer.style.cssText = `
        display: flex;
        gap: 12px;
        justify-content: center;
    `;
    
    // 创建确认按钮
    const confirmBtn = document.createElement('button');
    confirmBtn.textContent = confirmText;
    confirmBtn.style.cssText = `
        padding: 10px 20px;
        border: 2px solid #e76f51;
        background: #e76f51;
        color: white;
        border-radius: 6px;
        cursor: pointer;
        font-size: 14px;
        transition: all 0.3s ease;
    `;
    
    // 创建取消按钮（如果需要）
    let cancelBtn = null;
    if (cancelText) {
        cancelBtn = document.createElement('button');
        cancelBtn.textContent = cancelText;
        cancelBtn.style.cssText = `
            padding: 10px 20px;
            border: 2px solid #ddd0c0;
            background: white;
            color: #433520;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
            transition: all 0.3s ease;
        `;
        
        // 添加悬停效果
        cancelBtn.addEventListener('mouseenter', () => {
            cancelBtn.style.background = '#f5f5f5';
        });
        cancelBtn.addEventListener('mouseleave', () => {
            cancelBtn.style.background = 'white';
        });
        
        // 添加点击事件
        cancelBtn.addEventListener('click', () => {
            document.body.removeChild(overlay);
        });
    }
    
    confirmBtn.addEventListener('mouseenter', () => {
        confirmBtn.style.background = '#f4a261';
        confirmBtn.style.borderColor = '#f4a261';
    });
    confirmBtn.addEventListener('mouseleave', () => {
        confirmBtn.style.background = '#e76f51';
        confirmBtn.style.borderColor = '#e76f51';
    });
    
    confirmBtn.addEventListener('click', () => {
        document.body.removeChild(overlay);
        if (onConfirm) onConfirm();
    });
    
    // 点击遮罩层关闭
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) {
            document.body.removeChild(overlay);
        }
    });
    
    // 组装元素
    if (cancelBtn) {
        buttonContainer.appendChild(cancelBtn);
    }
    buttonContainer.appendChild(confirmBtn);
    modal.appendChild(titleEl);
    modal.appendChild(messageEl);
    modal.appendChild(buttonContainer);
    overlay.appendChild(modal);
    
    // 显示弹框
    document.body.appendChild(overlay);
    
    // 聚焦到确认按钮
    setTimeout(() => confirmBtn.focus(), 100);
}

// 自定义提示弹框
function showCustomAlert(title, message, buttonText = '确定') {
    showCustomConfirm(title, message, buttonText, null, null);
}

// 显示日记权限配置帮助
function showDiaryPasswordHelp() {
    const helpText = `💡 配置方法：
在配置文件中添加：diary_password=你的安全密码

🔑 默认密码：diary123

📝 权限效果：
• 访问日记博客需要输入此密码
• 可与其他权限组合使用
• 独立于内容加密功能

🛡️ 安全建议：
设置强密码并妥善保管`;
    
    showCustomAlert('📔 日记密码配置指南', helpText);
}

// 显示协作权限配置帮助
function showCooperationHelp() {
    const helpText = `👥 协作用户管理：
协作用户需要在系统配置中添加

🔧 配置方法：
1. 访问系统配置页面
2. 添加协作用户账号和密码
3. 为协作用户指定可访问的博客

⚙️ 权限说明：
• 协作用户可以访问标记为"协作"的博客
• 需要单独的登录认证
• 可与其他权限组合使用

💡 使用场景：
适合团队协作或特定用户分享`;
    
    showCustomAlert('🤝 协作权限配置指南', helpText);
}

// 导出函数供其他脚本使用
window.PermissionManager = {
    validate: validatePermissions,
    getSummary: getPermissionSummary,
    showHelp: showPermissionHelp,
    showDiaryHelp: showDiaryPasswordHelp
}; 