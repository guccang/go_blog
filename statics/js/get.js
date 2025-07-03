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

// Toggle sidebar
bubble.addEventListener('click', function() {
	if (isPCDevice()){
		sidebar.classList.toggle('hide-sidebar');
		container.classList.toggle('hide-sidebar');
	}else{
		sidebar.classList.toggle('hide-sidebar-mobile');
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
	const k = document.getElementById('encrypt').value;
	
	if (!k) {
		showToast('请输入解密密码', 'error');
		return;
	}
	
	try {
		const content = e.innerHTML;
		const txt = aesDecrypt(content, k);
		e.innerHTML = txt;
		mdRender(txt);
		showToast('解密成功', 'success');
	} catch (error) {
		showToast('解密失败，密码可能不正确', 'error');
	}
}

function onShowComment() {
	const btn = document.getElementById('comment-show');
	const comments = document.getElementById('comments');
	const divComment = document.getElementById('div-comment');
	
	if (btn.innerText === '显示评论') {
		comments.classList.remove('hide');
		divComment.classList.remove('hide');
		btn.innerText = '折叠评论';
	} else {
		comments.classList.add('hide');
		divComment.classList.add('hide');
		btn.innerText = '显示评论';
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
						window.location.href = '/link';
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
	let pwd = document.getElementById('input-pwd').value;
	
	// Validate form
	if (!owner || !comment) {
		showToast('请填写用户名和评论内容', 'error');
		return;
	}
	
	// Hash password
	pwd = CryptoJS.MD5(pwd).toString();

	// Send data
	const xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState == 4) {
			if (xhr.status == 200) {
				showToast('评论提交成功', 'success');
				// Clear form
				document.getElementById('input-comment').value = '';
				// Refresh to see new comment
				setTimeout(() => {
					location.reload();
				}, 1500);
			} else {
				showToast('评论提交失败: ' + xhr.responseText, 'error');
			}
		}
	};
	
	const formData = new FormData();
	formData.append('title', title);
	formData.append('owner', owner);
	formData.append('pwd', pwd);
	formData.append('mail', mail);
	formData.append('comment', comment);
	xhr.open('POST', '/comment', true);
	xhr.send(formData);
}

function onEditor() {
	const toggleBtn = document.getElementById('toggle-button');
	
	if (isPCDevice()) {
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
	} else {
		// Mobile version
		if (toggleBtn.innerText === '编辑') {
			md.className = 'hide';
			editor.className = 'editor th_black';
			document.getElementById('editor-button').className = 'left-button';
			toggleBtn.innerText = '预览';
			
			// 设置适合移动设备的编辑器高度
			const minHeight = Math.max(window.innerHeight * 0.9, 300);
			editor.style.height = Math.max(editor.scrollHeight, minHeight) + 'px';
			console.log("height ",minHeight,editor.style.height,window.innerHeight)
		} else {
			md.className = 'md';
			editor.className = 'hide th_black';
			document.getElementById('editor-button').className = 'hide';
			toggleBtn.innerText = '编辑';
		}
	}
}

function submitFirst() {
	const encryptInput = document.getElementById('encrypt');
	
	if (encryptInput !== null) {
		if (confirm('确定要提交修改吗？')) {
			submitContent();
		}
	} else {
		submitContent();
	}
}

function submitContent() {
	const content = editor.value;
	const title = document.getElementById('title').innerText;
	const tags = document.getElementById('tags').value;
	const encryptInput = document.getElementById('encrypt');
	let key = '';
	
	if (encryptInput !== null) {
		key = encryptInput.value;
	}
	
	const authType = document.querySelector('input[name="auth_type"]:checked').value;
	
	// Show loading status
	showToast('正在保存...', 'info');
	
	// Create request
	const xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState == 4) {
			if (xhr.status == 200) {
				showToast('保存成功', 'success');
			} else {
				showToast('保存失败: ' + xhr.responseText, 'error');
			}
		}
	};
	
	// Handle encryption if needed
	let finalContent = content;
	
	if (key.length > 0) {
		finalContent = aesEncrypt(content, key);
		key = 'use_aes_cbc';
	}
	
	// Send data
	const formData = new FormData();
	formData.append('title', title);
	formData.append('content', finalContent);
	formData.append('auth_type', authType);
	formData.append('tags', tags);
	formData.append('encrypt', key);
	xhr.open('POST', '/modify', true);
	xhr.send(formData);
}

// Handle editor resizing and preview updates
editor.addEventListener('input', function() {
	// 针对不同设备采用不同的高度调整策略
	if (isPCDevice()) {
		// PC版本全屏显示
		this.style.height = 'auto';
		// 使用更大的高度值，覆盖整个可见区域
		this.style.height = (window.innerHeight * 0.85) + 'px';
		mdRender(this.value);
	} else {
		// 移动端版本使用固定高度或基于内容的自适应高度
		this.style.height = 'auto';
		// 增加移动端高度比例，确保显示更完整
		const minHeight = Math.max(window.innerHeight * 0.95, 300);
		this.style.height = Math.max(this.scrollHeight, minHeight) + 'px';
		mdRender(this.value);
	}
});

// Initialize editor and preview on page load
window.onload = function() {
	mdRender(editor.value);
	checkTime();

	// 自动隐藏侧边栏
	if (isPCDevice()){
		sidebar.classList.toggle('hide-sidebar');
		container.classList.toggle('hide-sidebar');
	}else{
		sidebar.classList.toggle('hide-sidebar-mobile');
		container.classList.toggle('hide-sidebar');
	}
	
	// 初始化编辑器高度，避免高度为0的问题
	if (isPCDevice()) {
		// PC端全屏显示
		editor.style.height = (window.innerHeight * 0.85) + 'px';
		// 同时应用全屏样式类
		editor.classList.add('editorfullscreen');
		
		// 预览区域也需要适应全屏
		if (md) {
			md.style.height = (window.innerHeight * 0.85) + 'px';
		}
	} else {
		// 增加移动端高度比例
		const minHeight = Math.max(window.innerHeight * 0.95, 300);
		editor.style.height = minHeight + 'px';
	}
	
	// 添加返回按钮
	addBackButton();
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