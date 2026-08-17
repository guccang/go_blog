(() => {
    const state = {
        products: [], query: '', type: '', tag: '', editingID: '', toastTimer: null,
        researchSources: [], confidence: {}, evidence: {}, lastResearchedAt: '', dialogMode: 'edit',
        scanJobs: [], scanPollTimer: null
    };
    const elements = {
        grid: document.getElementById('productGrid'), empty: document.getElementById('productEmpty'),
        filterEmpty: document.getElementById('filterEmpty'), count: document.getElementById('productCount'),
        search: document.getElementById('productSearch'), type: document.getElementById('typeFilter'),
        tags: document.getElementById('tagFilters'), dialog: document.getElementById('productDialog'),
        form: document.getElementById('productForm'), dialogTitle: document.getElementById('dialogTitle'),
        dialogEyebrow: document.getElementById('dialogEyebrow'), draftNotice: document.getElementById('draftNotice'),
        deleteButton: document.getElementById('deleteProductButton'), saveButton: document.getElementById('saveProductButton'),
        formStatus: document.getElementById('formStatus'), scanForm: document.getElementById('scanForm'),
        scanURL: document.getElementById('scanURL'), scanButton: document.getElementById('scanButton'),
        scanStatus: document.getElementById('scanStatus'), scanConsole: document.getElementById('scanConsole'),
        scanJobs: document.getElementById('scanJobs'), scanJobList: document.getElementById('scanJobList'),
        activeScanCount: document.getElementById('activeScanCount'),
        evidence: document.getElementById('researchEvidence'), researchTime: document.getElementById('researchTime'),
        confidence: document.getElementById('confidenceSummary'), sources: document.getElementById('researchSources'),
        preview: document.getElementById('productPreview'), editor: document.getElementById('productEditor'),
        previewModeButton: document.getElementById('previewModeButton'), editModeButton: document.getElementById('editModeButton'),
        modeHint: document.getElementById('dialogModeHint'), previewEditButton: document.getElementById('previewEditButton'),
        toast: document.getElementById('toast')
    };

    const escapeHTML = (value) => {
        const node = document.createElement('span');
        node.textContent = value == null ? '' : String(value);
        return node.innerHTML;
    };

    const safeURL = (value) => {
        try {
            const parsed = new URL(value || '', window.location.href);
            return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.href : '';
        } catch (_) { return ''; }
    };

    const confidenceLabels = {
        positioning: '产品定位', target_users: '目标用户', problem: '解决问题', core_loop: '核心循环',
        core_mechanism: '核心机制', business_model: '商业模式', strengths: '认可点',
        user_complaints: '用户吐槽', competitive_edge: '竞品差异'
    };
    const levelLabels = { high: '高', medium: '中', low: '低' };
    const kindLabels = { official: '官方', review: '评测', forum: '社区', search: '搜索' };

    const richTextHTML = (value) => escapeHTML(value || '').replace(/\r?\n/g, '<br>');

    function confidenceChipsHTML(confidence = {}) {
        return Object.entries(confidence).filter(([key]) => confidenceLabels[key]).map(([key, level]) => {
            const levelClass = ['high', 'medium', 'low'].includes(level) ? level : '';
            return `<span class="confidence-chip ${levelClass}">${escapeHTML(confidenceLabels[key])} · ${escapeHTML(levelLabels[level] || level)}</span>`;
        }).join('');
    }

    function researchSourcesHTML(sources = []) {
        return sources.map((source) => {
            const href = safeURL(source.url);
            const title = escapeHTML(source.title || source.url || '研究来源');
            const heading = href ? `<a href="${escapeHTML(href)}" target="_blank" rel="noopener noreferrer">${title}</a>` : `<strong>${title}</strong>`;
            const kind = Object.prototype.hasOwnProperty.call(kindLabels, source.kind) ? source.kind : 'search';
            return `<article class="research-source"><span class="source-kind ${kind}">${escapeHTML(kindLabels[kind])}</span><div>${heading}<p>${escapeHTML(source.snippet || (source.fetched ? '已读取页面正文' : '使用搜索摘要'))}</p></div><small>${source.fetched ? '已读取' : '摘要'}</small></article>`;
        }).join('');
    }

    const displayDate = (value) => {
        if (!value) return '刚刚更新';
        const date = value.slice(0, 10);
        return date === new Date().toISOString().slice(0, 10) ? '今天更新' : `${date} 更新`;
    };

    const getError = async (response, fallback) => {
        try { const data = await response.json(); return data.error || fallback; } catch (_) { return fallback; }
    };

    function showToast(message) {
        clearTimeout(state.toastTimer);
        elements.toast.textContent = message;
        elements.toast.hidden = false;
        state.toastTimer = setTimeout(() => { elements.toast.hidden = true; }, 2800);
    }

    function cardHTML(product) {
        const coverURL = safeURL(product.cover_url);
        const initial = Array.from(product.name || '产')[0];
        const cover = coverURL
            ? `<div class="product-cover"><img src="${escapeHTML(coverURL)}" alt="" loading="lazy" onerror="this.parentElement.classList.add('no-image');this.remove()"><span class="product-type">${escapeHTML(product.product_type || '未分类')}</span></div>`
            : `<div class="product-cover no-image"><strong>${escapeHTML(initial)}</strong><span class="product-type">${escapeHTML(product.product_type || '未分类')}</span></div>`;
        const tags = (product.tags || []).slice(0, 4).map((tag) => `<span>${escapeHTML(tag)}</span>`).join('');
        const ideas = (product.transferable_ideas || []).length;
        const newBadge = product.is_new ? '<span class="product-new-badge">NEW</span>' : '';
        return `<button class="product-card${product.is_new ? ' is-new' : ''}" type="button" data-product-id="${escapeHTML(product.id)}" aria-label="查看 ${escapeHTML(product.name)}${product.is_new ? '，未查看' : ''}">
            ${cover}${newBadge}<span class="product-card-body"><h3 title="${escapeHTML(product.name)}">${escapeHTML(product.name)}</h3>
            <span class="product-summary">${escapeHTML(product.summary || product.problem || '打开产品卡，补充它值得研究的原因。')}</span>
            <span class="product-tags">${tags}</span><span class="product-card-footer"><span>${escapeHTML(displayDate(product.updated_at))}</span><strong>${ideas ? `${ideas} 条可迁移灵感` : '继续拆解 →'}</strong></span></span></button>`;
    }

    function searchableText(product) {
        return [product.name, product.summary, product.positioning, product.target_users, product.problem,
            product.core_loop, product.core_mechanism, product.feedback_rewards, product.social_mechanism,
            product.surprise, product.retention, product.business_model, product.competitive_edge,
            ...(product.key_mechanics || []), ...(product.strengths || []), ...(product.user_complaints || []),
            ...(product.tags || []), ...(product.transferable_ideas || []), ...(product.opportunities || [])].join(' ').toLowerCase();
    }

    function filteredProducts() {
        const query = state.query.trim().toLowerCase();
        return state.products.filter((product) => (!query || searchableText(product).includes(query)) &&
            (!state.type || product.product_type === state.type) && (!state.tag || (product.tags || []).includes(state.tag)));
    }

    function renderProducts() {
        const products = filteredProducts();
        elements.count.textContent = state.products.length;
        elements.grid.innerHTML = products.map(cardHTML).join('');
        elements.empty.hidden = state.products.length !== 0;
        elements.filterEmpty.hidden = state.products.length === 0 || products.length !== 0;
        elements.grid.hidden = products.length === 0;
    }

    function renderFilters() {
        const types = [...new Set(state.products.map((product) => product.product_type).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'zh-CN'));
        elements.type.innerHTML = '<option value="">全部产品</option>' + types.map((type) => `<option value="${escapeHTML(type)}">${escapeHTML(type)}</option>`).join('');
        elements.type.value = types.includes(state.type) ? state.type : '';
        const counts = new Map();
        state.products.forEach((product) => (product.tags || []).forEach((tag) => counts.set(tag, (counts.get(tag) || 0) + 1)));
        const tags = [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'zh-CN')).slice(0, 12);
        elements.tags.innerHTML = tags.map(([tag, count]) => `<button type="button" class="tag-filter${state.tag === tag ? ' active' : ''}" data-tag="${escapeHTML(tag)}">${escapeHTML(tag)} · ${count}</button>`).join('');
    }

    async function loadProducts() {
        const response = await fetch('/api/products', { headers: { Accept: 'application/json' } });
        if (!response.ok) throw new Error(await getError(response, '产品库加载失败'));
        const data = await response.json();
        state.products = data.products || [];
        renderFilters();
        renderProducts();
    }

    const scanIsActive = (job) => job.status === 'queued' || job.status === 'running';

    function scanJobName(rawURL) {
        try {
            const parsed = new URL(rawURL);
            const path = parsed.pathname === '/' ? '' : parsed.pathname.replace(/\/$/, '');
            return `${parsed.hostname}${path}`;
        } catch (_) { return rawURL || '产品扫描'; }
    }

    function renderScanJobs() {
        const visibleJobs = state.scanJobs.filter((job, index) => scanIsActive(job) || job.status === 'failed' || index < 4).slice(0, 8);
        const activeCount = state.scanJobs.filter(scanIsActive).length;
        elements.scanJobs.hidden = visibleJobs.length === 0;
        elements.activeScanCount.textContent = activeCount ? `${activeCount} 个进行中` : '最近任务';
        const statusLabels = { queued: '排队中', running: '研究中', succeeded: '已入库', failed: '失败' };
        elements.scanJobList.innerHTML = visibleJobs.map((job) => {
            const retry = job.status === 'failed'
                ? `<button type="button" data-retry-scan="${escapeHTML(job.source_url)}">重新扫描</button>` : '';
            const detail = job.status === 'failed' ? job.error_message : job.status === 'succeeded' ? '产品卡已自动保存' : job.status === 'running' ? '正在收集资料并生成产品卡' : '等待后台 worker';
            return `<article class="scan-job ${escapeHTML(job.status)}"><span class="scan-job-indicator" aria-hidden="true"></span><div><strong title="${escapeHTML(job.source_url)}">${escapeHTML(scanJobName(job.source_url))}</strong><p>${escapeHTML(detail || '')}</p></div><span class="scan-job-status">${escapeHTML(statusLabels[job.status] || job.status)}</span>${retry}</article>`;
        }).join('');
    }

    function scheduleScanPolling() {
        window.clearTimeout(state.scanPollTimer);
        state.scanPollTimer = null;
        if (!state.scanJobs.some(scanIsActive)) return;
        state.scanPollTimer = window.setTimeout(() => {
            loadScanJobs(true).catch((error) => {
                elements.scanStatus.textContent = error.message;
                scheduleScanPolling();
            });
        }, 5000);
    }

    async function loadScanJobs(notifyTransitions = false) {
        const previous = new Map(state.scanJobs.map((job) => [job.id, job.status]));
        const response = await fetch('/api/products/scan', { headers: { Accept: 'application/json' } });
        if (!response.ok) throw new Error(await getError(response, '扫描任务加载失败'));
        const data = await response.json();
        state.scanJobs = data.jobs || [];
        renderScanJobs();
        if (notifyTransitions) {
            const completed = state.scanJobs.some((job) => job.status === 'succeeded' && scanIsActive({ status: previous.get(job.id) }));
            const failed = state.scanJobs.find((job) => job.status === 'failed' && scanIsActive({ status: previous.get(job.id) }));
            if (completed) {
                await loadProducts();
                showToast('后台扫描完成，产品卡已自动入库');
            } else if (failed) {
                showToast(`扫描失败：${failed.error_message || '请重新尝试'}`);
            }
        }
        scheduleScanPolling();
    }

    async function queueProductScan(rawURL) {
        const response = await fetch('/api/products/scan', {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: rawURL })
        });
        if (!response.ok) throw new Error(await getError(response, '扫描任务创建失败'));
        const data = await response.json();
        const job = data.job;
        if (job) {
            state.scanJobs = [job, ...state.scanJobs.filter((item) => item.id !== job.id)];
            renderScanJobs();
            scheduleScanPolling();
        }
        return data;
    }

    const field = (name) => elements.form.elements.namedItem(name);
    const lines = (value) => (value || []).join('\n');

    function fillForm(product = {}) {
        elements.form.reset();
        ['name', 'product_type', 'source_url', 'cover_url', 'summary', 'positioning', 'target_users', 'problem',
            'core_loop', 'core_mechanism', 'feedback_rewards', 'social_mechanism', 'surprise', 'retention',
            'business_model', 'competitive_edge'].forEach((name) => {
            field(name).value = product[name] || '';
        });
        field('key_mechanics').value = lines(product.key_mechanics);
        field('strengths').value = lines(product.strengths);
        field('user_complaints').value = lines(product.user_complaints);
        field('transferable_ideas').value = lines(product.transferable_ideas);
        field('opportunities').value = lines(product.opportunities);
        field('tags').value = (product.tags || []).join('、');
    }

    function renderResearchEvidence(product = {}) {
        state.researchSources = Array.isArray(product.research_sources) ? product.research_sources : [];
        state.confidence = product.confidence || {};
        state.evidence = product.evidence || {};
        state.lastResearchedAt = product.last_researched_at || '';
        elements.evidence.hidden = state.researchSources.length === 0;
        if (elements.evidence.hidden) return;
        elements.researchTime.textContent = state.lastResearchedAt ? `${state.lastResearchedAt} 完成` : '';
        elements.confidence.innerHTML = confidenceChipsHTML(state.confidence);
        elements.sources.innerHTML = researchSourcesHTML(state.researchSources);
    }

    function previewList(values = [], className = '') {
        if (!values.length) return '';
        return `<ul class="preview-list ${className}">${values.map((value) => `<li>${escapeHTML(value)}</li>`).join('')}</ul>`;
    }

    function previewDetail(label, value, className = '') {
        if (!value) return '';
        return `<article class="preview-detail ${className}"><p>${escapeHTML(label)}</p><div>${richTextHTML(value)}</div></article>`;
    }

    function renderProductPreview(product) {
        const name = product.name || '未命名产品';
        const initial = Array.from(name)[0] || '产';
        const coverURL = safeURL(product.cover_url);
        const sourceURL = safeURL(product.source_url);
        const cover = coverURL
            ? `<figure class="preview-cover"><img src="${escapeHTML(coverURL)}" alt="${escapeHTML(name)} 封面" onerror="this.parentElement.classList.add('no-image');this.remove()"></figure>`
            : `<figure class="preview-cover no-image" aria-hidden="true"><strong>${escapeHTML(initial)}</strong><span>PRODUCT NOTE</span></figure>`;
        const tags = (product.tags || []).map((tag) => `<span>${escapeHTML(tag)}</span>`).join('');
        const sourceLink = sourceURL
            ? `<a class="preview-source-link" href="${escapeHTML(sourceURL)}" target="_blank" rel="noopener noreferrer">访问产品来源 <span aria-hidden="true">↗</span></a>` : '';
        const keyMechanics = previewList(product.key_mechanics, 'mechanics-list');
        const researchSources = Array.isArray(product.research_sources) ? product.research_sources : [];
        const confidence = product.confidence || {};

        elements.preview.innerHTML = `<article class="product-report">
            <header class="preview-hero">
                ${cover}
                <div class="preview-identity">
                    <p class="preview-kicker">PRODUCT RESEARCH NOTE · ${escapeHTML(product.product_type || '未分类')}</p>
                    <h3>${escapeHTML(name)}</h3>
                    <p class="preview-summary">${richTextHTML(product.summary || '这张产品卡还没有一句话介绍。')}</p>
                    ${tags ? `<div class="preview-tags">${tags}</div>` : ''}
                    ${sourceLink}
                </div>
            </header>

            <section class="preview-question-grid" aria-label="产品定位">
                ${previewDetail('产品定位', product.positioning)}
                ${previewDetail('目标用户', product.target_users)}
                ${previewDetail('解决的问题', product.problem, 'problem')}
            </section>

            ${(product.core_loop || product.core_mechanism || keyMechanics) ? `<section class="preview-section preview-core">
                <div class="preview-section-heading"><p>MECHANISM MAP</p><h4>核心玩法与工作流</h4></div>
                <div class="preview-core-grid">
                    ${previewDetail('核心循环', product.core_loop, 'core-loop')}
                    ${previewDetail('机制拆解', product.core_mechanism, 'core-mechanism')}
                </div>
                ${keyMechanics ? `<div class="preview-mechanics"><p>关键规则与玩法机制</p>${keyMechanics}</div>` : ''}
            </section>` : ''}

            <section class="preview-detail-grid">
                ${previewDetail('反馈与奖励', product.feedback_rewards)}
                ${previewDetail('社交 / 竞争 / 协作', product.social_mechanism)}
                ${previewDetail('惊喜点', product.surprise)}
                ${previewDetail('留存点', product.retention)}
                ${previewDetail('商业模式', product.business_model)}
                ${previewDetail('竞品差异', product.competitive_edge)}
            </section>

            ${(product.strengths || []).length || (product.user_complaints || []).length ? `<section class="preview-section preview-voices">
                <div class="preview-section-heading"><p>MARKET SIGNAL</p><h4>认可与摩擦</h4></div>
                <div class="preview-list-grid">
                    <article><p>用户认可点</p>${previewList(product.strengths, 'strength-list')}</article>
                    <article><p>论坛与用户吐槽</p>${previewList(product.user_complaints, 'complaint-list')}</article>
                </div>
            </section>` : ''}

            ${(product.transferable_ideas || []).length || (product.opportunities || []).length ? `<section class="preview-section preview-inspiration">
                <div class="preview-section-heading"><p>DESIGN TRANSFER</p><h4>带走什么</h4></div>
                <div class="preview-list-grid">
                    <article><p>可迁移灵感</p>${previewList(product.transferable_ideas, 'idea-list')}</article>
                    <article><p>缺点与机会</p>${previewList(product.opportunities, 'opportunity-list')}</article>
                </div>
            </section>` : ''}

            ${researchSources.length ? `<section class="preview-section preview-research">
                <div class="preview-section-heading"><p>RESEARCH EVIDENCE</p><h4>来源与可信度</h4><span>${escapeHTML(product.last_researched_at || '')}</span></div>
                <div class="confidence-summary">${confidenceChipsHTML(confidence)}</div>
                <div class="research-sources">${researchSourcesHTML(researchSources)}</div>
            </section>` : ''}
        </article>`;
    }

    function setDialogMode(mode) {
        const previewing = mode === 'preview';
        state.dialogMode = previewing ? 'preview' : 'edit';
        elements.preview.hidden = !previewing;
        elements.editor.hidden = previewing;
        elements.previewModeButton.setAttribute('aria-selected', String(previewing));
        elements.editModeButton.setAttribute('aria-selected', String(!previewing));
        elements.previewModeButton.classList.toggle('active', previewing);
        elements.editModeButton.classList.toggle('active', !previewing);
        elements.modeHint.textContent = previewing ? '以研究报告形式阅读产品卡。' : '修改字段后可随时切回预览，未保存内容也会参与渲染。';
        elements.previewEditButton.hidden = !previewing;
        elements.saveButton.hidden = previewing;
        if (state.editingID) {
            elements.dialogTitle.textContent = previewing ? '产品卡详情' : '编辑产品卡';
        }
        if (previewing) {
            renderProductPreview(formProduct());
        } else {
            setTimeout(() => field('name').focus(), 0);
        }
    }

    function openDialog(product = null, aiDraft = false) {
        state.editingID = product && product.id ? product.id : '';
        fillForm(product || {});
        renderResearchEvidence(product || {});
        elements.dialogTitle.textContent = aiDraft ? '审阅扫描草稿' : state.editingID ? '编辑产品卡' : '手动录入产品';
        elements.dialogEyebrow.textContent = aiDraft ? 'AI SCAN DRAFT' : state.editingID ? 'PRODUCT CARD' : 'MANUAL ENTRY';
        elements.draftNotice.hidden = !aiDraft;
        elements.deleteButton.hidden = !state.editingID;
        elements.formStatus.textContent = '';
        elements.dialog.showModal();
        setDialogMode(state.editingID && !aiDraft ? 'preview' : 'edit');
    }

    function splitValues(value) {
        return [...new Set(String(value || '').split(/[\n,，、]+/).map((item) => item.trim()).filter(Boolean))];
    }

    function formProduct() {
        return {
            name: field('name').value.trim(), product_type: field('product_type').value.trim(),
            source_url: field('source_url').value.trim(), cover_url: field('cover_url').value.trim(),
            summary: field('summary').value.trim(), positioning: field('positioning').value.trim(),
            target_users: field('target_users').value.trim(), problem: field('problem').value.trim(),
            core_loop: field('core_loop').value.trim(), core_mechanism: field('core_mechanism').value.trim(),
            key_mechanics: splitValues(field('key_mechanics').value),
            feedback_rewards: field('feedback_rewards').value.trim(), social_mechanism: field('social_mechanism').value.trim(),
            surprise: field('surprise').value.trim(), retention: field('retention').value.trim(),
            business_model: field('business_model').value.trim(), strengths: splitValues(field('strengths').value),
            user_complaints: splitValues(field('user_complaints').value), competitive_edge: field('competitive_edge').value.trim(),
            transferable_ideas: splitValues(field('transferable_ideas').value),
            opportunities: splitValues(field('opportunities').value), tags: splitValues(field('tags').value),
            research_sources: state.researchSources, confidence: state.confidence, evidence: state.evidence,
            last_researched_at: state.lastResearchedAt
        };
    }

    async function saveProduct(event) {
        event.preventDefault();
        if (!elements.form.reportValidity()) return;
        elements.saveButton.disabled = true;
        elements.formStatus.textContent = '正在保存…';
        const method = state.editingID ? 'PUT' : 'POST';
        const endpoint = `/api/products${state.editingID ? `?id=${encodeURIComponent(state.editingID)}` : ''}`;
        try {
            const response = await fetch(endpoint, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(formProduct()) });
            if (!response.ok) throw new Error(await getError(response, '产品保存失败'));
            elements.dialog.close();
            await loadProducts();
            showToast(state.editingID ? '产品卡已更新' : '产品已加入星图');
        } catch (error) { elements.formStatus.textContent = error.message; }
        finally { elements.saveButton.disabled = false; }
    }

    async function deleteProduct() {
        if (!state.editingID || !window.confirm('确定删除这张产品卡吗？此操作无法撤销。')) return;
        elements.deleteButton.disabled = true;
        try {
            const response = await fetch(`/api/products?id=${encodeURIComponent(state.editingID)}`, { method: 'DELETE' });
            if (!response.ok) throw new Error(await getError(response, '产品删除失败'));
            elements.dialog.close();
            await loadProducts();
            showToast('产品卡已删除');
        } catch (error) { elements.formStatus.textContent = error.message; }
        finally { elements.deleteButton.disabled = false; }
    }

    async function scanProduct(event) {
        event.preventDefault();
        if (!elements.scanForm.reportValidity()) return;
        elements.scanButton.disabled = true;
        elements.scanConsole.classList.add('scanning');
        elements.scanStatus.textContent = '正在加入后台队列…';
        try {
            const data = await queueProductScan(elements.scanURL.value.trim());
            elements.scanStatus.textContent = data.reused ? '这个产品已经在扫描队列中。' : '已加入后台队列，可以继续提交下一个游戏。';
            if (!data.reused) elements.scanURL.value = '';
        } catch (error) { elements.scanStatus.textContent = error.message; }
        finally { elements.scanButton.disabled = false; elements.scanConsole.classList.remove('scanning'); }
    }

    async function markProductViewed(product) {
        if (!product.is_new) return;
        product.is_new = false;
        renderProducts();
        try {
            const response = await fetch(`/api/products?id=${encodeURIComponent(product.id)}`, { method: 'PATCH' });
            if (!response.ok) throw new Error(await getError(response, 'NEW 状态更新失败'));
        } catch (error) {
            product.is_new = true;
            renderProducts();
            showToast(error.message);
        }
    }

    document.getElementById('manualAddButton').addEventListener('click', () => openDialog());
    document.getElementById('emptyAddButton').addEventListener('click', () => openDialog());
    document.getElementById('closeDialogButton').addEventListener('click', () => elements.dialog.close());
    document.getElementById('cancelProductButton').addEventListener('click', () => elements.dialog.close());
    document.getElementById('clearFiltersButton').addEventListener('click', () => {
        state.query = state.type = state.tag = ''; elements.search.value = ''; elements.type.value = ''; renderFilters(); renderProducts();
    });
    elements.form.addEventListener('submit', saveProduct);
    elements.previewModeButton.addEventListener('click', () => setDialogMode('preview'));
    elements.editModeButton.addEventListener('click', () => setDialogMode('edit'));
    elements.previewEditButton.addEventListener('click', () => setDialogMode('edit'));
    elements.deleteButton.addEventListener('click', deleteProduct);
    elements.scanForm.addEventListener('submit', scanProduct);
    elements.scanJobList.addEventListener('click', async (event) => {
        const button = event.target.closest('[data-retry-scan]');
        if (!button) return;
        button.disabled = true;
        try {
            await queueProductScan(button.dataset.retryScan);
            elements.scanStatus.textContent = '已重新加入后台队列。';
        } catch (error) {
            elements.scanStatus.textContent = error.message;
        } finally { button.disabled = false; }
    });
    elements.search.addEventListener('input', () => { state.query = elements.search.value; renderProducts(); });
    elements.type.addEventListener('change', () => { state.type = elements.type.value; renderProducts(); });
    elements.tags.addEventListener('click', (event) => {
        const button = event.target.closest('[data-tag]'); if (!button) return;
        state.tag = state.tag === button.dataset.tag ? '' : button.dataset.tag; renderFilters(); renderProducts();
    });
    elements.grid.addEventListener('click', (event) => {
        const card = event.target.closest('[data-product-id]'); if (!card) return;
        const product = state.products.find((item) => item.id === card.dataset.productId);
        if (product) {
            openDialog(product);
            markProductViewed(product);
        }
    });
    elements.dialog.addEventListener('click', (event) => {
        if (event.target === elements.dialog) elements.dialog.close();
    });

    loadProducts().catch((error) => {
        elements.empty.hidden = false;
        elements.empty.querySelector('h3').textContent = '产品库暂时无法加载';
        elements.empty.querySelector('p').textContent = error.message;
    });
    loadScanJobs().catch((error) => { elements.scanStatus.textContent = error.message; });
})();
