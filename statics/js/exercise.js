// 全局变量
let currentDate = new Date().toISOString().split('T')[0];
let currentView = 'exercise';
let currentPeriod = 'week';
let isEditing = false;
let editingId = null;
let isTemplateEditing = false;
let editingTemplateId = null;
let isCollectionEditing = false;
let editingCollectionId = null;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializePage();
});

// 初始化页面
function initializePage() {
    console.log('开始初始化页面');
    
    // 从URL参数获取日期
    const urlParams = new URLSearchParams(window.location.search);
    const dateParam = urlParams.get('date');
    if (dateParam) {
        currentDate = dateParam;
    }
    
    // 设置当前日期
    document.getElementById('datePicker').value = currentDate;
    updateCurrentDateDisplay();
    
    // 首先确保所有视图都隐藏
    hideAllViews();
    
    // 绑定事件监听器
    bindEventListeners();
    
    // 显示锻炼视图（这会自动加载对应数据）
    setTimeout(() => {
        showExerciseView();
    }, 200);
    
    showToast('锻炼管理页面加载完成', 'success');
}

// 绑定事件监听器
function bindEventListeners() {
    // 日期选择器
    document.getElementById('datePicker').addEventListener('change', function() {
        currentDate = this.value;
        updateCurrentDateDisplay();
        loadExercises();
    });
    
    // 锻炼表单
    document.getElementById('exerciseFormElement').addEventListener('submit', function(e) {
        e.preventDefault();
        saveExercise();
    });
    
    // 模板表单
    document.getElementById('templateFormElement').addEventListener('submit', function(e) {
        e.preventDefault();
        saveTemplate();
    });
    
    // 集合表单
    document.getElementById('collectionFormElement').addEventListener('submit', function(e) {
        e.preventDefault();
        saveCollection();
    });
    
    // 个人信息表单
    document.getElementById('profileFormElement').addEventListener('submit', function(e) {
        e.preventDefault();
        saveUserProfile();
    });
    
    // BMI计算监听器
    document.getElementById('profileWeight').addEventListener('input', calculateBMI);
    document.getElementById('profileHeight').addEventListener('input', calculateBMI);
    
    // 模板表单自动计算监听器
    document.getElementById('templateType').addEventListener('change', autoCalculateTemplateCalories);
    document.getElementById('templateIntensity').addEventListener('change', autoCalculateTemplateCalories);
    document.getElementById('templateDuration').addEventListener('input', autoCalculateTemplateCalories);
    document.getElementById('templateWeight').addEventListener('input', autoCalculateTemplateCalories);
    
    // 锻炼表单自动计算监听器
    document.getElementById('exerciseType').addEventListener('change', autoCalculateExerciseCalories);
    document.getElementById('exerciseIntensity').addEventListener('change', autoCalculateExerciseCalories);
    document.getElementById('exerciseDuration').addEventListener('input', autoCalculateExerciseCalories);
    document.getElementById('exerciseWeight').addEventListener('input', autoCalculateExerciseCalories);
    
    // MET值显示监听器
    document.getElementById('templateType').addEventListener('change', updateTemplateMETDisplay);
    document.getElementById('templateIntensity').addEventListener('change', updateTemplateMETDisplay);
    document.getElementById('exerciseType').addEventListener('change', updateExerciseMETDisplay);
    document.getElementById('exerciseIntensity').addEventListener('change', updateExerciseMETDisplay);
    
    // 统计年份和月份选择
    document.getElementById('statsYear').addEventListener('change', updateStats);
    document.getElementById('statsMonth').addEventListener('change', updateStats);
    
    // 初始化统计控件
    const now = new Date();
    document.getElementById('statsYear').value = now.getFullYear();
    document.getElementById('statsMonth').value = now.getMonth() + 1;
}

// 视图切换函数
function showExerciseView() {
    showView('exerciseView');
    setActiveNavButton(0);
    currentView = 'exercise';
    
    // 只加载锻炼相关数据
    loadExercises();
    
    // 确保其他表单隐藏
    hideAddForm();
    resetExerciseForm();
    
    console.log('切换到锻炼视图');
}

function showTemplateView() {
    showView('templateView');
    setActiveNavButton(1);
    currentView = 'template';
    
    // 只加载模板相关数据
    loadTemplates();
    
    // 重置模板表单状态
    resetTemplateForm();
    
    console.log('切换到模板视图');
}

function showCollectionView() {
    showView('collectionView');
    setActiveNavButton(2);
    currentView = 'collection';
    
    // 只加载集合相关数据
    loadCollections();
    loadTemplatesForCheckboxes();
    
    // 重置集合表单状态
    resetCollectionForm();
    
    console.log('切换到集合管理视图');
}

function showProfileView() {
    showView('profileView');
    setActiveNavButton(3);
    currentView = 'profile';
    
    // 只加载个人信息相关数据
    loadUserProfile();
    loadMETValues();
    
    console.log('切换到个人信息视图');
}

function showStatsView() {
    showView('statsView');
    setActiveNavButton(4);
    currentView = 'stats';
    
    // 只加载统计相关数据
    updateStats();
    
    console.log('切换到统计分析视图');
}

function hideAllViews() {
    // 获取所有视图并强制隐藏
    const views = ['exerciseView', 'templateView', 'collectionView', 'profileView', 'statsView'];
    views.forEach(viewId => {
        const view = document.getElementById(viewId);
        if (view) {
            view.classList.remove('active');
            view.style.display = 'none';
            view.style.visibility = 'hidden';
            view.style.opacity = '0';
            view.style.position = 'absolute';
            view.style.left = '-9999px';
            view.style.top = '-9999px';
        }
    });
    
    // 同时隐藏所有可能的弹出表单
    hideAddForm();
    resetExerciseForm();
    resetTemplateForm();
    resetCollectionForm();
}

// 显示指定视图的通用函数
function showView(viewId) {
    // 首先隐藏所有视图
    hideAllViews();
    hideAllEditForms();
    
    // 移除所有视图类名
    document.body.classList.remove('view-exercise', 'view-template', 'view-collection', 'view-profile', 'view-stats');
    
    // 显示指定视图
    const view = document.getElementById(viewId);
    if (view) {
        view.classList.add('active');
        view.style.display = 'block';
        view.style.visibility = 'visible';
        view.style.opacity = '1';
        view.style.position = 'static';
        view.style.left = 'auto';
        view.style.top = 'auto';
    }
    
    // 根据视图ID给body添加对应的类名
    switch (viewId) {
        case 'exerciseView':
            document.body.classList.add('view-exercise');
            break;
        case 'templateView':
            document.body.classList.add('view-template');
            break;
        case 'collectionView':
            document.body.classList.add('view-collection');
            break;
        case 'profileView':
            document.body.classList.add('view-profile');
            break;
        case 'statsView':
            document.body.classList.add('view-stats');
            break;
    }
}



function setActiveNavButton(index) {
    document.querySelectorAll('.nav-btn').forEach((btn, i) => {
        if (i === index) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });
}

// 日期相关函数
function goToToday() {
    currentDate = new Date().toISOString().split('T')[0];
    document.getElementById('datePicker').value = currentDate;
    updateCurrentDateDisplay();
    loadExercises();
}

function updateCurrentDateDisplay() {
    const date = new Date(currentDate);
    const options = { 
        year: 'numeric', 
        month: 'long', 
        day: 'numeric',
        weekday: 'long'
    };
    document.getElementById('currentDate').textContent = date.toLocaleDateString('zh-CN', options);
}

// 锻炼管理函数
async function loadExercises() {
    try {
        const response = await fetch(`/api/exercises?date=${currentDate}`);
        const data = await response.json();
        
        renderExercises(data.items || []);
        updateDailyStats(data.items || []);
    } catch (error) {
        console.error('加载锻炼数据失败:', error);
        showToast('加载锻炼数据失败', 'error');
    }
}

function renderExercises(exercises) {
    const container = document.getElementById('exerciseItems');
    const emptyState = document.getElementById('exerciseEmpty');
    
    if (exercises.length === 0) {
        container.innerHTML = '';
        emptyState.style.display = 'block';
        return;
    }
    
    emptyState.style.display = 'none';
    container.innerHTML = exercises.map(exercise => `
        <div class="exercise-item ${exercise.completed ? 'completed' : ''}" data-id="${exercise.id}">
            <div class="exercise-header">
                <div>
                    <div class="exercise-name">${exercise.name}</div>
                    <span class="exercise-type">${getTypeLabel(exercise.type)}</span>
                </div>
                <div class="exercise-actions">
                    <button class="btn-success" onclick="toggleExercise('${exercise.id}')" title="${exercise.completed ? '标记未完成' : '标记完成'}">
                        ${exercise.completed ? '✓' : '○'}
                    </button>
                    <button class="btn-secondary" onclick="editExercise('${exercise.id}')" title="编辑">✏️</button>
                    <button class="btn-danger" onclick="deleteExercise('${exercise.id}')" title="删除">🗑️</button>
                </div>
            </div>
            <div class="exercise-details">
                <div class="detail-item">
                    <div class="detail-label">时长</div>
                    <div class="detail-value">${exercise.duration}分钟</div>
                </div>
                <div class="detail-item">
                    <div class="detail-label">强度</div>
                    <div class="detail-value">${getIntensityLabel(exercise.intensity)}</div>
                </div>
                <div class="detail-item">
                    <div class="detail-label">卡路里</div>
                    <div class="detail-value">${exercise.calories || 0}</div>
                </div>
                ${exercise.weight > 0 ? `
                <div class="detail-item">
                    <div class="detail-label">负重</div>
                    <div class="detail-value">${exercise.weight}kg</div>
                </div>
                ` : ''}
                <div class="detail-item">
                    <div class="detail-label">部位</div>
                    <div class="detail-value">${(exercise.body_parts || []).join('、') || '-'}</div>
                </div>
            </div>
            ${exercise.notes ? `<div class="exercise-notes">${exercise.notes}</div>` : ''}
        </div>
    `).join('');
}

function updateDailyStats(exercises) {
    const completedExercises = exercises.filter(ex => ex.completed);
    const totalDuration = completedExercises.reduce((sum, ex) => sum + ex.duration, 0);
    const totalCalories = completedExercises.reduce((sum, ex) => sum + (ex.calories || 0), 0);
    
    document.getElementById('todayDuration').textContent = totalDuration;
    document.getElementById('todayCalories').textContent = totalCalories;
    document.getElementById('todayCount').textContent = completedExercises.length;
}

// 锻炼表单函数
function showAddForm() {
    const form = document.getElementById('exerciseForm');
    if (form) {
        form.style.display = 'block';
    }
    
    // 重置表单状态
    resetExerciseForm();
    
    // 确保表单标题正确
    const formTitle = document.getElementById('formTitle');
    if (formTitle) {
        formTitle.textContent = '添加锻炼';
    }
    
    // 滚动到表单位置
    form && form.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function hideAddForm() {
    const form = document.getElementById('exerciseForm');
    if (form) {
        form.style.display = 'none';
    }
    
    // 重置锻炼表单
    resetExerciseForm();
    
    // 清除锻炼部位选择
    const bodyPartCheckboxes = document.querySelectorAll('#exerciseBodyParts input[name="body_parts"]');
    bodyPartCheckboxes.forEach(cb => {
        cb.checked = false;
    });
}

function resetExerciseForm() {
    const form = document.getElementById('exerciseFormElement');
    if (form) {
        form.reset();
    }
    const idField = document.getElementById('exerciseId');
    if (idField) {
        idField.value = '';
    }
    hideMETDisplay('exercise');
    
    // 重置编辑状态
    isEditing = false;
    editingId = null;
    
    // 重置表单标题
    const formTitle = document.getElementById('formTitle');
    if (formTitle) {
        formTitle.textContent = '添加锻炼';
    }
}

// 修改saveExercise函数，收集锻炼部位
async function saveExercise() {
    const formData = {
        date: currentDate,
        name: document.getElementById('exerciseName').value,
        type: document.getElementById('exerciseType').value,
        duration: parseInt(document.getElementById('exerciseDuration').value),
        intensity: document.getElementById('exerciseIntensity').value,
        calories: parseInt(document.getElementById('exerciseCalories').value) || 0,
        notes: document.getElementById('exerciseNotes').value,
        weight: parseFloat(document.getElementById('exerciseWeight').value) || 0,
        // 新增：收集锻炼部位
        body_parts: Array.from(document.querySelectorAll('#exerciseBodyParts input[name="body_parts"]:checked')).map(cb => cb.value)
    };
    
    try {
        let response;
        if (isEditing) {
            formData.id = editingId;
            response = await fetch('/api/exercises', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData)
            });
        } else {
            response = await fetch('/api/exercises', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData)
            });
        }
        
        if (response.ok) {
            showToast(isEditing ? '锻炼更新成功' : '锻炼添加成功', 'success');
            hideAddForm();
            loadExercises();
        } else {
            throw new Error('保存失败');
        }
    } catch (error) {
        console.error('保存锻炼失败:', error);
        showToast('保存锻炼失败', 'error');
    }
}

// 修改editExercise函数，编辑时自动勾选body_parts
async function editExercise(id) {
    try {
        const response = await fetch(`/api/exercises?date=${currentDate}`);
        const data = await response.json();
        const exercise = data.items.find(ex => ex.id === id);
        
        if (exercise) {
            document.getElementById('exerciseId').value = exercise.id;
            document.getElementById('exerciseName').value = exercise.name;
            document.getElementById('exerciseType').value = exercise.type;
            document.getElementById('exerciseDuration').value = exercise.duration;
            document.getElementById('exerciseIntensity').value = exercise.intensity;
            document.getElementById('exerciseCalories').value = exercise.calories || 0;
            document.getElementById('exerciseNotes').value = exercise.notes || '';
            document.getElementById('exerciseWeight').value = exercise.weight || 0;
            // 新增：设置锻炼部位多选
            const allCbs = document.querySelectorAll('#exerciseBodyParts input[name="body_parts"]');
            allCbs.forEach(cb => {
                cb.checked = (exercise.body_parts || []).includes(cb.value);
            });
            document.getElementById('exerciseForm').style.display = 'block';
            document.getElementById('formTitle').textContent = '编辑锻炼';
            isEditing = true;
            editingId = id;
            // 更新MET显示
            updateExerciseMETDisplay();
        }
    } catch (error) {
        console.error('加载锻炼数据失败:', error);
        showToast('加载锻炼数据失败', 'error');
    }
}

async function deleteExercise(id) {
    if (!confirm('确定要删除这个锻炼项目吗？')) {
        return;
    }
    
    try {
        const response = await fetch('/api/exercises', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ date: currentDate, id: id })
        });
        
        if (response.ok) {
            showToast('锻炼删除成功', 'success');
            loadExercises();
        } else {
            throw new Error('删除失败');
        }
    } catch (error) {
        console.error('删除锻炼失败:', error);
        showToast('删除锻炼失败', 'error');
    }
}

async function toggleExercise(id) {
    try {
        const response = await fetch('/api/exercises/toggle', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ date: currentDate, id: id })
        });
        
        if (response.ok) {
            loadExercises();
        } else {
            throw new Error('切换状态失败');
        }
    } catch (error) {
        console.error('切换锻炼状态失败:', error);
        showToast('切换锻炼状态失败', 'error');
    }
}

// 模板管理函数
async function loadTemplates() {
    try {
        const response = await fetch('/api/exercise-templates');
        const templates = await response.json();
        
        // 隐藏所有编辑表单
        hideAllEditForms();
        
        renderTemplates(templates || []);
        updateTemplateSelect(templates || []);
    } catch (error) {
        console.error('加载模板失败:', error);
        showToast('加载模板失败', 'error');
    }
}

function renderTemplates(templates) {
    const container = document.getElementById('templateItems');
    const emptyState = document.getElementById('templateEmpty');
    
    if (templates.length === 0) {
        container.innerHTML = '';
        emptyState.style.display = 'block';
        return;
    }
    
    emptyState.style.display = 'none';
    container.innerHTML = templates.map(template => `
        <div class="template-item" data-id="${template.id}">
            <div class="template-header">
                <div>
                    <div class="template-name">${template.name}</div>
                    <span class="template-type">${getTypeLabel(template.type)}</span>
                </div>
                <div class="template-actions">
                    <button class="btn-secondary" onclick="editTemplate('${template.id}')" title="编辑">✏️</button>
                    <button class="btn-danger" onclick="deleteTemplate('${template.id}')" title="删除">🗑️</button>
                </div>
            </div>
            <div class="template-details">
                <div class="detail-item"><span class="detail-label">时长</span><span class="detail-value">${template.duration}分钟</span></div>
                <div class="detail-item"><span class="detail-label">强度</span><span class="detail-value">${getIntensityLabel(template.intensity)}</span></div>
                <div class="detail-item"><span class="detail-label">卡路里</span><span class="detail-value">${template.calories || 0}</span></div>
                <div class="detail-item"><span class="detail-label">负重</span><span class="detail-value">${template.weight || 0}kg</span></div>
                <div class="detail-item"><span class="detail-label">部位</span><span class="detail-value">${(template.body_parts || []).join('、') || '-'}</span></div>
                ${template.notes ? `<div class="detail-item"><span class="detail-label">备注</span><span class="detail-value">${template.notes}</span></div>` : ''}
            </div>
        </div>
    `).join('');
}

function updateTemplateSelect(templates) {
    const select = document.getElementById('templateSelect');
    select.innerHTML = '<option value="">选择模板</option>' + 
        templates.map(template => `<option value="${template.id}">${template.name}</option>`).join('');
}

async function addFromTemplate() {
    const templateId = document.getElementById('templateSelect').value;
    if (!templateId) {
        showToast('请选择一个模板', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/exercise-templates');
        const templates = await response.json();
        const template = templates.find(t => t.id === templateId);
        
        if (template) {
            const exerciseData = {
                date: currentDate,
                name: template.name,
                type: template.type,
                duration: template.duration,
                intensity: template.intensity,
                calories: template.calories,
                notes: template.notes,
                weight: template.weight || 0,
                body_parts: template.body_parts || [] // 修复：同步部位
            };
            
            const addResponse = await fetch('/api/exercises', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(exerciseData)
            });
            
            if (addResponse.ok) {
                showToast('从模板添加锻炼成功', 'success');
                document.getElementById('templateSelect').value = '';
                loadExercises();
            } else {
                throw new Error('添加失败');
            }
        }
    } catch (error) {
        console.error('从模板添加锻炼失败:', error);
        showToast('从模板添加锻炼失败', 'error');
    }
}

// 修改saveTemplate函数，收集锻炼部位
async function saveTemplate() {
    const id = document.getElementById('templateId').value;
    const name = document.getElementById('templateName').value.trim();
    const type = document.getElementById('templateType').value;
    const duration = parseInt(document.getElementById('templateDuration').value) || 0;
    const intensity = document.getElementById('templateIntensity').value;
    const calories = parseInt(document.getElementById('templateCalories').value) || 0;
    const notes = document.getElementById('templateNotes').value.trim();
    const weight = parseFloat(document.getElementById('templateWeight').value) || 0;
    // 新增：收集锻炼部位
    const bodyParts = Array.from(document.querySelectorAll('#templateBodyParts input[name="body_parts"]:checked')).map(cb => cb.value);

    if (!name || !type || !duration || !intensity) {
        showToast('请填写完整模板信息', 'error');
        return;
    }

    const templateData = {
        id,
        name,
        type,
        duration,
        intensity,
        calories,
        notes,
        weight,
        body_parts: bodyParts
    };

    let url = '/api/exercise-templates';
    let method = id ? 'PUT' : 'POST';

    try {
        const response = await fetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(templateData)
        });
        if (!response.ok) throw new Error('保存失败');
        showToast('模板保存成功', 'success');
        resetTemplateForm();
        loadTemplates();
    } catch (error) {
        showToast('保存模板失败', 'error');
    }
}

async function editTemplate(id) {
    try {
        const response = await fetch('/api/exercise-templates');
        const templates = await response.json();
        
        // 获取模板数据
        const template = templates.find(t => t.id === id);
        if (!template) return;
        
        // 隐藏所有现有的编辑表单
        hideAllEditForms();
        
        // 在对应模板上方创建编辑表单
        showTemplateEditForm(id, template);
        
        // 设置锻炼部位多选
        const allCbs = document.querySelectorAll('#templateBodyParts input[name="body_parts"]');
        allCbs.forEach(cb => {
            cb.checked = (template.body_parts || []).includes(cb.value);
        });
        
        isTemplateEditing = true;
        editingTemplateId = id;
    } catch (error) {
        console.error('加载模板数据失败:', error);
        showToast('加载模板数据失败', 'error');
    }
}

async function deleteTemplate(id) {
    if (!confirm('确定要删除这个模板吗？')) {
        return;
    }
    
    try {
        const response = await fetch('/api/exercise-templates', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
        });
        
        if (response.ok) {
            showToast('模板删除成功', 'success');
            loadTemplates();
        } else {
            throw new Error('删除失败');
        }
    } catch (error) {
        console.error('删除模板失败:', error);
        showToast('删除模板失败', 'error');
    }
}

function resetTemplateForm() {
    const form = document.getElementById('templateFormElement');
    if (form) {
        form.reset();
    }
    const idField = document.getElementById('templateId');
    if (idField) {
        idField.value = '';
    }
    const formTitle = document.getElementById('templateFormTitle');
    if (formTitle) {
        formTitle.textContent = '添加模板';
    }
    
    // 重置编辑状态
    isTemplateEditing = false;
    editingTemplateId = null;
    
    // 隐藏MET显示
    hideMETDisplay('template');
    
    // 隐藏所有编辑表单
    hideAllEditForms();
    
    // 清除锻炼部位选择
    const bodyPartCheckboxes = document.querySelectorAll('#templateBodyParts input[name="body_parts"]');
    bodyPartCheckboxes.forEach(cb => {
        cb.checked = false;
    });
}

// 隐藏所有编辑表单
function hideAllEditForms() {
    const editForms = document.querySelectorAll('.template-edit-form');
    editForms.forEach(form => form.remove());
}

// 显示模板编辑表单
function showTemplateEditForm(templateId, template) {
    const templateItem = document.querySelector(`[data-id="${templateId}"]`);
    if (!templateItem) return;
    
    // 创建编辑表单HTML
    const editFormHTML = `
        <div class="template-edit-form" data-template-id="${templateId}">
            <h3>编辑模板</h3>
            <form id="editTemplateForm-${templateId}">
                <input type="hidden" value="${template.id}">
                <div class="form-row">
                    <div class="form-group">
                        <label>模板名称*</label>
                        <input type="text" id="editTemplateName-${templateId}" value="${template.name}" required>
                    </div>
                    <div class="form-group">
                        <label>锻炼类型*</label>
                        <select id="editTemplateType-${templateId}" required>
                            <option value="">选择类型</option>
                            <option value="cardio" ${template.type === 'cardio' ? 'selected' : ''}>有氧运动</option>
                            <option value="strength" ${template.type === 'strength' ? 'selected' : ''}>力量训练</option>
                            <option value="flexibility" ${template.type === 'flexibility' ? 'selected' : ''}>柔韧性训练</option>
                            <option value="sports" ${template.type === 'sports' ? 'selected' : ''}>运动项目</option>
                            <option value="other" ${template.type === 'other' ? 'selected' : ''}>其他</option>
                        </select>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label>持续时间(分钟)*</label>
                        <input type="number" id="editTemplateDuration-${templateId}" value="${template.duration}" min="1" required>
                    </div>
                    <div class="form-group">
                        <label>运动强度*</label>
                        <select id="editTemplateIntensity-${templateId}" required>
                            <option value="">选择强度</option>
                            <option value="low" ${template.intensity === 'low' ? 'selected' : ''}>低强度</option>
                            <option value="medium" ${template.intensity === 'medium' ? 'selected' : ''}>中等强度</option>
                            <option value="high" ${template.intensity === 'high' ? 'selected' : ''}>高强度</option>
                        </select>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label>负重 (kg)</label>
                        <input type="number" id="editTemplateWeight-${templateId}" value="${template.weight || 0}" min="0" step="0.5">
                    </div>
                    <div class="form-group">
                        <label>备注</label>
                        <input type="text" id="editTemplateNotes-${templateId}" value="${template.notes || ''}">
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label>消耗卡路里</label>
                        <div class="calories-input-group">
                            <input type="number" id="editTemplateCalories-${templateId}" value="${template.calories || 0}" min="0" placeholder="自动计算">
                            <button type="button" class="btn-secondary" onclick="calculateEditTemplateCalories('${templateId}')">计算</button>
                        </div>
                        <div class="met-display" id="editTemplateMETDisplay-${templateId}" style="display: none;">
                            <span class="met-info">MET: <strong id="editTemplateMETValue-${templateId}">--</strong></span>
                            <span class="met-description" id="editTemplateMETDescription-${templateId}">--</span>
                        </div>
                    </div>
                    <div class="form-group">
                        <label>锻炼部位</label>
                        <div id="editTemplateBodyParts-${templateId}" class="body-parts-checkboxes">
                            <label><input type="checkbox" name="body_parts" value="胸肌"> 胸肌</label>
                            <label><input type="checkbox" name="body_parts" value="肱三头肌"> 肱三头肌</label>
                            <label><input type="checkbox" name="body_parts" value="大腿"> 大腿</label>
                            <label><input type="checkbox" name="body_parts" value="背部"> 背部</label>
                            <label><input type="checkbox" name="body_parts" value="肱二头肌"> 肱二头肌</label>
                            <label><input type="checkbox" name="body_parts" value="腹肌"> 腹肌</label>
                            <label><input type="checkbox" name="body_parts" value="脊柱"> 脊柱</label>
                            <label><input type="checkbox" name="body_parts" value="肩膀"> 肩膀</label>
                        </div>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label>&nbsp;</label>
                        <div class="weight-info">
                            <small>💡 负重将加到体重上用于卡路里计算</small>
                        </div>
                    </div>
                </div>
                <div class="form-buttons">
                    <button type="button" class="btn-primary" onclick="saveEditTemplate('${templateId}')">保存模板</button>
                    <button type="button" class="btn-secondary" onclick="cancelEditTemplate('${templateId}')">取消</button>
                </div>
            </form>
        </div>
    `;
    
    // 在模板项目之前插入编辑表单
    templateItem.insertAdjacentHTML('beforebegin', editFormHTML);
    
    // 设置锻炼部位多选
    const allCbs = document.querySelectorAll(`#editTemplateBodyParts-${templateId} input[name='body_parts']`);
    allCbs.forEach(cb => {
        cb.checked = (template.body_parts || []).includes(cb.value);
    });
    
    // 添加事件监听器用于自动计算和MET显示
    setupEditFormListeners(templateId);
    
    // 初始化MET显示
    updateEditTemplateMETDisplay(templateId);
    
    // 滚动到编辑表单
    const editForm = document.querySelector(`[data-template-id="${templateId}"]`);
    editForm.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

// 统计分析函数
function changePeriod(period) {
    currentPeriod = period;
    
    // 更新按钮状态
    document.querySelectorAll('.period-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    document.querySelector(`[data-period="${period}"]`).classList.add('active');
    
    // 显示/隐藏月份选择
    const monthSelect = document.getElementById('statsMonth');
    if (period === 'month') {
        monthSelect.style.display = 'inline-block';
    } else {
        monthSelect.style.display = 'none';
    }
    
    updateStats();
}

async function updateStats() {
    try {
        let url = `/api/exercise-stats?period=${currentPeriod}`;
        
        const year = document.getElementById('statsYear').value;
        url += `&year=${year}`;
        
        if (currentPeriod === 'month') {
            const month = document.getElementById('statsMonth').value;
            url += `&month=${month}`;
        } else if (currentPeriod === 'week') {
            url += `&date=${currentDate}`;
        }
        
        const response = await fetch(url);
        const stats = await response.json();
        
        renderStats(stats);
    } catch (error) {
        console.error('加载统计数据失败:', error);
        showToast('加载统计数据失败', 'error');
    }
}

function renderStats(stats) {
    document.getElementById('statsTotalDays').textContent = stats.total_days || 0;
    document.getElementById('statsExerciseDays').textContent = stats.exercise_days || 0;
    document.getElementById('statsTotalDuration').textContent = stats.total_duration || 0;
    document.getElementById('statsTotalCalories').textContent = stats.total_calories || 0;
    document.getElementById('statsConsistency').textContent = (stats.consistency || 0).toFixed(1) + '%';
    document.getElementById('statsWeeklyAvg').textContent = (stats.weekly_avg || 0).toFixed(1);
    
    // 渲染类型统计
    renderTypeStats(stats.type_stats || {});
}

function renderTypeStats(typeStats) {
    const container = document.getElementById('typeStatsChart');
    
    if (Object.keys(typeStats).length === 0) {
        container.innerHTML = '<div class="empty-state"><p>暂无数据</p></div>';
        return;
    }
    
    container.innerHTML = Object.entries(typeStats).map(([type, count]) => `
        <div class="type-chart-item">
            <span class="type-name">${getTypeLabel(type)}</span>
            <span class="type-count">${count}次</span>
        </div>
    `).join('');
}

// 工具函数
function getTypeLabel(type) {
    const types = {
        'cardio': '有氧运动',
        'strength': '力量训练',
        'flexibility': '柔韧性训练',
        'sports': '运动项目',
        'other': '其他'
    };
    return types[type] || type;
}

function getIntensityLabel(intensity) {
    const intensities = {
        'low': '低强度',
        'medium': '中等强度',
        'high': '高强度'
    };
    return intensities[intensity] || intensity;
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    
    container.appendChild(toast);
    
    setTimeout(() => {
        toast.style.animation = 'slideOut 0.3s ease-out';
        setTimeout(() => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        }, 300);
    }, 3000);
}

// 全局函数声明 - 确保这些函数可以在HTML中直接调用
window.showExerciseView = showExerciseView;
window.showTemplateView = showTemplateView;
window.showCollectionView = showCollectionView;
window.showProfileView = showProfileView;
window.showStatsView = showStatsView;
window.goToToday = goToToday;
window.showAddForm = showAddForm;
window.hideAddForm = hideAddForm;
window.addFromTemplate = addFromTemplate;
window.addFromCollection = addFromCollection;
window.editExercise = editExercise;
window.deleteExercise = deleteExercise;
window.toggleExercise = toggleExercise;
window.editTemplate = editTemplate;
window.deleteTemplate = deleteTemplate;
window.resetTemplateForm = resetTemplateForm;
window.editCollection = editCollection;
window.deleteCollection = deleteCollection;
window.resetCollectionForm = resetCollectionForm;
window.changePeriod = changePeriod;
window.updateStats = updateStats;
window.calculateEditTemplateCalories = calculateEditTemplateCalories;
window.saveEditTemplate = saveEditTemplate;
window.cancelEditTemplate = cancelEditTemplate;

// 模板集合管理函数
async function loadCollections() {
    try {
        const response = await fetch('/api/exercise-collections');
        const collections = await response.json();
        
        renderCollections(collections || []);
        updateCollectionSelect(collections || []);
    } catch (error) {
        console.error('加载集合失败:', error);
        showToast('加载集合失败', 'error');
    }
}

function renderCollections(collections) {
    const container = document.getElementById('collectionItems');
    const emptyState = document.getElementById('collectionEmpty');
    
    if (collections.length === 0) {
        container.innerHTML = '';
        emptyState.style.display = 'block';
        return;
    }
    
    emptyState.style.display = 'none';
    container.innerHTML = collections.map(collection => `
        <div class="collection-item" data-id="${collection.id}">
            <div class="collection-header">
                <div>
                    <div class="collection-name">${collection.name}</div>
                    ${collection.description ? `<div class="collection-description">${collection.description}</div>` : ''}
                </div>
                <div class="collection-actions">
                    <button class="btn-secondary" onclick="editCollection('${collection.id}')" title="编辑">✏️</button>
                    <button class="btn-danger" onclick="deleteCollection('${collection.id}')" title="删除">🗑️</button>
                </div>
            </div>
            <div class="collection-templates" id="collection-templates-${collection.id}">
                <!-- 模板标签将在这里显示 -->
            </div>
            <div class="collection-meta">
                创建时间: ${new Date(collection.created_at).toLocaleDateString('zh-CN')}
            </div>
        </div>
    `).join('');
    
    // 加载每个集合的模板信息
    collections.forEach(collection => {
        loadCollectionTemplates(collection.id);
    });
}

async function loadCollectionTemplates(collectionId) {
    try {
        const response = await fetch(`/api/exercise-collections/details?id=${collectionId}`);
        const data = await response.json();
        
        const container = document.getElementById(`collection-templates-${collectionId}`);
        if (container && data.templates) {
            container.innerHTML = data.templates.map(template => 
                `<span class="collection-template-tag">${template.name}</span>`
            ).join('');
        }
    } catch (error) {
        console.error('加载集合模板失败:', error);
    }
}

function updateCollectionSelect(collections) {
    const select = document.getElementById('collectionSelect');
    select.innerHTML = '<option value="">选择集合</option>' + 
        collections.map(collection => `<option value="${collection.id}">${collection.name}</option>`).join('');
}

async function loadTemplatesForCheckboxes() {
    try {
        const response = await fetch('/api/exercise-templates');
        const templates = await response.json();
        
        const container = document.getElementById('templateCheckboxes');
        container.innerHTML = templates.map(template => `
            <div class="template-checkbox">
                <input type="checkbox" id="template-${template.id}" value="${template.id}">
                <label for="template-${template.id}" class="template-checkbox-label">${template.name}</label>
            </div>
        `).join('');
    } catch (error) {
        console.error('加载模板复选框失败:', error);
        showToast('加载模板复选框失败', 'error');
    }
}

async function addFromCollection() {
    const collectionId = document.getElementById('collectionSelect').value;
    if (!collectionId) {
        showToast('请选择一个集合', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/exercise-collections/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                date: currentDate,
                collection_id: collectionId
            })
        });
        
        if (response.ok) {
            showToast('从集合添加锻炼成功', 'success');
            document.getElementById('collectionSelect').value = '';
            loadExercises();
        } else {
            throw new Error('添加失败');
        }
    } catch (error) {
        console.error('从集合添加锻炼失败:', error);
        showToast('从集合添加锻炼失败', 'error');
    }
}

async function saveCollection() {
    const formData = {
        name: document.getElementById('collectionName').value,
        description: document.getElementById('collectionDescription').value,
        template_ids: []
    };
    
    // 获取选中的模板ID
    const checkboxes = document.querySelectorAll('#templateCheckboxes input[type="checkbox"]:checked');
    formData.template_ids = Array.from(checkboxes).map(cb => cb.value);
    
    if (formData.template_ids.length === 0) {
        showToast('请至少选择一个模板', 'error');
        return;
    }
    
    try {
        let response;
        if (isCollectionEditing) {
            formData.id = editingCollectionId;
            response = await fetch('/api/exercise-collections', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData)
            });
        } else {
            response = await fetch('/api/exercise-collections', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData)
            });
        }
        
        if (response.ok) {
            showToast(isCollectionEditing ? '集合更新成功' : '集合添加成功', 'success');
            resetCollectionForm();
            loadCollections();
        } else {
            throw new Error('保存失败');
        }
    } catch (error) {
        console.error('保存集合失败:', error);
        showToast('保存集合失败', 'error');
    }
}

async function editCollection(id) {
    try {
        const response = await fetch(`/api/exercise-collections/details?id=${id}`);
        const data = await response.json();
        
        if (data.collection) {
            const collection = data.collection;
            
            document.getElementById('collectionId').value = collection.id;
            document.getElementById('collectionName').value = collection.name;
            document.getElementById('collectionDescription').value = collection.description || '';
            
            // 清除所有复选框选择
            document.querySelectorAll('#templateCheckboxes input[type="checkbox"]').forEach(cb => {
                cb.checked = false;
            });
            
            // 选中集合中的模板
            collection.template_ids.forEach(templateId => {
                const checkbox = document.getElementById(`template-${templateId}`);
                if (checkbox) {
                    checkbox.checked = true;
                }
            });
            
            document.getElementById('collectionFormTitle').textContent = '编辑集合';
            isCollectionEditing = true;
            editingCollectionId = id;
        }
    } catch (error) {
        console.error('加载集合数据失败:', error);
        showToast('加载集合数据失败', 'error');
    }
}

async function deleteCollection(id) {
    if (!confirm('确定要删除这个集合吗？')) {
        return;
    }
    
    try {
        const response = await fetch('/api/exercise-collections', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
        });
        
        if (response.ok) {
            showToast('集合删除成功', 'success');
            loadCollections();
        } else {
            throw new Error('删除失败');
        }
    } catch (error) {
        console.error('删除集合失败:', error);
        showToast('删除集合失败', 'error');
    }
}

function resetCollectionForm() {
    const form = document.getElementById('collectionFormElement');
    if (form) {
        form.reset();
    }
    const idField = document.getElementById('collectionId');
    if (idField) {
        idField.value = '';
    }
    const formTitle = document.getElementById('collectionFormTitle');
    if (formTitle) {
        formTitle.textContent = '添加集合';
    }
    
    // 清除所有模板复选框选择
    const checkboxes = document.querySelectorAll('#templateCheckboxes input[type="checkbox"]');
    checkboxes.forEach(cb => {
        cb.checked = false;
    });
    
    // 重置编辑状态
    isCollectionEditing = false;
    editingCollectionId = null;
}

// 个人信息管理函数
async function loadUserProfile() {
    try {
        const response = await fetch('/api/exercise-profile');
        const profile = await response.json();
        
        if (profile && profile.name) {
            document.getElementById('profileName').value = profile.name || '';
            document.getElementById('profileGender').value = profile.gender || '';
            document.getElementById('profileWeight').value = profile.weight || '';
            document.getElementById('profileHeight').value = profile.height || '';
            document.getElementById('profileAge').value = profile.age || '';
            
            calculateBMI();
        }
    } catch (error) {
        console.error('加载用户信息失败:', error);
        showToast('加载用户信息失败', 'error');
    }
}

async function saveUserProfile() {
    const formData = {
        name: document.getElementById('profileName').value,
        gender: document.getElementById('profileGender').value,
        weight: parseFloat(document.getElementById('profileWeight').value),
        height: parseFloat(document.getElementById('profileHeight').value),
        age: parseInt(document.getElementById('profileAge').value) || 0
    };
    
    if (!formData.name || !formData.gender || !formData.weight) {
        showToast('请填写必要信息（姓名、性别、体重）', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/exercise-profile', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(formData)
        });
        
        if (response.ok) {
            showToast('个人信息保存成功', 'success');
            calculateBMI();
        } else {
            throw new Error('保存失败');
        }
    } catch (error) {
        console.error('保存个人信息失败:', error);
        showToast('保存个人信息失败', 'error');
    }
}

function resetProfileForm() {
    document.getElementById('profileFormElement').reset();
    document.getElementById('bmiDisplay').innerHTML = `
        <span class="bmi-label">BMI: </span>
        <span class="bmi-value">--</span>
        <span class="bmi-status">--</span>
    `;
}

function calculateBMI() {
    const weight = parseFloat(document.getElementById('profileWeight').value);
    const height = parseFloat(document.getElementById('profileHeight').value);
    
    if (weight && height && weight > 0 && height > 0) {
        const bmi = weight / Math.pow(height / 100, 2);
        const bmiValue = bmi.toFixed(1);
        
        let status = '';
        let statusClass = '';
        
        if (bmi < 18.5) {
            status = '偏瘦';
            statusClass = 'underweight';
        } else if (bmi < 24) {
            status = '正常';
            statusClass = 'normal';
        } else if (bmi < 28) {
            status = '超重';
            statusClass = 'overweight';
        } else {
            status = '肥胖';
            statusClass = 'obese';
        }
        
        document.getElementById('bmiDisplay').innerHTML = `
            <span class="bmi-label">BMI: </span>
            <span class="bmi-value">${bmiValue}</span>
            <span class="bmi-status ${statusClass}">${status}</span>
        `;
    } else {
        document.getElementById('bmiDisplay').innerHTML = `
            <span class="bmi-label">BMI: </span>
            <span class="bmi-value">--</span>
            <span class="bmi-status">--</span>
        `;
    }
}

async function loadMETValues() {
    try {
        const response = await fetch('/api/exercise-met-values');
        const metValues = await response.json();
        
        renderMETTable(metValues);
    } catch (error) {
        console.error('加载MET值失败:', error);
        showToast('加载MET值失败', 'error');
    }
}

function renderMETTable(metValues) {
    const container = document.getElementById('metValuesTable');
    
    // 按运动类型分组
    const groupedMET = {};
    metValues.forEach(met => {
        if (!groupedMET[met.exercise_type]) {
            groupedMET[met.exercise_type] = [];
        }
        groupedMET[met.exercise_type].push(met);
    });
    
    const typeLabels = {
        'cardio': '有氧运动',
        'strength': '力量训练',
        'flexibility': '柔韧性训练',
        'sports': '运动项目',
        'other': '其他'
    };
    
    container.innerHTML = Object.entries(groupedMET).map(([type, values]) => `
        <div class="met-category">
            <div class="met-category-title">${typeLabels[type] || type}</div>
            ${values.map(met => `
                <div class="met-item">
                    <span class="met-description">${met.description}</span>
                    <span class="met-value">${met.met}</span>
                </div>
            `).join('')}
        </div>
    `).join('');
}

async function calculateCalories() {
    const exerciseType = document.getElementById('exerciseType').value;
    const intensity = document.getElementById('exerciseIntensity').value;
    const duration = parseInt(document.getElementById('exerciseDuration').value);
    const exerciseWeight = parseFloat(document.getElementById('exerciseWeight').value) || 0;
    
    if (!exerciseType || !intensity || !duration) {
        showToast('请先填写锻炼类型、强度和时长', 'error');
        return;
    }
    
    // 获取用户体重，如果没有则使用标准体重70kg
    const profile = await getUserProfile();
    const baseWeight = (profile && profile.weight) ? profile.weight : 70;
    const totalWeight = baseWeight + exerciseWeight; // 体重加负重
    
    try {
        const response = await fetch('/api/exercise-calculate-calories', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                exercise_type: exerciseType,
                intensity: intensity,
                duration: duration,
                weight: totalWeight
            })
        });
        
        const data = await response.json();
        document.getElementById('exerciseCalories').value = data.calories;
        
        const weightSource = (profile && profile.weight) ? `您的体重 ${baseWeight}kg` : `标准体重 ${baseWeight}kg`;
        const weightInfo = exerciseWeight > 0 ? ` + 负重 ${exerciseWeight}kg = ${totalWeight}kg` : '';
        showToast(`计算结果：${data.calories} kcal (${weightSource}${weightInfo})`, 'success');
    } catch (error) {
        console.error('计算卡路里失败:', error);
        showToast('计算卡路里失败', 'error');
    }
}

async function getUserProfile() {
    try {
        const response = await fetch('/api/exercise-profile');
        return await response.json();
    } catch (error) {
        console.error('获取用户信息失败:', error);
        return null;
    }
}

// 模板卡路里计算函数
async function calculateTemplateCalories() {
    const exerciseType = document.getElementById('templateType').value;
    const intensity = document.getElementById('templateIntensity').value;
    const duration = parseInt(document.getElementById('templateDuration').value);
    const templateWeight = parseFloat(document.getElementById('templateWeight').value) || 0;
    
    if (!exerciseType || !intensity || !duration) {
        showToast('请先填写锻炼类型、强度和时长', 'error');
        return;
    }
    
    // 获取用户体重，如果没有则使用标准体重70kg
    const profile = await getUserProfile();
    const baseWeight = (profile && profile.weight) ? profile.weight : 70;
    const totalWeight = baseWeight + templateWeight;
    
    try {
        const response = await fetch('/api/exercise-calculate-calories', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                exercise_type: exerciseType,
                intensity: intensity,
                duration: duration,
                weight: totalWeight
            })
        });
        
        const data = await response.json();
        document.getElementById('templateCalories').value = data.calories;
        
        const weightSource = (profile && profile.weight) ? `您的体重 ${baseWeight}kg` : `标准体重 ${baseWeight}kg`;
        const weightInfo = templateWeight > 0 ? ` + 负重 ${templateWeight}kg = ${totalWeight}kg` : '';
        showToast(`计算结果：${data.calories} kcal (${weightSource}${weightInfo})`, 'success');
    } catch (error) {
        console.error('计算模板卡路里失败:', error);
        showToast('计算模板卡路里失败', 'error');
    }
}

// 自动计算模板卡路里（当用户修改类型、强度或时长时）
async function autoCalculateTemplateCalories() {
    const exerciseType = document.getElementById('templateType').value;
    const intensity = document.getElementById('templateIntensity').value;
    const duration = parseInt(document.getElementById('templateDuration').value);
    const templateWeight = parseFloat(document.getElementById('templateWeight').value) || 0;
    
    // 只有当所有必要信息都填写完整时才自动计算
    if (exerciseType && intensity && duration && duration > 0) {
        // 获取用户体重，如果没有则使用标准体重70kg
        const profile = await getUserProfile();
        const baseWeight = (profile && profile.weight) ? profile.weight : 70;
        const totalWeight = baseWeight + templateWeight;
        
        try {
            const response = await fetch('/api/exercise-calculate-calories', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    exercise_type: exerciseType,
                    intensity: intensity,
                    duration: duration,
                    weight: totalWeight
                })
            });
            
            const data = await response.json();
            document.getElementById('templateCalories').value = data.calories;
        } catch (error) {
            console.error('自动计算模板卡路里失败:', error);
            // 自动计算失败不显示错误提示，避免干扰用户
        }
    }
}

// 自动计算锻炼卡路里（当用户修改类型、强度、时长或负重时）
async function autoCalculateExerciseCalories() {
    const exerciseType = document.getElementById('exerciseType').value;
    const intensity = document.getElementById('exerciseIntensity').value;
    const duration = parseInt(document.getElementById('exerciseDuration').value);
    const exerciseWeight = parseFloat(document.getElementById('exerciseWeight').value) || 0;
    
    // 只有当所有必要信息都填写完整时才自动计算
    if (exerciseType && intensity && duration && duration > 0) {
        // 获取用户体重，如果没有则使用标准体重70kg
        const profile = await getUserProfile();
        const baseWeight = (profile && profile.weight) ? profile.weight : 70;
        const totalWeight = baseWeight + exerciseWeight; // 体重加负重
        
        try {
            const response = await fetch('/api/exercise-calculate-calories', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    exercise_type: exerciseType,
                    intensity: intensity,
                    duration: duration,
                    weight: totalWeight
                })
            });
            
            const data = await response.json();
            document.getElementById('exerciseCalories').value = data.calories;
        } catch (error) {
            console.error('自动计算锻炼卡路里失败:', error);
            // 自动计算失败不显示错误提示，避免干扰用户
        }
    }
}

// 批量更新所有模板的卡路里
async function updateAllTemplateCalories() {
    if (!confirm('确定要更新所有模板的卡路里吗？\n这将根据当前的MET值和体重重新计算所有模板的卡路里。')) {
        return;
    }
    
    // 获取用户体重，如果没有则使用标准体重70kg
    const profile = await getUserProfile();
    const weight = (profile && profile.weight) ? profile.weight : 70;
    
    try {
        const response = await fetch('/api/exercise-update-template-calories', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                weight: weight
            })
        });
        
        if (response.ok) {
            const data = await response.json();
            const weightSource = (profile && profile.weight) ? `使用您的体重 ${weight}kg` : `使用标准体重 ${weight}kg`;
            showToast(`${data.message} (${weightSource})`, 'success');
            
            // 重新加载模板列表
            loadTemplates();
        } else {
            throw new Error('更新失败');
        }
    } catch (error) {
        console.error('批量更新模板卡路里失败:', error);
        showToast('批量更新模板卡路里失败', 'error');
    }
}

// 批量更新所有锻炼记录的卡路里
async function updateAllExerciseCalories() {
    if (!confirm('确定要更新所有锻炼记录的卡路里吗？\n这将根据当前的MET值和体重重新计算所有历史锻炼记录的卡路里。\n\n⚠️ 注意：这个操作会影响所有日期的锻炼记录，请谨慎操作！')) {
        return;
    }
    
    // 获取用户体重，如果没有则使用标准体重70kg
    const profile = await getUserProfile();
    const weight = (profile && profile.weight) ? profile.weight : 70;
    
    try {
        showToast('正在更新锻炼记录，请稍候...', 'info');
        
        const response = await fetch('/api/exercise-update-exercise-calories', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                weight: weight
            })
        });
        
        if (response.ok) {
            const data = await response.json();
            const weightSource = (profile && profile.weight) ? `使用您的体重 ${weight}kg` : `使用标准体重 ${weight}kg`;
            showToast(`${data.message} (${weightSource})`, 'success');
            
            // 重新加载当前日期的锻炼列表
            loadExercises();
            // 重新加载统计数据
            updateStats();
        } else {
            throw new Error('更新失败');
        }
    } catch (error) {
        console.error('批量更新锻炼记录卡路里失败:', error);
        showToast('批量更新锻炼记录卡路里失败', 'error');
    }
}

// 更新模板MET值显示
async function updateTemplateMETDisplay() {
    const exerciseType = document.getElementById('templateType').value;
    const intensity = document.getElementById('templateIntensity').value;
    
    const display = document.getElementById('templateMETDisplay');
    const valueElement = document.getElementById('templateMETValue');
    const descElement = document.getElementById('templateMETDescription');
    
    if (!exerciseType || !intensity) {
        display.style.display = 'none';
        return;
    }
    
    try {
        const response = await fetch(`/api/exercise-get-met-value?type=${exerciseType}&intensity=${intensity}`);
        if (response.ok) {
            const data = await response.json();
            valueElement.textContent = data.met;
            descElement.textContent = data.description;
            display.style.display = 'block';
        } else {
            display.style.display = 'none';
        }
    } catch (error) {
        console.error('获取MET值失败:', error);
        display.style.display = 'none';
    }
}

// 更新锻炼MET值显示
async function updateExerciseMETDisplay() {
    const exerciseType = document.getElementById('exerciseType').value;
    const intensity = document.getElementById('exerciseIntensity').value;
    
    const display = document.getElementById('exerciseMETDisplay');
    const valueElement = document.getElementById('exerciseMETValue');
    const descElement = document.getElementById('exerciseMETDescription');
    
    if (!exerciseType || !intensity) {
        display.style.display = 'none';
        return;
    }
    
    try {
        const response = await fetch(`/api/exercise-get-met-value?type=${exerciseType}&intensity=${intensity}`);
        if (response.ok) {
            const data = await response.json();
            valueElement.textContent = data.met;
            descElement.textContent = data.description;
            display.style.display = 'block';
        } else {
            display.style.display = 'none';
        }
    } catch (error) {
        console.error('获取MET值失败:', error);
        display.style.display = 'none';
    }
}

// 隐藏MET值显示
function hideMETDisplay(prefix) {
    const display = document.getElementById(prefix + 'METDisplay');
    if (display) {
        display.style.display = 'none';
    }
}

// 设置编辑表单的事件监听器
function setupEditFormListeners(templateId) {
    const typeSelect = document.getElementById(`editTemplateType-${templateId}`);
    const intensitySelect = document.getElementById(`editTemplateIntensity-${templateId}`);
    const durationInput = document.getElementById(`editTemplateDuration-${templateId}`);
    const weightInput = document.getElementById(`editTemplateWeight-${templateId}`);
    
    if (typeSelect && intensitySelect && durationInput && weightInput) {
        typeSelect.addEventListener('change', () => autoCalculateEditTemplateCalories(templateId));
        intensitySelect.addEventListener('change', () => autoCalculateEditTemplateCalories(templateId));
        durationInput.addEventListener('input', () => autoCalculateEditTemplateCalories(templateId));
        weightInput.addEventListener('input', () => autoCalculateEditTemplateCalories(templateId));
        
        typeSelect.addEventListener('change', () => updateEditTemplateMETDisplay(templateId));
        intensitySelect.addEventListener('change', () => updateEditTemplateMETDisplay(templateId));
    }
}

// 更新编辑表单的MET显示
async function updateEditTemplateMETDisplay(templateId) {
    const exerciseType = document.getElementById(`editTemplateType-${templateId}`).value;
    const intensity = document.getElementById(`editTemplateIntensity-${templateId}`).value;
    
    const display = document.getElementById(`editTemplateMETDisplay-${templateId}`);
    const valueElement = document.getElementById(`editTemplateMETValue-${templateId}`);
    const descElement = document.getElementById(`editTemplateMETDescription-${templateId}`);
    
    if (!exerciseType || !intensity) {
        display.style.display = 'none';
        return;
    }
    
    try {
        const response = await fetch(`/api/exercise-get-met-value?type=${exerciseType}&intensity=${intensity}`);
        if (response.ok) {
            const data = await response.json();
            valueElement.textContent = data.met;
            descElement.textContent = data.description;
            display.style.display = 'block';
        } else {
            display.style.display = 'none';
        }
    } catch (error) {
        console.error('获取MET值失败:', error);
        display.style.display = 'none';
    }
}

// 自动计算编辑表单的卡路里
async function autoCalculateEditTemplateCalories(templateId) {
    const exerciseType = document.getElementById(`editTemplateType-${templateId}`).value;
    const intensity = document.getElementById(`editTemplateIntensity-${templateId}`).value;
    const duration = parseInt(document.getElementById(`editTemplateDuration-${templateId}`).value);
    const templateWeight = parseFloat(document.getElementById(`editTemplateWeight-${templateId}`).value) || 0;
    
    if (exerciseType && intensity && duration && duration > 0) {
        const profile = await getUserProfile();
        const baseWeight = (profile && profile.weight) ? profile.weight : 70;
        const totalWeight = baseWeight + templateWeight;
        
        try {
            const response = await fetch('/api/exercise-calculate-calories', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    exercise_type: exerciseType,
                    intensity: intensity,
                    duration: duration,
                    weight: totalWeight
                })
            });
            
            const data = await response.json();
            document.getElementById(`editTemplateCalories-${templateId}`).value = data.calories;
        } catch (error) {
            console.error('自动计算编辑表单卡路里失败:', error);
        }
    }
}

// 手动计算编辑表单的卡路里
async function calculateEditTemplateCalories(templateId) {
    const exerciseType = document.getElementById(`editTemplateType-${templateId}`).value;
    const intensity = document.getElementById(`editTemplateIntensity-${templateId}`).value;
    const duration = parseInt(document.getElementById(`editTemplateDuration-${templateId}`).value);
    const templateWeight = parseFloat(document.getElementById(`editTemplateWeight-${templateId}`).value) || 0;
    
    if (!exerciseType || !intensity || !duration) {
        showToast('请先填写锻炼类型、强度和时长', 'error');
        return;
    }
    
    const profile = await getUserProfile();
    const baseWeight = (profile && profile.weight) ? profile.weight : 70;
    const totalWeight = baseWeight + templateWeight;
    
    try {
        const response = await fetch('/api/exercise-calculate-calories', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                exercise_type: exerciseType,
                intensity: intensity,
                duration: duration,
                weight: totalWeight
            })
        });
        
        const data = await response.json();
        document.getElementById(`editTemplateCalories-${templateId}`).value = data.calories;
        
        const weightSource = (profile && profile.weight) ? `您的体重 ${baseWeight}kg` : `标准体重 ${baseWeight}kg`;
        const weightInfo = templateWeight > 0 ? ` + 负重 ${templateWeight}kg = ${totalWeight}kg` : '';
        showToast(`计算结果：${data.calories} kcal (${weightSource}${weightInfo})`, 'success');
    } catch (error) {
        console.error('计算编辑表单卡路里失败:', error);
        showToast('计算编辑表单卡路里失败', 'error');
    }
}

// 保存编辑表单
async function saveEditTemplate(templateId) {
    const formData = {
        id: templateId,
        name: document.getElementById(`editTemplateName-${templateId}`).value,
        type: document.getElementById(`editTemplateType-${templateId}`).value,
        duration: parseInt(document.getElementById(`editTemplateDuration-${templateId}`).value),
        intensity: document.getElementById(`editTemplateIntensity-${templateId}`).value,
        calories: parseInt(document.getElementById(`editTemplateCalories-${templateId}`).value) || 0,
        notes: document.getElementById(`editTemplateNotes-${templateId}`).value,
        weight: parseFloat(document.getElementById(`editTemplateWeight-${templateId}`).value) || 0,
        // 新增：收集锻炼部位
        body_parts: Array.from(document.querySelectorAll(`#editTemplateBodyParts-${templateId} input[name='body_parts']:checked`)).map(cb => cb.value)
    };
    
    if (!formData.name || !formData.type || !formData.duration || !formData.intensity) {
        showToast('请填写所有必填字段', 'error');
        return;
    }
    
    try {
        const response = await fetch('/api/exercise-templates', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(formData)
        });
        
        if (response.ok) {
            showToast('模板更新成功', 'success');
            hideAllEditForms();
            isTemplateEditing = false;
            editingTemplateId = null;
            loadTemplates();
        } else {
            throw new Error('保存失败');
        }
    } catch (error) {
        console.error('保存编辑模板失败:', error);
        showToast('保存编辑模板失败', 'error');
    }
}

// 取消编辑
function cancelEditTemplate(templateId) {
    hideAllEditForms();
    isTemplateEditing = false;
    editingTemplateId = null;
} 