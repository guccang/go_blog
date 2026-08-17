(() => {
  const textInput = document.getElementById('clipboardText');
  const fileInput = document.getElementById('clipboardImages');
  const draftImages = document.getElementById('draftImages');
  const saveButton = document.getElementById('saveClipboard');
  const refreshButton = document.getElementById('refreshClipboard');
  const itemsContainer = document.getElementById('clipboardItems');
  const hint = document.getElementById('composerHint');
  const toast = document.getElementById('clipboardToast');
  let pendingImages = [];
  let toastTimer = null;

  function showToast(message) {
    toast.textContent = message;
    toast.classList.add('show');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toast.classList.remove('show'), 2400);
  }

  async function request(url, options = {}) {
    const response = await fetch(url, options);
    const raw = await response.text();
    let body = null;
    try { body = raw ? JSON.parse(raw) : null; } catch { body = null; }
    if (!response.ok) throw new Error(body?.message || raw || `HTTP ${response.status}`);
    return body;
  }

  function addImages(files) {
    const accepted = [...files].filter(file => file.type.startsWith('image/'));
    for (const file of accepted) {
      if (pendingImages.length >= 8) {
        showToast('每条记录最多支持 8 张图片');
        break;
      }
      if (file.size > 10 * 1024 * 1024) {
        showToast(`${file.name || '图片'}超过 10MB`);
        continue;
      }
      pendingImages.push({ file, preview: URL.createObjectURL(file) });
    }
    renderDraftImages();
  }

  function renderDraftImages() {
    draftImages.replaceChildren();
    draftImages.hidden = pendingImages.length === 0;
    pendingImages.forEach((entry, index) => {
      const wrapper = document.createElement('div');
      wrapper.className = 'draft-image';
      const image = document.createElement('img');
      image.src = entry.preview;
      image.alt = `待上传图片 ${index + 1}`;
      const remove = document.createElement('button');
      remove.type = 'button';
      remove.textContent = '×';
      remove.setAttribute('aria-label', `移除第 ${index + 1} 张图片`);
      remove.addEventListener('click', () => {
        URL.revokeObjectURL(entry.preview);
        pendingImages.splice(index, 1);
        renderDraftImages();
      });
      wrapper.append(image, remove);
      draftImages.append(wrapper);
    });
  }

  function insertPastedText(text) {
    if (!text) return;
    const start = textInput.selectionStart;
    const end = textInput.selectionEnd;
    textInput.value = textInput.value.slice(0, start) + text + textInput.value.slice(end);
    const position = start + text.length;
    textInput.setSelectionRange(position, position);
  }

  textInput.addEventListener('paste', event => {
    const files = [...(event.clipboardData?.items || [])]
      .filter(item => item.kind === 'file' && item.type.startsWith('image/'))
      .map(item => item.getAsFile())
      .filter(Boolean);
    if (!files.length) return;
    event.preventDefault();
    insertPastedText(event.clipboardData.getData('text/plain'));
    addImages(files);
    hint.textContent = `已接收 ${files.length} 张图片，保存后其他设备即可读取。`;
  });

  fileInput.addEventListener('change', () => {
    addImages(fileInput.files || []);
    fileInput.value = '';
  });

  async function uploadImage(file) {
    const form = new FormData();
    form.append('image', file, file.name || 'clipboard.png');
    const result = await request('/api/media/upload', { method: 'POST', body: form });
    return result.url.split('/').pop();
  }

  saveButton.addEventListener('click', async () => {
    const text = textInput.value.trim();
    if (!text && !pendingImages.length) {
      showToast('请先粘贴文字或图片');
      textInput.focus();
      return;
    }
    saveButton.disabled = true;
    saveButton.textContent = pendingImages.length ? '正在传输图片…' : '正在保存…';
    try {
      const imageIDs = [];
      for (const entry of pendingImages) imageIDs.push(await uploadImage(entry.file));
      await request('/api/clipboard', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text, image_ids: imageIDs }),
      });
      pendingImages.forEach(entry => URL.revokeObjectURL(entry.preview));
      pendingImages = [];
      textInput.value = '';
      hint.textContent = '每条最多 8 张图片，单张不超过 10MB。';
      renderDraftImages();
      showToast('已保存到内贴板');
      await loadItems();
    } catch (error) {
      showToast(error.message);
    } finally {
      saveButton.disabled = false;
      saveButton.textContent = '保存到内贴板';
    }
  });

  function formatTime(value) {
    const date = new Date(value.replace(' ', 'T'));
    if (Number.isNaN(date.getTime())) return value;
    return new Intl.DateTimeFormat('zh-CN', {
      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
    }).format(date);
  }

  function buildCard(item) {
    const card = document.createElement('article');
    card.className = 'clipboard-card';
    card.dataset.id = item.id;

    const meta = document.createElement('div');
    meta.className = 'card-meta';
    meta.textContent = formatTime(item.created_at);
    card.append(meta);

    if (item.text) {
      const text = document.createElement('p');
      text.className = 'card-text';
      text.textContent = item.text;
      card.append(text);
    }

    if (item.images?.length) {
      const gallery = document.createElement('div');
      gallery.className = 'card-images';
      item.images.forEach((source, index) => {
        const link = document.createElement('a');
        link.href = source;
        link.target = '_blank';
        link.rel = 'noopener';
        const image = document.createElement('img');
        image.src = source;
        image.alt = `贴板图片 ${index + 1}`;
        image.loading = 'lazy';
        link.append(image);
        gallery.append(link);
      });
      card.append(gallery);
    }

    const actions = document.createElement('div');
    actions.className = 'card-actions';
    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'delete-item';
    remove.textContent = '删除';
    remove.addEventListener('click', () => deleteItem(item, card));
    const copy = document.createElement('button');
    copy.type = 'button';
    copy.className = 'copy-item';
    copy.textContent = item.images?.length ? '复制文字和图片' : '复制文字';
    copy.addEventListener('click', () => copyItem(item));
    actions.append(remove, copy);
    card.append(actions);
    return card;
  }

  async function loadItems(silent = false) {
    if (!silent) itemsContainer.innerHTML = '<div class="clipboard-empty">正在读取内贴板…</div>';
    try {
      const result = await request('/api/clipboard');
      itemsContainer.replaceChildren();
      if (!result.data?.length) {
        const empty = document.createElement('div');
        empty.className = 'clipboard-empty';
        empty.textContent = '还没有记录。在上方粘贴第一条内容。';
        itemsContainer.append(empty);
        return;
      }
      result.data.forEach(item => itemsContainer.append(buildCard(item)));
    } catch (error) {
      if (!silent) {
        itemsContainer.innerHTML = '<div class="clipboard-empty">读取失败，请刷新重试。</div>';
        showToast(error.message);
      }
    }
  }

  async function imageAsPNG(source) {
    const response = await fetch(source);
    if (!response.ok) throw new Error('图片读取失败');
    const sourceBlob = await response.blob();
    if (sourceBlob.type === 'image/png') return sourceBlob;
    const bitmap = await createImageBitmap(sourceBlob);
    const canvas = document.createElement('canvas');
    canvas.width = bitmap.width;
    canvas.height = bitmap.height;
    canvas.getContext('2d').drawImage(bitmap, 0, 0);
    bitmap.close();
    return new Promise((resolve, reject) => canvas.toBlob(
      blob => blob ? resolve(blob) : reject(new Error('图片转换失败')), 'image/png'));
  }

  async function copyItem(item) {
    try {
      if (item.images?.length && window.ClipboardItem && navigator.clipboard?.write) {
        const png = await imageAsPNG(item.images[0]);
        const content = { 'image/png': png };
        if (item.text) content['text/plain'] = new Blob([item.text], { type: 'text/plain' });
        await navigator.clipboard.write([new ClipboardItem(content)]);
        showToast(item.images.length > 1 ? '已复制文字和第一张图片' : '已复制文字和图片');
        return;
      }
      if (navigator.clipboard?.writeText && !item.images?.length) {
        await navigator.clipboard.writeText(item.text || '');
        showToast('已复制文字');
        return;
      }
      if (await legacyCopyItem(item)) {
        showToast(item.images?.length ? '已复制文字和图片' : '已复制文字');
        return;
      }
      throw new Error('copy unavailable');
    } catch {
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(item.text || item.images?.join('\n') || '');
        } else if (!await legacyCopyItem({ text: item.text || item.images?.join('\n') || '', images: [] })) {
          throw new Error('copy unavailable');
        }
        showToast(item.images?.length ? '图片复制受限，已复制文字或图片链接' : '已复制文字');
      } catch {
        showToast('浏览器未允许复制，请长按或右键复制');
      }
    }
  }

  async function legacyCopyItem(item) {
    const content = document.createElement('div');
    content.contentEditable = 'true';
    content.setAttribute('aria-hidden', 'true');
    Object.assign(content.style, {
      position: 'fixed', left: '-10000px', top: '0', opacity: '0', pointerEvents: 'none',
    });
    if (item.text) {
      const paragraph = document.createElement('p');
      paragraph.textContent = item.text;
      content.append(paragraph);
    }
    const imageLoads = (item.images || []).map(source => new Promise(resolve => {
      const image = document.createElement('img');
      image.addEventListener('load', resolve, { once: true });
      image.addEventListener('error', resolve, { once: true });
      image.src = source;
      content.append(image);
    }));
    document.body.append(content);
    await Promise.all(imageLoads);
    const selection = window.getSelection();
    const range = document.createRange();
    range.selectNodeContents(content);
    selection.removeAllRanges();
    selection.addRange(range);
    let copied = false;
    try { copied = document.execCommand('copy'); } catch { copied = false; }
    selection.removeAllRanges();
    content.remove();
    return copied;
  }

  async function deleteItem(item, card) {
    if (!confirm('删除这条内贴板记录？')) return;
    try {
      await request(`/api/clipboard?id=${encodeURIComponent(item.id)}`, { method: 'DELETE' });
      card.remove();
      if (!itemsContainer.querySelector('.clipboard-card')) await loadItems();
      showToast('记录已删除');
    } catch (error) {
      showToast(error.message);
    }
  }

  refreshButton.addEventListener('click', () => loadItems());
  loadItems();
  setInterval(() => {
    if (document.visibilityState === 'visible') loadItems(true);
  }, 10000);
})();
