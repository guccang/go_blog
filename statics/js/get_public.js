    // Helper function to split strings
    function split(str, separator) {
        return str.split(separator);
    }

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

    function onShowComment() {
        const btn = document.getElementById('comment-show');
        const comments = document.getElementById('comments');
        const divComment = document.getElementById('div-comment');
        
        if (btn.innerText === '收起评论') {
            comments.classList.add('hide');
            divComment.classList.add('hide');
            btn.innerText = '显示评论';
        } else {
            comments.classList.remove('hide');
            divComment.classList.remove('hide');
            btn.innerText = '收起评论';
        }
    }

    function onCommitComment() {
        const title = document.getElementById('title').innerText;
        const comment = document.getElementById('input-comment').value;
        const owner = document.getElementById('input-owner').value;
        const mail = document.getElementById('input-mail').value;
        const pwd = document.getElementById('input-pwd').value; // 不需要MD5哈希，使用原始密码
        
        // Validate form
        if (!owner || !comment) {
            showToast('请填写用户名和评论内容', 'error');
            return;
        }

        // Show loading status
        showToast('正在提交评论...', 'info');
        
        // Send data
        const xhr = new XMLHttpRequest();
		xhr.onreadystatechange = function() {
            if (xhr.readyState == 4) {
                if (xhr.status == 200) {
                    // 尝试从响应中提取会话ID并保存
                    try {
                        const response = JSON.parse(xhr.responseText);
                        if (response.session_id) {
                            showToast('评论提交成功！已保存身份信息', 'success');
                        }
                    } catch (e) {
                        // 响应不是JSON格式，说明是普通文本响应
                        showToast('评论提交成功！', 'success');
                    }
                    
                    // Clear form
                    document.getElementById('input-comment').value = '';
                    document.getElementById('input-owner').value = '';
                    document.getElementById('input-mail').value = '';
                    document.getElementById('input-pwd').value = '';
                    
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

    // Initialize editor and preview on page load
    window.onload = function() {
        mdRender(editor.value);
        checkTime();
        // hide sidebar
        sidebar.classList.add('hide-sidebar');
        container.classList.add('hide-sidebar');
    }