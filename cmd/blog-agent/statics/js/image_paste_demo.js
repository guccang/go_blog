(() => {
    const editor = document.getElementById('demo-editor');
    const preview = document.getElementById('preview');
    const state = document.getElementById('paste-state');
    const output = document.getElementById('markdown-output');
    let currentObjectURL = '';

    function setState(text, active) {
        state.textContent = text;
        state.style.background = active ? '#e5f3ef' : '#f8eee9';
        state.style.color = active ? '#287d74' : '#dc704b';
    }
    function insertAtCursor(text) {
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        editor.value = editor.value.slice(0, start) + text + editor.value.slice(end);
        editor.selectionStart = editor.selectionEnd = start + text.length;
    }
    function handleImage(image) {
        if (!image || !image.type.startsWith('image/')) return;
        setState('正在模拟上传…', true);
        preview.textContent = '正在处理截图…';
        const pending = '![图片上传中…]()';
        insertAtCursor(pending);
        window.setTimeout(() => {
            if (currentObjectURL) URL.revokeObjectURL(currentObjectURL);
            currentObjectURL = URL.createObjectURL(image);
            const extension = image.type.split('/')[1] || 'png';
            const markdown = `![截图](本地上传后的图片地址/${Date.now()}.${extension})`;
            editor.value = editor.value.replace(pending, markdown);
            output.textContent = markdown;
            const img = document.createElement('img');
            img.src = currentObjectURL;
            img.alt = '刚粘贴的截图预览';
            preview.replaceChildren(img);
            setState('已插入图片', true);
        }, 500);
    }
    editor.addEventListener('paste', (event) => {
        const item = [...event.clipboardData.items].find((candidate) => candidate.type.startsWith('image/'));
        if (!item) return;
        event.preventDefault();
        handleImage(item.getAsFile());
    });
    editor.addEventListener('dragenter', (event) => {
        event.preventDefault();
        editor.classList.add('drag-over');
    });
    editor.addEventListener('dragover', (event) => event.preventDefault());
    editor.addEventListener('dragleave', (event) => {
        if (event.relatedTarget !== editor) editor.classList.remove('drag-over');
    });
    editor.addEventListener('drop', (event) => {
        event.preventDefault();
        editor.classList.remove('drag-over');
        handleImage([...event.dataTransfer.files].find((file) => file.type.startsWith('image/')));
    });
    editor.focus();
})();
