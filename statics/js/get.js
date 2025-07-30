// Initialize Vim
vim.open({
	//debug : true
});

// DOM Elements
const sidebar = document.getElementById('sidebar-container');
const bubble = document.getElementById('bubble');
const container = document.querySelector('.container');
const editor = document.getElementById('editor-inner');
const md = document.getElementById('md');
const toastContainer = document.getElementById('toast-container');

// 初始化编辑页面权限控制
document.addEventListener('DOMContentLoaded', function() {
    if (typeof initPermissionControls === 'function') {
        initPermissionControls();
    }
    initEditPagePermissions();
});

// Toggle sidebar
bubble.addEventListener('click', function() {
		sidebar.classList.toggle('hide-sidebar');
		container.classList.toggle('hide-sidebar');
});

// Function to show toast notifications
function showToast(message, type = 'info') {
	const toast = document.createElement('div');
	toast.className = `toast ${type}`;
	toast.innerHTML = `<span class="toast-message">${message}</span>`;
	toastContainer.appendChild(toast);
	
	// Remove toast after 4 seconds
	setTimeout(() => {
		toast.remove();
	}, 4000);
}

PageHistoryBack();

function onDecrypt() {
	const e = document.getElementById('editor-inner');
	const decryptInput = document.getElementById('decrypt-password');
	const k = decryptInput ? decryptInput.value : '';
	
	if (!k) {
		showToast('请输入解密密码', 'error');
		if (decryptInput) decryptInput.focus();
		return;
	}
	
	try {
		const content = e.innerHTML;
		
		// 检查内容是否为空
		if (!content || content.trim() === '') {
			showToast('博客内容为空', 'warning');
			return;
		}
		
		const txt = aesDecrypt(content, k);
		e.innerHTML = txt;
		mdRender(txt);
		showToast('解密成功', 'success');
		
		// 解密成功后，将解密密码复制到加密设置框作为默认值
		const encryptPasswordInput = document.getElementById('encrypt-password');
		if (encryptPasswordInput && !encryptPasswordInput.value) {
			encryptPasswordInput.value = k;
		}
	} catch (error) {
		showToast('解密失败，密码可能不正确', 'error');
		if (decryptInput) decryptInput.focus();
	}
}

function onShowComment() {
	const btn = document.getElementById('comment-show');
	const toggleText = document.getElementById('toggle-text');
	const toggleIcon = document.getElementById('toggle-icon');
	const commentsContainer = document.getElementById('comments-container');
	
	if (commentsContainer.classList.contains('hide')) {
		// 显示评论
		commentsContainer.classList.remove('hide');
		toggleText.textContent = '收起评论';
		toggleIcon.textContent = '▲';
		btn.classList.add('expanded');
	} else {
		// 隐藏评论
		commentsContainer.classList.add('hide');
		toggleText.textContent = '显示评论';
		toggleIcon.textContent = '▼';
		btn.classList.remove('expanded');
	}
}

function onDelete() {
	if (confirm('确定要删除此博客吗？此操作不可撤销。')) {
		const title = document.getElementById('title').innerText;
		
		const xhr = new XMLHttpRequest();
		xhr.onreadystatechange = function() {
			if (xhr.readyState == 4) {
				if (xhr.status == 200) {
					showToast('删除成功', 'success');
					setTimeout(() => {
						window.location.href = '/main';
					}, 1500);
				} else {
					showToast('删除失败: ' + xhr.responseText, 'error');
				}
			}
		};
		
		const formData = new FormData();
		formData.append('title', title);
		xhr.open('POST', '/delete', true);
		xhr.send(formData);
	}
}

function onCommitComment() {
	const title = document.getElementById('title').innerText;
	const comment = document.getElementById('input-comment').value;
	const owner = document.getElementById('input-owner').value;
	const mail = document.getElementById('input-mail').value;
	const password = document.getElementById('input-pwd').value;
	
	// Validate form
	if (!owner.trim() || !comment.trim()) {
		showToast('请填写用户名和评论内容', 'error');
		return;
	}
	
	// Check character limit
	if (comment.length > 500) {
		showToast('评论内容不能超过500个字符', 'error');
		return;
	}
	
	// 检查用户名是否已被注册，如果是则必须提供密码
	if (window.currentUsernameStatus && window.currentUsernameStatus.user_count > 0 && !password.trim()) {
		showToast('该用户名已被注册，请输入密码进行身份验证', 'error');
		return;
	}
	
	// Disable submit button to prevent double submission
	const submitBtn = document.getElementById('commit-comment');
	const originalText = submitBtn.innerHTML;
	submitBtn.disabled = true;
	submitBtn.innerHTML = '<span class="btn-icon">⏳</span><span class="btn-text">提交中...</span>';
	
	// Check if username is available and submit comment
	checkUsernameAndSubmit(title, comment, owner, mail, submitBtn, originalText);
}

// 检查用户名可用性并提交评论
function checkUsernameAndSubmit(title, comment, owner, mail, submitBtn, originalText) {
	// 获取现有会话ID（如果有）
	const sessionID = getCommentSessionID(owner);
	
	if (sessionID) {
		// 使用现有会话提交评论
		submitCommentWithSession(title, comment, sessionID, submitBtn, originalText);
	} else {
		// 获取密码（如果用户填写了）
		const password = document.getElementById('input-pwd').value;
		
		// 使用密码验证机制提交评论
		submitCommentWithPassword(title, comment, owner, mail, password, submitBtn, originalText);
	}
}

// 使用会话ID提交评论
function submitCommentWithSession(title, comment, sessionID, submitBtn, originalText) {
	const xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState == 4) {
			handleCommentSubmitResponse(xhr, submitBtn, originalText);
		}
	};
	
	const formData = new FormData();
	formData.append('title', title);
	formData.append('comment', comment);
	formData.append('session_id', sessionID);
	xhr.open('POST', '/comment', true);
	xhr.send(formData);
}

// 使用密码验证机制提交评论
function submitCommentWithPassword(title, comment, owner, mail, password, submitBtn, originalText) {
	const xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState == 4) {
			if (xhr.status == 200) {
				// 尝试从响应中提取会话ID并保存
				try {
					const response = JSON.parse(xhr.responseText);
					if (response.session_id) {
						saveCommentSessionID(owner, response.session_id);
						showToast('评论提交成功！已保存身份信息', 'success');
					}
				} catch (e) {
					// 响应不是JSON格式，说明是普通文本响应
					showToast('评论提交成功！', 'success');
				}
			}
			handleCommentSubmitResponse(xhr, submitBtn, originalText);
		}
	};
	
	const formData = new FormData();
	formData.append('title', title);
	formData.append('owner', owner);
	formData.append('mail', mail);
	formData.append('pwd', password); // 添加密码字段
	formData.append('comment', comment);
	xhr.open('POST', '/comment', true);
	xhr.send(formData);
}

// 作为匿名用户提交评论
function submitCommentAsAnonymous(title, comment, owner, mail, submitBtn, originalText) {
	const xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState == 4) {
			if (xhr.status == 200) {
				// 尝试从响应中提取会话ID并保存
				try {
					const response = JSON.parse(xhr.responseText);
					if (response.session_id) {
						saveCommentSessionID(owner, response.session_id);
					}
				} catch (e) {
					// 响应不是JSON格式，忽略
				}
			}
			handleCommentSubmitResponse(xhr, submitBtn, originalText);
		}
	};
	
	const formData = new FormData();
	formData.append('title', title);
	formData.append('owner', owner);
	formData.append('mail', mail);
	formData.append('comment', comment);
	xhr.open('POST', '/comment', true);
	xhr.send(formData);
}

// 处理评论提交响应
function handleCommentSubmitResponse(xhr, submitBtn, originalText) {
	// Re-enable submit button
	submitBtn.disabled = false;
	submitBtn.innerHTML = originalText;
	
	if (xhr.status == 200) {
		// 清空表单
		document.getElementById('input-comment').value = '';
		document.getElementById('input-owner').value = '';
		document.getElementById('input-mail').value = '';
		document.getElementById('input-pwd').value = '';
		updateCharCount('');
		
		// 刷新页面查看新评论
		setTimeout(() => {
			location.reload();
		}, 1500);
	} else {
		showToast('评论提交失败: ' + xhr.responseText, 'error');
	}
}

// 获取本地存储的评论会话ID
function getCommentSessionID(username) {
	try {
		const sessions = JSON.parse(localStorage.getItem('commentSessions') || '{}');
		return sessions[username] || null;
	} catch (e) {
		return null;
	}
}

// 保存评论会话ID到本地存储
function saveCommentSessionID(username, sessionID) {
	try {
		const sessions = JSON.parse(localStorage.getItem('commentSessions') || '{}');
		sessions[username] = sessionID;
		localStorage.setItem('commentSessions', JSON.stringify(sessions));
		
		// 设置过期时间（7天）
		const expiry = Date.now() + (7 * 24 * 60 * 60 * 1000);
		localStorage.setItem('commentSessionsExpiry', expiry.toString());
	} catch (e) {
		console.warn('无法保存评论会话ID:', e);
	}
}

// 清理过期的会话ID
function cleanupExpiredSessions() {
	try {
		const expiry = localStorage.getItem('commentSessionsExpiry');
		if (expiry && Date.now() > parseInt(expiry)) {
			localStorage.removeItem('commentSessions');
			localStorage.removeItem('commentSessionsExpiry');
		}
	} catch (e) {
		console.warn('清理过期会话时出错:', e);
	}
}

function onEditor() {
	const toggleBtn = document.getElementById('toggle-button');
	
		// PC version
		if (toggleBtn.innerText === '编辑') {
			md.className = 'mdEditor';
			editor.className = 'editor th_black';
			document.getElementById('editor-button').className = 'left-button';
			toggleBtn.innerText = '预览';
			editor.style.height = md.clientHeight + 'px';
		} else {
			md.className = 'md';
			editor.className = 'hide th_black';
			toggleBtn.innerText = '编辑';
		}
}

function submitFirst() {
	// 检查是否是加密博客
	const decryptInput = document.getElementById('decrypt-password');
	
	if (decryptInput !== null) {
		if (confirm('确定要提交修改吗？')) {
			submitContent();
		}
	} else {
		submitContent();
	}
}

function submitContent() {
	const content = editor ? editor.value : '';
	const titleElement = document.getElementById('title');
	const tagsElement = document.getElementById('tags');
	
	const title = titleElement ? titleElement.innerText : '';
	const tags = tagsElement ? tagsElement.value : '';
	
	// 获取加密密码 - 使用专门的加密设置输入框
	const encryptPasswordInput = document.getElementById('encrypt-password');
	let key = '';
	
	if (encryptPasswordInput !== null) {
		key = encryptPasswordInput.value;
	}
	
	// Get base auth type with null check
	const baseAuthElement = document.querySelector('input[name="base_auth_type"]:checked');
	const baseAuthType = baseAuthElement ? baseAuthElement.value : 'private';
	
	// Get special permissions with null checks
	const diaryElement = document.getElementById('diary_permission');
	const cooperationElement = document.getElementById('cooperation_permission');
	const encryptElement = document.getElementById('encrypt_permission');
	
	const diaryPermission = diaryElement ? diaryElement.checked : false;
	const cooperationPermission = cooperationElement ? cooperationElement.checked : false;
	const encryptPermission = encryptElement ? encryptElement.checked : false;
	
	// 检查是否已经是加密博客
	const decryptInput = document.getElementById('decrypt-password');
	const isAlreadyEncrypted = decryptInput !== null;
	
	// 验证加密权限与密码的一致性
	if (encryptPermission && !isAlreadyEncrypted && (!key || key.trim() === '')) {
		showToast('启用内容加密时必须设置加密密码', 'error');
		if (encryptPasswordInput) {
			encryptPasswordInput.focus();
		}
		return;
	}
	
	// Build combined auth type string
	let authTypeArray = [baseAuthType];
	if (diaryPermission) authTypeArray.push('diary');
	if (cooperationPermission) authTypeArray.push('cooperation');
	if (encryptPermission) authTypeArray.push('encrypt');
	
	const authType = authTypeArray.join(',');
	
	// Validate permissions using PermissionManager
	if (window.PermissionManager && !window.PermissionManager.validate()) {
		return;
	}
	
	// Show loading status with permission summary
	const permissionSummary = window.PermissionManager ? window.PermissionManager.getSummary() : '';
	showToast(`正在保存修改 (${permissionSummary})...`, 'info');
	
	// Create request
	const xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState == 4) {
			if (xhr.status == 200) {
				showToast(`修改保存成功！权限：${permissionSummary}`, 'success');
			} else {
				showToast('保存失败: ' + xhr.responseText, 'error');
			}
		}
	};
	
	// Handle encryption if needed
	let finalContent = content;
	let encryptFlag = '';
	
	if (encryptPermission) {
		if (key.length > 0) {
			// 有密码，进行加密（新加密或重新加密）
			finalContent = aesEncrypt(content, key);
			encryptFlag = 'use_aes_cbc';
		} else if (isAlreadyEncrypted) {
			// 已加密博客，没有新密码，保持原有加密状态
			encryptFlag = 'use_aes_cbc';
		}
		// 如果没有密码且不是已加密博客，前面的验证已经阻止了这种情况
	}
	
	// Send data
	const formData = new FormData();
	formData.append('title', title);
	formData.append('content', finalContent);
	formData.append('auth_type', authType);
	formData.append('tags', tags);
	formData.append('encrypt', encryptFlag);
	
	console.log('发送的表单数据:', {
		title,
		auth_type: authType,
		tags,
		encrypt: encryptFlag
	});
	
	xhr.open('POST', '/modify', true);
	xhr.send(formData);
}

// Handle editor resizing and preview updates
editor.addEventListener('input', function() {
	// 针对不同设备采用不同的高度调整策略
		// PC版本全屏显示
		this.style.height = 'auto';
		// 使用更大的高度值，覆盖整个可见区域
		this.style.height = (window.innerHeight * 0.85) + 'px';
		mdRender(this.value);
});

// Initialize editor and preview on page load
window.onload = function() {
	mdRender(editor.value);
	checkTime();

	// 自动隐藏侧边栏
		sidebar.classList.toggle('hide-sidebar');
		container.classList.toggle('hide-sidebar');
	
	// 初始化编辑器高度，避免高度为0的问题
		// PC端全屏显示
		editor.style.height = (window.innerHeight * 0.85) + 'px';
		// 同时应用全屏样式类
		editor.classList.add('editorfullscreen');
		
		// 预览区域也需要适应全屏
		if (md) {
			md.style.height = (window.innerHeight * 0.85) + 'px';
		}

	
	// 添加返回按钮
	addBackButton();
	
	// 初始化评论功能
	initCommentFeatures();
	
	// 清理过期的评论会话
	cleanupExpiredSessions();
}

// 添加返回按钮
function addBackButton() {
	// 创建返回按钮
	const backButton = document.createElement('button');
	backButton.id = 'back-button';
	backButton.className = 'back-button';
	backButton.innerHTML = '&larr; 返回';
	backButton.title = '返回上一页';
	
	// 添加点击事件
	backButton.addEventListener('click', function() {
		window.history.back();
	});
	
	// 添加到按钮容器内，而不是编辑器容器内
	const buttonsContainer = document.querySelector('.buttons-container');
	buttonsContainer.insertBefore(backButton, buttonsContainer.firstChild);
}

// 字符计数功能
function updateCharCount(text) {
	const charCounter = document.getElementById('char-counter');
	const charCount = document.querySelector('.char-count');
	
	if (charCounter && charCount) {
		const length = text.length;
		charCounter.textContent = length;
		
		// 更新颜色提示
		charCount.classList.remove('warning', 'danger');
		if (length > 400) {
			charCount.classList.add('danger');
		} else if (length > 300) {
			charCount.classList.add('warning');
		}
	}
}

// 初始化评论相关事件监听器
function initCommentFeatures() {
	const commentTextarea = document.getElementById('input-comment');
	
	if (commentTextarea) {
		// 添加字符计数监听器
		commentTextarea.addEventListener('input', function() {
			updateCharCount(this.value);
		});
		
		// 添加回车键支持（Ctrl+Enter提交）
		commentTextarea.addEventListener('keydown', function(e) {
			if (e.ctrlKey && e.key === 'Enter') {
				onCommitComment();
			}
		});
		
		// 初始化字符计数
		updateCharCount(commentTextarea.value);
	}
	
	// 添加表单验证提示
	const requiredInputs = document.querySelectorAll('#input-owner, #input-comment');
	requiredInputs.forEach(input => {
		input.addEventListener('blur', function() {
			if (this.hasAttribute('required') && !this.value.trim()) {
				this.style.borderColor = 'var(--danger-color)';
			} else {
				this.style.borderColor = 'var(--border-color)';
			}
		});
		
		input.addEventListener('input', function() {
			if (this.style.borderColor === 'var(--danger-color)' && this.value.trim()) {
				this.style.borderColor = 'var(--border-color)';
			}
		});
	});
	
	// 用户名实时检查
	const ownerInput = document.getElementById('input-owner');
	if (ownerInput) {
		ownerInput.addEventListener('input', function() {
			clearTimeout(window.usernameCheckTimeout);
			window.usernameCheckTimeout = setTimeout(checkUsernameStatus, 500);
		});
	}
}

// 检查用户名状态并显示提示
function checkUsernameStatus() {
	const ownerInput = document.getElementById('input-owner');
	const usernameHint = document.getElementById('username-hint');
	const passwordGroup = document.getElementById('input-pwd').closest('.form-group');
	const passwordLabel = passwordGroup ? passwordGroup.querySelector('label') : null;
	const username = ownerInput.value.trim();
	
	if (username.length < 2) {
		if (usernameHint) usernameHint.textContent = '';
		window.currentUsernameStatus = null;
		if (passwordLabel) passwordLabel.textContent = '身份密码';
		return;
	}
	
	const xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState == 4 && xhr.status == 200) {
			try {
				const response = JSON.parse(xhr.responseText);
				if (response.success && usernameHint) {
					// 保存状态供表单验证使用
					window.currentUsernameStatus = response;
					
					usernameHint.textContent = response.message;
					
					// 根据用户数量改变提示颜色和密码字段标签
					if (response.user_count === 0) {
						usernameHint.className = 'form-hint new-user';
						if (passwordLabel) passwordLabel.textContent = '身份密码（可选）';
					} else {
						usernameHint.className = 'form-hint existing-user';
						if (passwordLabel) passwordLabel.textContent = '身份密码 *';
					}
				}
			} catch (e) {
				console.error('解析用户名检查响应失败:', e);
			}
		}
	};
	
	xhr.open('GET', `/api/check-username?username=${encodeURIComponent(username)}`, true);
	xhr.send();
}

function checkLogin(value) {
    // 简单的登录检查函数
    return value && value.length > 0;
}

// 初始化加密权限交互
window.addEventListener('load', function() {
    const encryptCheckbox = document.getElementById('encrypt_permission');
    const encryptInput = document.getElementById('encrypt');
    
    if (encryptCheckbox && encryptInput) {
        encryptCheckbox.addEventListener('change', function() {
            if (this.checked && !encryptInput.value.trim()) {
                // 滚动到密码输入框
                encryptInput.scrollIntoView({ behavior: 'smooth', block: 'center' });
                
                setTimeout(() => {
                    encryptInput.focus();
                    encryptInput.style.animation = 'passwordHighlight 2s ease-in-out';
                }, 300);
                
                showToast('🔐 内容加密已启用！请在下方设置加密密码', 'info');
            }
        });
    }
});

// 编辑页面权限控制初始化
function initEditPagePermissions() {
    const encryptCheckbox = document.getElementById('encrypt_permission');
    const encryptPasswordInput = document.getElementById('encrypt-password');
    const encryptSection = document.getElementById('encrypt-section-edit');
    const encryptLabel = document.getElementById('encrypt-password-label');
    const encryptHint = document.getElementById('encrypt-password-hint');
    
    if (!encryptCheckbox || !encryptPasswordInput) {
        return; // 不是编辑页面或元素不存在
    }
    
    // 初始状态设置
    updateEditPageEncryptState();
    
    // 监听加密权限变化
    encryptCheckbox.addEventListener('change', function() {
        updateEditPageEncryptState();
        
        if (this.checked && !encryptPasswordInput.value.trim()) {
            // 滚动到密码输入框
            encryptPasswordInput.scrollIntoView({ behavior: 'smooth', block: 'center' });
            
            // 延迟聚焦，确保滚动完成
            setTimeout(() => {
                encryptPasswordInput.focus();
                // 添加视觉提示
                encryptPasswordInput.style.animation = 'passwordHighlight 2.5s ease-in-out';
            }, 300);
            
            showToast('🔐 内容加密已启用！请在下方密码区域设置加密密码', 'info');
        } else if (this.checked) {
            showToast('🔐 内容加密已启用！', 'success');
        }
    });
    
    // 监听密码输入框变化
    encryptPasswordInput.addEventListener('input', function() {
        // 如果输入了密码但没有启用加密权限，自动启用
        if (this.value.trim() && !encryptCheckbox.checked) {
            encryptCheckbox.checked = true;
            updateEditPageEncryptState();
            showToast('已自动启用内容加密', 'info');
        }
    });
    
    function updateEditPageEncryptState() {
        // 检查是否已经是加密博客
        const decryptInput = document.getElementById('decrypt-password');
        const isAlreadyEncrypted = decryptInput !== null;
        
        if (encryptCheckbox.checked) {
            // 启用加密时的样式
            encryptPasswordInput.style.borderColor = '#4CAF50';
            encryptPasswordInput.style.backgroundColor = 'rgba(76, 175, 80, 0.1)';
            
            if (isAlreadyEncrypted) {
                // 已加密博客的提示
                encryptPasswordInput.placeholder = '🔐 留空保持原密码，或输入新密码重新加密';
                encryptPasswordInput.required = false;
                
                if (encryptLabel) {
                    encryptLabel.textContent = '🔐 加密密码 (可选)';
                    encryptLabel.style.color = '#4CAF50';
                    encryptLabel.style.fontWeight = 'bold';
                }
                
                if (encryptHint) {
                    encryptHint.textContent = '✅ 内容已加密 - 留空保持原密码，输入新密码则重新加密';
                    encryptHint.style.color = '#4CAF50';
                }
            } else {
                // 新加密博客的提示
                encryptPasswordInput.placeholder = '🔐 请输入加密密码（必填）';
                encryptPasswordInput.required = true;
                
                if (encryptLabel) {
                    encryptLabel.textContent = '🔐 加密密码 (必填)';
                    encryptLabel.style.color = '#4CAF50';
                    encryptLabel.style.fontWeight = 'bold';
                }
                
                if (encryptHint) {
                    encryptHint.textContent = '✅ 内容加密已启用 - 请设置一个安全的密码';
                    encryptHint.style.color = '#4CAF50';
                }
            }
            
            if (encryptSection) {
                encryptSection.style.backgroundColor = 'rgba(76, 175, 80, 0.05)';
                encryptSection.style.border = '1px solid rgba(76, 175, 80, 0.3)';
                encryptSection.style.borderRadius = '6px';
                encryptSection.style.padding = '10px';
            }
        } else {
            // 未启用加密时的样式
            encryptPasswordInput.style.borderColor = '';
            encryptPasswordInput.style.backgroundColor = '';
            encryptPasswordInput.placeholder = '设置加密密码...';
            encryptPasswordInput.required = false;
            
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

