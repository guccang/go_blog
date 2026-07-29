(() => {
    const editor = document.getElementById('editor') || document.getElementById('editor-inner');
    if (!editor) return;
    const maxBytes = 10 * 1024 * 1024;
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

    async function uploadImage(file) {
        if (!file || !file.type.startsWith('image/')) {
            showMessage('请选择图片文件', 'error');
            return;
        }
        if (file.size > maxBytes) {
            showMessage('图片不能超过 10MB', 'error');
            return;
        }
        const marker = `![图片上传中 ${Date.now()}-${++markerSequence}]()`;
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
            const response = await fetch('/api/media/upload', {
                method: 'POST',
                credentials: 'same-origin',
                body: form
            });
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

    async function uploadSelectedImages(files) {
        if (uploading) return;
        const images = [...files].filter((file) => file.type.startsWith('image/'));
        if (!images.length) {
            showMessage('请选择图片文件', 'error');
            return;
        }
        uploading = true;
        if (uploadButton) {
            uploadButton.disabled = true;
            uploadButton.setAttribute('aria-busy', 'true');
            uploadButton.textContent = images.length > 1 ? `上传 1/${images.length}` : '上传中…';
        }
        for (let index = 0; index < images.length; index += 1) {
            if (uploadButton && images.length > 1) {
                uploadButton.textContent = `上传 ${index + 1}/${images.length}`;
            }
            await uploadImage(images[index]);
        }
        uploading = false;
        if (uploadButton) {
            uploadButton.disabled = false;
            uploadButton.removeAttribute('aria-busy');
            uploadButton.textContent = '上传图片';
        }
        if (fileInput) fileInput.value = '';
    }

    if (uploadButton && fileInput) {
        uploadButton.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', () => uploadSelectedImages(fileInput.files));
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
        uploadSelectedImages(event.dataTransfer.files);
    });
})();
