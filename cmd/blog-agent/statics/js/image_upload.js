(() => {
    const editor = document.getElementById('editor') || document.getElementById('editor-inner');
    if (!editor) return;
    const maxBytes = 10 * 1024 * 1024;
    const allowedExtensions = new Set(['txt', 'md', 'html', 'htm', 'csv', 'json', 'xml', 'yaml', 'yml', 'zip', 'pdf']);
    const fileInput = document.getElementById('image-file-input');
    const uploadButton = document.getElementById('btn-upload-image');
    let markerSequence = 0;
    let uploading = false;

    function showMessage(message, type) {
        if (typeof showToast === 'function') showToast(message, type || 'info');
    }

    function refreshPreview() {
        if (typeof mdRender === 'function') mdRender(editor.value);
        editor.dispatchEvent(new Event('input', { bubbles: true }));
    }

    function replaceMarker(marker, replacement) {
        editor.value = editor.value.replace(marker, replacement);
        refreshPreview();
    }

    function isSupportedFile(file) {
        if (!file) return false;
        if (file.type.startsWith('image/')) return true;
        const extension = (file.name.split('.').pop() || '').toLowerCase();
        return allowedExtensions.has(extension);
    }

    function escapeMarkdownLabel(label) {
        return label.replace(/[\r\n]/g, '').replace(/([\\[\]])/g, '\\$1');
    }

    async function uploadFile(file) {
        if (!isSupportedFile(file)) {
            showMessage('不支持该文件格式', 'error');
            return;
        }
        if (file.size > maxBytes) {
            showMessage('文件不能超过 10MB', 'error');
            return;
        }
        const isImage = file.type.startsWith('image/');
        const marker = `${isImage ? '!' : ''}[文件上传中 ${Date.now()}-${++markerSequence}]()`;
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        editor.value = editor.value.slice(0, start) + marker + editor.value.slice(end);
        editor.focus();
        editor.setSelectionRange(start + marker.length, start + marker.length);
        refreshPreview();
        showMessage(`正在上传 ${file.name || '文件'}…`);

        const form = new FormData();
        form.append('file', file, file.name || 'pasted-image.png');
        try {
            const response = await fetch('/api/media/upload', {
                method: 'POST',
                credentials: 'same-origin',
                body: form
            });
            const responseText = await response.text();
            let result = {};
            try {
                result = JSON.parse(responseText);
            } catch (_) {
                // 兼容后端 http.Error 返回的纯文本错误。
            }
            if (!response.ok || !result.url) throw new Error(result.error || responseText.trim() || '上传失败');
            const label = escapeMarkdownLabel(result.is_image ? (result.alt || '图片') : (result.name || '附件'));
            replaceMarker(marker, `${result.is_image ? '!' : ''}[${label}](${result.url})`);
            showMessage(`${result.is_image ? '图片' : '文件'}已插入正文`, 'success');
        } catch (error) {
            replaceMarker(marker, '');
            showMessage(`文件上传失败：${error.message}`, 'error');
        }
    }

    async function uploadSelectedFiles(files) {
        if (uploading) return;
        const selectedFiles = [...files].filter(isSupportedFile);
        if (!selectedFiles.length) {
            showMessage('请选择支持的小文件', 'error');
            return;
        }
        uploading = true;
        if (uploadButton) {
            uploadButton.disabled = true;
            uploadButton.setAttribute('aria-busy', 'true');
            uploadButton.textContent = selectedFiles.length > 1 ? `上传 1/${selectedFiles.length}` : '上传中…';
        }
        for (let index = 0; index < selectedFiles.length; index += 1) {
            if (uploadButton && selectedFiles.length > 1) {
                uploadButton.textContent = `上传 ${index + 1}/${selectedFiles.length}`;
            }
            await uploadFile(selectedFiles[index]);
        }
        uploading = false;
        if (uploadButton) {
            uploadButton.disabled = false;
            uploadButton.removeAttribute('aria-busy');
            uploadButton.textContent = '上传文件';
        }
        if (fileInput) fileInput.value = '';
    }

    if (uploadButton && fileInput) {
        uploadButton.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', () => uploadSelectedFiles(fileInput.files));
    }

    editor.addEventListener('paste', (event) => {
        const item = [...event.clipboardData.items].find((candidate) => candidate.type.startsWith('image/'));
        if (!item) return;
        event.preventDefault();
        uploadFile(item.getAsFile());
    });
    editor.addEventListener('dragenter', (event) => {
        if ([...event.dataTransfer.types].includes('Files')) {
            event.preventDefault();
            editor.classList.add('image-drag-over');
        }
    });
    editor.addEventListener('dragover', (event) => event.preventDefault());
    editor.addEventListener('dragleave', () => editor.classList.remove('image-drag-over'));
    editor.addEventListener('drop', (event) => {
        event.preventDefault();
        editor.classList.remove('image-drag-over');
        uploadSelectedFiles(event.dataTransfer.files);
    });
})();
