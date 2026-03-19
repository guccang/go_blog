(function() {
    'use strict';

    let TOKEN = localStorage.getItem('bridge_token') || '';
    let selectedFile = null;
    let currentSSE = null;

    // ========================= Token 管理 =========================

    const tokenModal = document.getElementById('token-modal');
    const tokenInput = document.getElementById('token-input');
    const tokenSave = document.getElementById('token-save');
    const btnLogout = document.getElementById('btn-logout');

    function showTokenModal() {
        tokenModal.classList.remove('hidden');
        tokenInput.value = '';
        tokenInput.focus();
    }

    function hideTokenModal() {
        tokenModal.classList.add('hidden');
    }

    if (!TOKEN) {
        showTokenModal();
    } else {
        hideTokenModal();
        init();
    }

    tokenSave.addEventListener('click', function() {
        const val = tokenInput.value.trim();
        if (!val) return;
        TOKEN = val;
        localStorage.setItem('bridge_token', TOKEN);
        hideTokenModal();
        init();
    });

    tokenInput.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') tokenSave.click();
    });

    btnLogout.addEventListener('click', function() {
        TOKEN = '';
        localStorage.removeItem('bridge_token');
        showTokenModal();
    });

    // ========================= API 调用 =========================

    function apiURL(path) {
        return path + (path.includes('?') ? '&' : '?') + 'token=' + encodeURIComponent(TOKEN);
    }

    async function apiFetch(path, opts) {
        opts = opts || {};
        opts.headers = opts.headers || {};
        opts.headers['Authorization'] = 'Bearer ' + TOKEN;
        const resp = await fetch(path, opts);
        if (resp.status === 401) {
            showTokenModal();
            throw new Error('unauthorized');
        }
        return resp;
    }

    // ========================= 初始化 =========================

    function init() {
        loadPackages();
        loadDeploys();
    }

    // ========================= 文件上传 =========================

    const dropZone = document.getElementById('drop-zone');
    const fileInput = document.getElementById('file-input');
    const btnUploadDeploy = document.getElementById('btn-upload-deploy');
    const btnUploadOnly = document.getElementById('btn-upload-only');
    const uploadStatus = document.getElementById('upload-status');

    dropZone.addEventListener('click', function() { fileInput.click(); });

    dropZone.addEventListener('dragover', function(e) {
        e.preventDefault();
        dropZone.classList.add('dragover');
    });

    dropZone.addEventListener('dragleave', function() {
        dropZone.classList.remove('dragover');
    });

    dropZone.addEventListener('drop', function(e) {
        e.preventDefault();
        dropZone.classList.remove('dragover');
        if (e.dataTransfer.files.length > 0) {
            selectFile(e.dataTransfer.files[0]);
        }
    });

    fileInput.addEventListener('change', function() {
        if (fileInput.files.length > 0) {
            selectFile(fileInput.files[0]);
        }
    });

    function selectFile(file) {
        if (!file.name.endsWith('.zip')) {
            uploadStatus.textContent = '只接受 .zip 文件';
            return;
        }
        selectedFile = file;
        dropZone.classList.add('has-file');
        dropZone.querySelector('p').textContent = file.name + ' (' + formatSize(file.size) + ')';
        btnUploadDeploy.disabled = false;
        btnUploadOnly.disabled = false;
        uploadStatus.textContent = '';
    }

    btnUploadOnly.addEventListener('click', function() { doUpload(false); });
    btnUploadDeploy.addEventListener('click', function() { doUpload(true); });

    async function doUpload(andDeploy) {
        if (!selectedFile) return;

        btnUploadDeploy.disabled = true;
        btnUploadOnly.disabled = true;
        uploadStatus.textContent = '上传中...';

        const form = new FormData();
        form.append('file', selectedFile);

        try {
            const resp = await apiFetch('/api/upload', { method: 'POST', body: form });
            const data = await resp.json();
            if (data.error) {
                uploadStatus.textContent = '上传失败: ' + data.error;
                return;
            }

            uploadStatus.textContent = '上传成功: ' + data.filename;
            loadPackages();

            if (andDeploy) {
                const targetDir = document.getElementById('target-dir').value.trim();
                const script = document.getElementById('script-name').value.trim();
                if (!targetDir) {
                    uploadStatus.textContent = '请填写目标目录';
                    return;
                }
                await triggerDeploy(data.filename, targetDir, script);
            }
        } catch (e) {
            uploadStatus.textContent = '上传失败: ' + e.message;
        } finally {
            btnUploadDeploy.disabled = false;
            btnUploadOnly.disabled = false;
        }
    }

    // ========================= 包列表 =========================

    document.getElementById('btn-refresh-pkgs').addEventListener('click', loadPackages);

    async function loadPackages() {
        try {
            const resp = await apiFetch('/api/packages');
            const pkgs = await resp.json();
            const tbody = document.querySelector('#pkg-table tbody');
            tbody.innerHTML = '';

            pkgs.forEach(function(pkg) {
                const tr = document.createElement('tr');
                tr.innerHTML =
                    '<td>' + esc(pkg.name) + '</td>' +
                    '<td>' + formatSize(pkg.size) + '</td>' +
                    '<td>' + formatTime(pkg.mod_time) + '</td>' +
                    '<td><button class="btn-deploy" data-name="' + esc(pkg.name) + '">部署</button></td>';
                tbody.appendChild(tr);
            });

            tbody.querySelectorAll('.btn-deploy').forEach(function(btn) {
                btn.addEventListener('click', function() {
                    const filename = btn.dataset.name;
                    const targetDir = document.getElementById('target-dir').value.trim();
                    const script = document.getElementById('script-name').value.trim();
                    if (!targetDir) {
                        alert('请先填写目标目录');
                        return;
                    }
                    triggerDeploy(filename, targetDir, script);
                });
            });
        } catch (e) {
            // ignore
        }
    }

    // ========================= 部署 =========================

    async function triggerDeploy(filename, targetDir, script) {
        try {
            const resp = await apiFetch('/api/deploy', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    filename: filename,
                    target_dir: targetDir,
                    script: script || 'publish.sh'
                })
            });
            const data = await resp.json();
            if (data.error) {
                alert('部署失败: ' + data.error);
                return;
            }
            loadDeploys();
            showLogs(data.deploy_id);
        } catch (e) {
            alert('部署请求失败: ' + e.message);
        }
    }

    // ========================= 部署历史 =========================

    document.getElementById('btn-refresh-deploys').addEventListener('click', loadDeploys);

    async function loadDeploys() {
        try {
            const resp = await apiFetch('/api/deploys');
            const deploys = await resp.json();
            const tbody = document.querySelector('#deploy-table tbody');
            tbody.innerHTML = '';

            deploys.forEach(function(d) {
                const tr = document.createElement('tr');
                const statusClass = 'status-' + d.status;
                tr.innerHTML =
                    '<td>' + esc(d.id) + '</td>' +
                    '<td>' + esc(d.filename) + '</td>' +
                    '<td><span class="' + statusClass + '">' + esc(d.status) + '</span></td>' +
                    '<td>' + (d.duration || '-') + '</td>' +
                    '<td><button class="btn-log" data-id="' + esc(d.id) + '">日志</button></td>';
                tbody.appendChild(tr);
            });

            tbody.querySelectorAll('.btn-log').forEach(function(btn) {
                btn.addEventListener('click', function() {
                    showLogs(btn.dataset.id);
                });
            });
        } catch (e) {
            // ignore
        }
    }

    // ========================= 日志面板 =========================

    const logSection = document.getElementById('log-section');
    const logPanel = document.getElementById('log-panel');
    const logDeployId = document.getElementById('log-deploy-id');

    document.getElementById('btn-close-log').addEventListener('click', function() {
        closeLogs();
    });

    function closeLogs() {
        logSection.style.display = 'none';
        logPanel.innerHTML = '';
        if (currentSSE) {
            currentSSE.close();
            currentSSE = null;
        }
    }

    function showLogs(deployId) {
        closeLogs();
        logSection.style.display = 'block';
        logDeployId.textContent = deployId;

        const url = apiURL('/api/deploy/' + deployId + '/logs');
        currentSSE = new EventSource(url);

        currentSSE.onmessage = function(e) {
            try {
                const entry = JSON.parse(e.data);
                appendLog(entry);
            } catch (err) {
                // ignore
            }
        };

        currentSSE.onerror = function() {
            currentSSE.close();
            currentSSE = null;
            loadDeploys();
        };
    }

    function appendLog(entry) {
        const line = document.createElement('div');
        const timeSpan = '<span class="log-time">[' + esc(entry.time) + ']</span> ';

        if (entry.level === 'error') {
            line.innerHTML = timeSpan + '<span class="log-error">' + esc(entry.text) + '</span>';
        } else if (entry.level === 'done') {
            line.innerHTML = timeSpan + '<span class="log-done">' + esc(entry.text) + '</span>';
        } else {
            line.innerHTML = timeSpan + esc(entry.text);
        }

        logPanel.appendChild(line);
        logPanel.scrollTop = logPanel.scrollHeight;
    }

    // ========================= 工具函数 =========================

    function formatSize(bytes) {
        if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB';
        if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB';
        return bytes + ' B';
    }

    function formatTime(t) {
        if (!t) return '-';
        var d = new Date(t);
        return d.toLocaleString('zh-CN');
    }

    function esc(s) {
        if (!s) return '';
        var div = document.createElement('div');
        div.textContent = s;
        return div.innerHTML;
    }
})();
