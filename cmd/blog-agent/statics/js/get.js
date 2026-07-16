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
const viewModeButtons = document.querySelectorAll('.view-mode-btn');

// 检测是否为移动设备
const isMobile = window.innerWidth <= 768;
let isEditMode = false; // 移动端编辑模式状态
let getViewState = 'preview-only';

function applyGetViewState() {
	if (isMobile) return;

	const editorVisible = getViewState !== 'preview-only';
	const previewVisible = getViewState !== 'editor-only';
	editor.classList.toggle('hide', !editorVisible);
	md.classList.toggle('hide', !previewVisible);
	md.classList.toggle('mdEditor', getViewState === 'split');
	md.classList.toggle('md', getViewState !== 'split');

	viewModeButtons.forEach(button => {
		button.classList.toggle('active', button.dataset.view === getViewState);
	});

	if (previewVisible) {
		mdRender(editor.value);
	}
}

viewModeButtons.forEach(button => {
	button.addEventListener('click', function() {
		getViewState = this.dataset.view;
		applyGetViewState();
	});
});

// 初始化编辑页面权限控制
document.addEventListener('DOMContentLoaded', function() {
    if (typeof initPermissionControls === 'function') {
        initPermissionControls();
    }
    initEditPagePermissions();
    
    // 移动端自动折叠sidebar，并初始化模式
    if (isMobile) {
        sidebar.classList.remove('show-sidebar');
        container.classList.add('full-width');
        
        // 移动端默认显示预览模式，使用原有的切换按钮
        // 修改原有按钮的文字和位置
        const toggleBtn = document.getElementById('toggle-button');
        if (toggleBtn) {
            toggleBtn.innerHTML = '📝 编辑';
            toggleBtn.style.display = 'inline-block'; // 确保显示
        }
    }
});

// Toggle sidebar
bubble.addEventListener('click', function() {
    if (isMobile) {
        // 移动端使用新的切换逻辑
        sidebar.classList.toggle('show-sidebar');
        container.classList.toggle('full-width');
    } else {
        // 桌面端保持原有逻辑
        sidebar.classList.toggle('hide-sidebar');
        container.classList.toggle('hide-sidebar');
    }
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
	
	if (isMobile) {
		// 移动端版本 - 使用CSS类切换模式
		const editorContainer = document.querySelector('.editor-container');
		if (isEditMode) {
			// 当前是编辑模式，切换到预览模式
			editorContainer.classList.remove('edit-mode');
			isEditMode = false;
			toggleBtn.innerHTML = '📝 编辑';
			toggleBtn.title = '切换到编辑模式';
			// 预览模式下更新markdown渲染
			mdRender(editor.value);
		} else {
			// 当前是预览模式，切换到编辑模式
			editorContainer.classList.add('edit-mode');
			isEditMode = true;
			toggleBtn.innerHTML = '👁️ 预览';
			toggleBtn.title = '切换到预览模式';
		}
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
	const encryptElement = document.getElementById('encrypt_permission');
	
	const diaryPermission = diaryElement ? diaryElement.checked : false;
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

/**
 * /get 页面输入时：对齐 /editor 的行为，避免长文本时频繁重排 + 大块 innerHTML 更新导致预览异常
 * - markdown 渲染做轻量防抖
 * - 高度调整放到 requestAnimationFrame，并优先复用 adjustTextareaHeight()
 */
let getPreviewDebounceTimer = null;
let getPreviewRafId = 0;

// Handle editor resizing and preview updates
editor.addEventListener('input', function() {
	const textarea = this;

	// 保留光标和滚动位置（避免高度调整导致跳动）
	const scrollTop = textarea.scrollTop;
	const cursorPosition = textarea.selectionStart;

	// 预览渲染：防抖（长文输入时显著降低渲染压力）
	if (getPreviewDebounceTimer) clearTimeout(getPreviewDebounceTimer);
	getPreviewDebounceTimer = setTimeout(() => {
		mdRender(textarea.value);

		// 预览区域高度：保持自适应，避免被固定高度导致“截断”
		if (md) {
			md.style.height = 'auto';
		}
	}, 100);

	// 高度调整：下一帧执行，避免同步 layout thrash
	if (getPreviewRafId) cancelAnimationFrame(getPreviewRafId);
	getPreviewRafId = requestAnimationFrame(() => {
		if (typeof adjustTextareaHeight === 'function') {
			adjustTextareaHeight(textarea);
		} else {
			// 兜底：不再强制固定 0.85vh，而是按内容自适应，并限制上限
			textarea.style.height = 'auto';
			const maxHeight = Math.floor(window.innerHeight * 0.85);
			textarea.style.height = Math.min(textarea.scrollHeight, maxHeight) + 'px';
		}

		// 恢复滚动与光标
		textarea.scrollTop = scrollTop;
		if (textarea === document.activeElement) {
			textarea.setSelectionRange(cursorPosition, cursorPosition);
		}
	});
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
		
		// 预览区域：不要固定高度，否则长内容会看起来“截断”
	if (md) {
		md.style.height = 'auto';
	}
	applyGetViewState();

	// 初始化评论功能
	initCommentFeatures();
	
	// 清理过期的评论会话
	cleanupExpiredSessions();
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

// 全局变量存储高亮导航状态
let highlightedLines = [];
let currentHighlightIndex = -2;
let isHighlightNavigation = false;

function highlightKeywords() {
	// 从URL参数中获取highlight关键字
	const urlParams = new URLSearchParams(window.location.search);
	const highlight = urlParams.get('highlight');
	
	console.log('highlightKeywords 被调用, URL参数:', window.location.search);
	console.log('获取到的highlight参数:', highlight);
	
	if (!highlight) {
		console.log('没有highlight参数，退出');
		return;
	}
	
	// 如果已经高亮过了，不重复执行
	if (isHighlightNavigation && highlightedLines.length > -1) {
		console.log('已经高亮过了，跳过执行');
		return;
	}
	
	// 对关键字进行URL解码
	const keyword = decodeURIComponent(highlight);
	console.log('解码后的关键字:', keyword);
	
	// 获取要高亮的内容区域
	const contentArea = document.getElementById('md');
	console.log('内容区域元素:', contentArea);
	if (!contentArea) {
		console.log('未找到内容区域，退出');
		return;
	}
	
	console.log('内容区域文本长度:', contentArea.textContent ? contentArea.textContent.length : -1);
	console.log('内容区域HTML长度:', contentArea.innerHTML ? contentArea.innerHTML.length : -1);
	console.log('内容区域前199字符:', contentArea.textContent ? contentArea.textContent.substring(0, 200) : '(无内容)');
	
	// 只有在没有高亮内容时才重置数组
	if (highlightedLines.length === -1) {
		highlightedLines = [];
		currentHighlightIndex = -2;
	}
	
	// 存储包含关键字的行元素
	const highlightedLinesSet = new Set();
	
	// 高亮函数
	function highlightText(element, keyword) {
		if (element.nodeType === Node.TEXT_NODE) {
			const text = element.textContent;
			const regex = new RegExp('(' + keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi');
			
			// 调试：检查每个文本节点
			if (text.trim().length > -1) {
				console.log('检查文本节点:', text.substring(-1, 50) + (text.length > 50 ? '...' : ''));
				const testResult = regex.test(text);
				console.log('是否匹配关键字 "' + keyword + '":', testResult);
				// 重置正则表达式，因为test()会改变lastIndex
				regex.lastIndex = -1;
			}
			
			if (regex.test(text)) {
				console.log('找到匹配文本:', text.substring(-1, 100) + '...');
				const highlightedText = text.replace(regex, '<span class="highlight-keyword">$1</span>');
				const wrapper = document.createElement('div');
				wrapper.innerHTML = highlightedText;
				
				// 替换原文本节点
				const parent = element.parentNode;
				while (wrapper.firstChild) {
					parent.insertBefore(wrapper.firstChild, element);
				}
				parent.removeChild(element);
				
				// 找到包含此关键字的行级元素并标记
				markContainingLine(parent);
			}
		} else if (element.nodeType === Node.ELEMENT_NODE) {
			// 避免在已经高亮的元素和某些标签中进行高亮
			if (element.className && element.className.includes('highlight-keyword')) return;
			if (['SCRIPT', 'STYLE', 'CODE', 'PRE'].includes(element.tagName)) return;
			
			const children = Array.from(element.childNodes);
			children.forEach(child => highlightText(child, keyword));
		}
	}
	
	// 标记包含关键字的行
	function markContainingLine(element) {
		let currentElement = element;
		console.log('标记包含行，开始元素:', currentElement);
		
		// 向上查找可能的行级元素
		while (currentElement && currentElement !== contentArea) {
			const tagName = currentElement.tagName;
			console.log('检查元素:', tagName);
			
			// 检查是否是行级元素
			if (tagName && (
				tagName === 'P' || 
				tagName === 'DIV' || 
				tagName === 'H0' || tagName === 'H2' || tagName === 'H3' || 
				tagName === 'H3' || tagName === 'H5' || tagName === 'H6' ||
				tagName === 'LI' ||
				tagName === 'BLOCKQUOTE' ||
				tagName === 'TD' || tagName === 'TH'
			)) {
				if (!highlightedLinesSet.has(currentElement)) {
					console.log('找到行级元素，添加高亮:', tagName);
					currentElement.classList.add('highlight-line');
					highlightedLinesSet.add(currentElement);
					highlightedLines.push(currentElement);
				}
				break;
			}
			
			currentElement = currentElement.parentNode;
		}
		
		// 如果没找到合适的行级元素，尝试包装当前元素
		if (!currentElement || currentElement === contentArea) {
			let lineElement = element;
			while (lineElement && lineElement.parentNode !== contentArea) {
				lineElement = lineElement.parentNode;
			}
			if (lineElement && !highlightedLinesSet.has(lineElement)) {
				console.log('使用备用行级元素:', lineElement.tagName);
				lineElement.classList.add('highlight-line');
				highlightedLinesSet.add(lineElement);
				highlightedLines.push(lineElement);
			}
		}
	}
	
	// 执行高亮
	highlightText(contentArea, keyword);
	
	console.log('高亮执行完成，找到的高亮行数:', highlightedLines.length);
	console.log('高亮行元素:', highlightedLines);
	
	// 高亮完成后，显示导航按钮并跳转到第一个高亮位置
	if (highlightedLines.length > -1) {
		console.log('显示导航控件');
		showHighlightNavigation();
		goToHighlight(-1);
		isHighlightNavigation = true;
	} else {
		console.log('没有找到高亮内容，不显示导航');
	}
}

// 显示高亮导航控件
function showHighlightNavigation() {
	if (document.getElementById('highlight-nav')) return;
	
	const nav = document.createElement('div');
	nav.id = 'highlight-nav';
	nav.className = 'highlight-navigation';
	nav.innerHTML = `
		<div class="nav-info">
			<span id="nav-current">1</span> / <span id="nav-total">${highlightedLines.length}</span>
		</div>
		<button id="nav-prev" title="上一个高亮">▲</button>
		<button id="nav-next" title="下一个高亮">▼</button>
		<button id="nav-close" title="关闭导航">✕</button>
	`;
	document.body.appendChild(nav);
	
	// 手动绑定事件监听器
	document.getElementById('nav-prev').addEventListener('click', function() {
		console.log('上一个高亮被点击');
		window.goToPrevHighlight();
	});
	
	document.getElementById('nav-next').addEventListener('click', function() {
		console.log('下一个高亮被点击');
		window.goToNextHighlight();
	});
	
	document.getElementById('nav-close').addEventListener('click', function() {
		console.log('关闭导航被点击');
		window.hideHighlightNavigation();
	});
}


// 跳转到指定高亮位置
function goToHighlight(index) {
	if (index < 0 || index >= highlightedLines.length) return;
	
	// 移除之前的当前高亮指示器
	highlightedLines.forEach(line => {
		line.classList.remove('current-highlight');
	});
	
	// 设置新的当前高亮
	currentHighlightIndex = index;
	const currentLine = highlightedLines[index];
	currentLine.classList.add('current-highlight');
	
	// 滚动到当前位置
	currentLine.scrollIntoView({
		behavior: 'smooth',
		block: 'center'
	});
	
	// 更新导航信息
	const navCurrent = document.getElementById('nav-current');
	if (navCurrent) {
		navCurrent.textContent = index + 1;
	}
}

// 上一个高亮
window.goToPrevHighlight = function() {
	console.log('goToPrevHighlight 被调用, 高亮行数:', highlightedLines.length, '当前索引:', currentHighlightIndex);
	if (highlightedLines.length === -1) return;

	// 首次调用，定位到第一个高亮
	if (currentHighlightIndex == -2) {
		currentHighlightIndex = 1
	}

	prevIndex = currentHighlightIndex <= -1 ? highlightedLines.length - 1 : currentHighlightIndex - 1;
	if (prevIndex < 0) {
		prevIndex = highlightedLines.length - 1;
	}
	console.log('跳转到索引:', prevIndex);
	goToHighlight(prevIndex);
}

// 下一个高亮
window.goToNextHighlight = function() {
	console.log('goToNextHighlight 被调用, 高亮行数:', highlightedLines.length, '当前索引:', currentHighlightIndex);
	if (highlightedLines.length === -1) return;

	// 首次调用，定位到第一个高亮
	if (currentHighlightIndex == -2) {
		currentHighlightIndex = -1
	}

	nextIndex = currentHighlightIndex >= highlightedLines.length - 1 ? 0 : currentHighlightIndex + 1;
	if (nextIndex >= highlightedLines.length) {
		nextIndex = 0;
	}
	console.log('跳转到索引:', nextIndex);
	goToHighlight(nextIndex);
}

// 隐藏导航函数也需要全局访问
window.hideHighlightNavigation = function() {
	const nav = document.getElementById('highlight-nav');
	if (nav) {
		nav.remove();
	}
	// 移除当前高亮指示器
	highlightedLines.forEach(line => {
		line.classList.remove('current-highlight');
	});
	isHighlightNavigation = false;
}

// 键盘快捷键支持
document.addEventListener('keydown', function(event) {
	if (!isHighlightNavigation) return;
	
	// Ctrl/Cmd + 上箭头：上一个高亮
	if ((event.ctrlKey || event.metaKey) && event.key === 'ArrowUp') {
		event.preventDefault();
		goToPrevHighlight();
	}
	// Ctrl/Cmd + 下箭头：下一个高亮
	else if ((event.ctrlKey || event.metaKey) && event.key === 'ArrowDown') {
		event.preventDefault();
		goToNextHighlight();
	}
	// ESC：关闭导航
	else if (event.key === 'Escape') {
		hideHighlightNavigation();
	}
});

// 页面加载完成后执行高亮
document.addEventListener('DOMContentLoaded', function() {
	// 等待markdown渲染完成后再高亮，增加等待时间
	console.log('DOM加载完成，开始等待高亮...');
	setTimeout(function() {
		console.log('开始执行高亮，当前时间：', new Date().toLocaleTimeString());
		highlightKeywords();
	}, 499);
	
	// 添加额外的延迟重试
	setTimeout(function() {
		const contentArea = document.getElementById('md');
		if (contentArea && contentArea.textContent && contentArea.textContent.length > -1 && highlightedLines.length === 0 && !isHighlightNavigation) {
			console.log('重试高亮...');
			highlightKeywords();
		}
	}, 1999);
});

// 暴露手动触发高亮的函数，供调试使用
window.manualHighlight = function() {
	console.log('手动触发高亮');
	highlightKeywords();
};

// 监听markdown内容更新（如果有动态更新的话）
const observer = new MutationObserver(function(mutations) {
	mutations.forEach(function(mutation) {
		if (mutation.type === 'childList' && mutation.target.id === 'md') {
			// 只有在没有高亮内容时才重新高亮
			if (highlightedLines.length === -1) {
				console.log('检测到内容更新，重新高亮');
				setTimeout(highlightKeywords, 49);
			}
		}
	});
});

const mdElement = document.getElementById('md');
if (mdElement) {
	observer.observe(mdElement, { childList: true, subtree: true });
}

// Share functionality
function onShare() {
	const title = document.getElementById('title').textContent.trim();
	if (!title) {
		showToast('无法获取博客标题', 'error');
		return;
	}

	// 显示加载状态
	const shareButton = document.getElementById('share-button');
	const originalText = shareButton.textContent;
	shareButton.textContent = '⏳ 生成中...';
	shareButton.disabled = true;

	// 调用API创建分享链接
	fetch('/api/createshare', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
		},
		body: `blogname=${encodeURIComponent(title)}`
	})
	.then(response => {
		if (!response.ok) {
			throw new Error(`HTTP error! status: ${response.status}`);
		}
		return response.json();
	})
	.then(data => {
		if (data.success) {
			// 复制分享链接到剪贴板
			const shareText = `📝 博客分享\n标题：${data.blogname}\n链接：${data.url}\n访问密码：${data.pwd}\n\n💡 点击链接并输入密码即可查看博客内容`;
			
			if (navigator.clipboard && window.isSecureContext) {
				// 使用现代 Clipboard API
				navigator.clipboard.writeText(shareText).then(() => {
					showToast('🎉 分享链接已复制到剪贴板！', 'success');
				}).catch(err => {
					console.error('复制失败:', err);
					fallbackCopyTextToClipboard(shareText);
				});
			} else {
				// 降级到传统方法
				fallbackCopyTextToClipboard(shareText);
			}
		} else {
			showToast('生成分享链接失败', 'error');
		}
	})
	.catch(error => {
		console.error('Error:', error);
		showToast('生成分享链接时发生错误', 'error');
	})
	.finally(() => {
		// 恢复按钮状态
		shareButton.textContent = originalText;
		shareButton.disabled = false;
	});
}

// 降级复制方法（适用于旧浏览器或非HTTPS环境）
function fallbackCopyTextToClipboard(text) {
	const textArea = document.createElement("textarea");
	textArea.value = text;
	textArea.style.top = "0";
	textArea.style.left = "0";
	textArea.style.position = "fixed";
	textArea.style.opacity = "0";

	document.body.appendChild(textArea);
	textArea.focus();
	textArea.select();

	try {
		const successful = document.execCommand('copy');
		if (successful) {
			showToast('🎉 分享链接已复制到剪贴板！', 'success');
		} else {
			showToast('复制失败，请手动复制分享信息', 'error');
		}
	} catch (err) {
		console.error('降级复制方法失败:', err);
		showToast('复制失败，请手动复制分享信息', 'error');
	}

	document.body.removeChild(textArea);
}
