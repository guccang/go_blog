const editor = document.getElementById('editor');
const md = document.getElementById('md');
const editorWrapper = document.getElementById('editor-wrapper');
const previewWrapper = document.getElementById('preview-wrapper');
const wordCount = document.getElementById('word-count');
const viewModeButtons = document.querySelectorAll('.view-mode-btn');
let viewState = 'split';

function updatePreview() {
    mdRender(editor.value);
    const count = editor.value.trim() ? editor.value.trim().length : 0;
    wordCount.textContent = `${count} 字符`;
}

function applyViewState() {
    const editorVisible = viewState !== 'preview-only';
    const previewVisible = viewState !== 'editor-only';
    editorWrapper.classList.toggle('hidden', !editorVisible);
    previewWrapper.classList.toggle('hidden', !previewVisible);
    editorWrapper.classList.toggle('fullscreen', editorVisible && !previewVisible);
    previewWrapper.classList.toggle('fullscreen', previewVisible && !editorVisible);
    viewModeButtons.forEach((button) => button.classList.toggle('active', button.dataset.view === viewState));
    if (previewVisible) updatePreview();
}

viewModeButtons.forEach((button) => button.addEventListener('click', () => {
    viewState = button.dataset.view;
    applyViewState();
}));

function insertMarkdown(before, after) {
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    const selected = editor.value.substring(start, end);
    editor.value = editor.value.substring(0, start) + before + selected + after + editor.value.substring(end);
    editor.focus();
    editor.setSelectionRange(start + before.length, start + before.length + selected.length);
    updatePreview();
}

document.getElementById('btn-bold').addEventListener('click', () => insertMarkdown('**', '**'));
document.getElementById('btn-italic').addEventListener('click', () => insertMarkdown('*', '*'));
document.getElementById('btn-heading').addEventListener('click', () => insertMarkdown('# ', ''));
document.getElementById('btn-link').addEventListener('click', () => insertMarkdown('[', '](https://)'));
document.getElementById('btn-image').addEventListener('click', () => insertMarkdown('![图片说明](', ')'));
document.getElementById('btn-code').addEventListener('click', () => insertMarkdown('```\n', '\n```'));
document.getElementById('btn-list').addEventListener('click', () => insertMarkdown('- ', ''));
document.getElementById('btn-quote').addEventListener('click', () => insertMarkdown('> ', ''));
editor.addEventListener('input', updatePreview);

function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    document.getElementById('toast-container').appendChild(toast);
    window.setTimeout(() => toast.remove(), 3500);
}

function submitContent() {
    const title = document.getElementById('title').value.trim();
    const tags = document.getElementById('tags').value;
    const encrypt = document.getElementById('encrypt').value;
    const baseAuth = document.querySelector('input[name="base_auth_type"]:checked');
    const diaryPermission = document.getElementById('diary_permission').checked;
    const encryptPermission = document.getElementById('encrypt_permission').checked;
    if (!title) { showToast('请输入博客标题', 'error'); return; }
    if (window.PermissionManager && !window.PermissionManager.validate()) return;
    const authTypes = [baseAuth ? baseAuth.value : 'private'];
    if (diaryPermission) authTypes.push('diary');
    if (encryptPermission) authTypes.push('encrypt');
    const formData = new FormData();
    formData.append('title', title);
    formData.append('content', encryptPermission && encrypt ? aesEncrypt(editor.value, encrypt) : editor.value);
    formData.append('authtype', authTypes.join(','));
    formData.append('tags', tags);
    formData.append('encrypt', encryptPermission && encrypt ? 'use_aes_cbc' : '');
    const request = new XMLHttpRequest();
    request.onreadystatechange = () => {
        if (request.readyState !== 4) return;
        showToast(request.status === 200 ? '博客已保存' : `保存失败：${request.responseText}`, request.status === 200 ? 'success' : 'error');
    };
    showToast('正在保存…');
    request.open('POST', '/save', true);
    request.send(formData);
}

updatePreview();
editor.focus();
