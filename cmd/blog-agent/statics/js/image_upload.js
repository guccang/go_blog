(() => {
    const editor = document.getElementById('editor') || document.getElementById('editor-inner');
    if (!editor) return;
    const maxBytes = 10 * 1024 * 1024;

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

    async function uploadImage(file) {
        if (!file || !file.type.startsWith('image/')) return;
        if (file.size > maxBytes) {
            showMessage('图片不能超过 10MB', 'error');
            return;
        }
        const marker = `![图片上传中 ${Date.now()}]()`;
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        editor.value = editor.value.slice(0, start) + marker + editor.value.slice(end);
        editor.focus();
        editor.setSelectionRange(start + marker.length, start + marker.length);
        refreshPreview();
        showMessage('正在上传图片…');

        const form = new FormData();
        form.append('image', file, file.name || 'pasted-image.png');
        try {
            const response = await fetch('/api/media/upload', { method: 'POST', body: form });
            const result = await response.json().catch(() => ({}));
            if (!response.ok || !result.url) throw new Error(result.error || '上传失败');
            const alt = (result.alt || '图片').replace(/[\[\]\n]/g, '');
            replaceMarker(marker, `![${alt}](${result.url})`);
            showMessage('图片已插入正文', 'success');
        } catch (error) {
            replaceMarker(marker, '');
            showMessage(`图片上传失败：${error.message}`, 'error');
        }
    }

    editor.addEventListener('paste', (event) => {
        const item = [...event.clipboardData.items].find((candidate) => candidate.type.startsWith('image/'));
        if (!item) return;
        event.preventDefault();
        uploadImage(item.getAsFile());
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
        uploadImage([...event.dataTransfer.files].find((file) => file.type.startsWith('image/')));
    });
})();
