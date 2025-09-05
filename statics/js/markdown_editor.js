    // Initialize Vim editor
	vim.open({
		//debug : true
	});

    PageHistoryBack();
    
    // DOM Elements
    const sidebar = document.getElementById('sidebar-container');
    const bubble = document.getElementById('bubble');
    const container = document.querySelector('.container');
    const editor = document.getElementById('editor');
    const md = document.getElementById('md');
    const editorWrapper = document.getElementById('editor-wrapper');
    const previewWrapper = document.getElementById('preview-wrapper');
    const btnToggleView = document.getElementById('btn-toggle-view');
    
    // Initialize view state
    let viewState = 'split'; // split, editor-only, preview-only
    
    // 检测是否为移动设备
    const isMobile = window.innerWidth <= 768;
    
    // 如果是移动设备，自动切换到纯编辑模式并折叠sidebar
    if (isMobile) {
        viewState = 'editor-only';
        editorWrapper.classList.add('fullscreen');
        previewWrapper.classList.add('hidden');
        btnToggleView.innerHTML = '👁️';
        btnToggleView.style.display = 'none'; // 移动端隐藏切换按钮
        
        // 移动端自动折叠sidebar
        sidebar.classList.remove('show-sidebar');
        container.classList.add('full-width');
    }
    
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
    
    // Toggle view (split, editor-only, preview-only)
    btnToggleView.addEventListener('click', function() {
        switch(viewState) {
            case 'split':
                viewState = 'editor-only';
                editorWrapper.classList.add('fullscreen');
                previewWrapper.classList.add('hidden');
                btnToggleView.innerHTML = '👁️';
                break;
            case 'editor-only':
                viewState = 'preview-only';
                editorWrapper.classList.add('hidden');
                editorWrapper.classList.remove('fullscreen');
                previewWrapper.classList.remove('hidden');
                previewWrapper.classList.add('fullscreen');
                btnToggleView.innerHTML = '📝';
                break;
            case 'preview-only':
                viewState = 'split';
                editorWrapper.classList.remove('hidden');
                previewWrapper.classList.remove('fullscreen');
                btnToggleView.innerHTML = '📑';
                break;
        }
    });
    
    // Toolbar buttons functionality
    document.getElementById('btn-bold').addEventListener('click', () => insertMarkdown('**', '**'));
    document.getElementById('btn-italic').addEventListener('click', () => insertMarkdown('*', '*'));
    document.getElementById('btn-heading').addEventListener('click', () => insertMarkdown('# ', ''));
    document.getElementById('btn-link').addEventListener('click', () => insertMarkdown('[', '](https://)'));
    document.getElementById('btn-image').addEventListener('click', () => insertMarkdown('![alt text](', ')'));
    document.getElementById('btn-code').addEventListener('click', () => insertMarkdown('```\n', '\n```'));
    document.getElementById('btn-list').addEventListener('click', () => insertMarkdown('- ', ''));
    document.getElementById('btn-quote').addEventListener('click', () => insertMarkdown('> ', ''));
    
    // Function to insert markdown syntax
    function insertMarkdown(before, after) {
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        const selectedText = editor.value.substring(start, end);
        const replacement = before + selectedText + after;
        editor.value = editor.value.substring(0, start) + replacement + editor.value.substring(end);
        
        // Update selection position
        const newPos = start + before.length + selectedText.length;
        editor.setSelectionRange(start + before.length, newPos);
        editor.focus();
        
        // Update preview
        mdRender(editor.value);
    }
    
    // Function to show toast notifications
    function showToast(message, type = 'info') {
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.innerHTML = `<span class="toast-message">${message}</span>`;
        document.getElementById('toast-container').appendChild(toast);
        
        // Remove toast after 4 seconds
        setTimeout(() => {
            toast.remove();
        }, 4000);
    }
    
    // Initialize editor and preview
    window.onload = function() {
        // First set the editor content (if any)
        if (editor.value) {
            const scrollPos = window.pageYOffset || document.documentElement.scrollTop;
            
            // Render markdown content
            mdRender(editor.value);
            
            // Ensure theme is correctly applied
            checkTime();
            
            // Set editor height to match container height after a small delay
            // to ensure the DOM is fully rendered
            setTimeout(() => {
                adjustEditorHeight();
                // Focus at the end of the content
                if (editor.value.length > 0) {
                    editor.selectionStart = editor.selectionEnd = editor.value.length;
                }
                // Restore scroll position
                window.scrollTo(0, scrollPos);
            }, 100);
        } else {
            // Empty editor - just adjust height
            adjustEditorHeight();
            checkTime();
        }
        
        // Set up additional listeners for window resize
        window.addEventListener('resize', function() {
            // Debounce resize event
            if (this.resizeTimeout) clearTimeout(this.resizeTimeout);
            this.resizeTimeout = setTimeout(function() {
                adjustEditorHeight();
            }, 200);
        });
    };
    
    // Adjust editor height
    function adjustEditorHeight() {
        const currentScrollPos = window.pageYOffset || document.documentElement.scrollTop;
        const cursorPosition = editor.selectionStart;
        const scrollTop = editor.scrollTop;
        
        const editorContent = document.getElementById('editor-content');
        const toolbar = document.querySelector('.editor-toolbar');
        const availableHeight = window.innerHeight - toolbar.offsetHeight;
        
        // Set the editor container height
        editorContent.style.height = availableHeight + 'px';
        
        // Ensure editor's height is proportional to content
        if (editor.scrollHeight > editor.clientHeight) {
            editor.style.height = 'auto';
            editor.style.height = Math.max(editor.scrollHeight, availableHeight) + 'px';
        }
        
        // Restore positions to prevent jumping
        window.scrollTo(0, currentScrollPos);
        editor.scrollTop = scrollTop;
        if (editor === document.activeElement) {
            editor.setSelectionRange(cursorPosition, cursorPosition);
        }
    }
    
    // Adjust height on window resize
    window.addEventListener('resize', adjustEditorHeight);
    
    // Real-time preview
    editor.addEventListener('input', function() {
        mdRender(this.value);
        
        // Preserve scroll position and cursor position when adjusting editor height
        const scrollTop = this.scrollTop;
        const cursorPosition = this.selectionStart;
        
        // Use requestAnimationFrame to wait for the DOM to update
        requestAnimationFrame(() => {
            // Adjust editor height smoothly without jumping
            if (typeof adjustTextareaHeight === 'function') {
                adjustTextareaHeight(this);
                // Restore cursor and scroll position
                this.scrollTop = scrollTop;
                this.setSelectionRange(cursorPosition, cursorPosition);
            } else {
                // Fall back to original adjustment if new function isn't available
                adjustEditorHeight();
            }
        });
    });
    
    // Save content
	function submitContent() {
        // Get form values
        const content = editor.value;
        const title = document.getElementById('title').value;
        const tags = document.getElementById('tags').value;
        const encrypt = document.getElementById('encrypt').value;
        
        // Get base auth type
        const baseAuthType = document.querySelector('input[name="base_auth_type"]:checked').value;
        
        // Get special permissions
        const diaryPermission = document.getElementById('diary_permission').checked;
        const encryptPermission = document.getElementById('encrypt_permission').checked;
        
        // 添加调试日志
        console.log('权限收集调试:', {
            baseAuthType,
            diaryPermission,
            encryptPermission,
            encryptKey: encrypt ? '[有密码]' : '[无密码]'
        });
        
        // Build combined auth type string
        let authTypeArray = [baseAuthType];
        if (diaryPermission) authTypeArray.push('diary');
        if (encryptPermission) authTypeArray.push('encrypt');
        
        const authType = authTypeArray.join(',');
        
        console.log('最终权限字符串:', authType);
        
        // Validate title
        if (!title.trim()) {
            showToast('请输入文章标题', 'error');
            return;
        }
        
        // Validate permissions using PermissionManager
        if (window.PermissionManager && !window.PermissionManager.validate()) {
            return;
        }
        
        // Show saving status with permission summary
        const permissionSummary = window.PermissionManager ? window.PermissionManager.getSummary() : '';
        showToast(`正在保存博客 (${permissionSummary})...`, 'info');
        
        // Handle encryption if needed
        let finalContent = content;
        let encryptFlag = '';
        
        if (encryptPermission && encrypt.length > 0) {
            finalContent = aesEncrypt(content, encrypt);
            encryptFlag = 'use_aes_cbc';
        }
        
        // Send data to server
        const xhr = new XMLHttpRequest();
		xhr.onreadystatechange = function() {
            if (xhr.readyState == 4) {
                if (xhr.status == 200) {
                    showToast(`博客保存成功！权限：${permissionSummary}`, 'success');
                } else {
                    showToast('保存失败: ' + xhr.responseText, 'error');
                }
            }
        };
        
        const formData = new FormData();
		formData.append('title', title);
        formData.append('content', finalContent);
        formData.append('authtype', authType);
		formData.append('tags', tags);
        formData.append('encrypt', encryptFlag);
		xhr.open('POST', '/save', true);
		xhr.send(formData);
	}