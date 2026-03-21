// Init Agent - Web Wizard Frontend
(function() {
    'use strict';

    // Base steps (always present)
    var BASE_STEP_IDS = ['step-welcome','step-env','step-global'];
    var BASE_STEP_NAMES = ['欢迎','环境检测','全局配置'];

    // Deploy steps (conditional)
    var DEPLOY_STEP_IDS = ['step-deploy-targets','step-deploy-projects','step-deploy-pipelines'];
    var DEPLOY_STEP_NAMES = ['Deploy Targets','Deploy Projects','Deploy Pipelines'];

    // Tail steps (always present)
    var TAIL_STEP_IDS = ['step-select','step-config','step-generate','step-avail'];
    var TAIL_STEP_NAMES = ['Agent 选择','Agent 配置','配置生成','可用性面板'];

    // Dynamic step arrays (rebuilt after deploy status check)
    var STEP_IDS = [];
    var STEP_NAMES = [];
    var deployAvailable = false;

    var currentStep = 0;
    var discoveredAgents = [];
    var existingConfigs = {};
    var selectedAgents = [];
    var agentValues = {};
    var currentAgentIdx = 0;
    var skippedAgents = [];
    var ws = null;

    // Deploy state
    var deployTargets = {};
    var deploySSHPassword = '';
    var deployProjects = {};
    var deployProjectOrder = [];
    var deployPipelines = [];
    var editingTargetName = null;  // null = add, string = edit
    var editingProjectName = null;
    var editingPipelineIdx = -1;  // -1 = add, >=0 = edit
    var pipelineSteps = [];       // temp steps for pipeline editor

    // Initialize
    document.addEventListener('DOMContentLoaded', function() {
        // Check deploy status first, then build steps
        checkDeployStatus(function() {
            buildStepArrays();
            renderStepDots();
            connectWS();
            loadDiscoveredAgents();
            loadExistingConfigs();
            if (deployAvailable) {
                loadDeployTargets();
                loadDeployProjects();
                loadDeployPipelines();
            }
            showStep(0);
        });
    });

    function checkDeployStatus(callback) {
        fetch('/api/deploy/status').then(function(r){return r.json()}).then(function(data) {
            deployAvailable = data.success && data.available;
            callback();
        }).catch(function() {
            callback();
        });
    }

    function buildStepArrays() {
        STEP_IDS = BASE_STEP_IDS.slice();
        STEP_NAMES = BASE_STEP_NAMES.slice();
        if (deployAvailable) {
            STEP_IDS = STEP_IDS.concat(DEPLOY_STEP_IDS);
            STEP_NAMES = STEP_NAMES.concat(DEPLOY_STEP_NAMES);
        }
        STEP_IDS = STEP_IDS.concat(TAIL_STEP_IDS);
        STEP_NAMES = STEP_NAMES.concat(TAIL_STEP_NAMES);
    }

    // WebSocket connection
    function connectWS() {
        var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(proto + '//' + location.host + '/ws');
        ws.onmessage = function(e) {
            var msg = JSON.parse(e.data);
            handleWSMessage(msg);
        };
        ws.onclose = function() {
            setTimeout(connectWS, 3000);
        };
    }

    function handleWSMessage(msg) {
        switch(msg.type) {
            case 'env_check_result':
                appendEnvResult(msg.data);
                break;
            case 'env_check_complete':
                document.getElementById('btn-env-next').classList.remove('hidden');
                break;
            case 'avail_layer_result':
                appendAvailLayer(msg.data);
                break;
            case 'avail_complete':
                break;
            case 'config_written':
                appendWriteResult(msg.data);
                break;
        }
    }

    // Step navigation
    function renderStepDots() {
        var container = document.getElementById('steps');
        container.innerHTML = STEP_NAMES.map(function(name, i) {
            return '<div class="step-dot" id="dot-'+i+'" title="'+name+'">'+(i+1)+'</div>';
        }).join('');
    }

    function showStep(idx) {
        // Hide all step sections
        var allSections = document.querySelectorAll('main > .step');
        allSections.forEach(function(el) { el.classList.add('hidden'); });

        // Show the target step
        var targetId = STEP_IDS[idx];
        var el = document.getElementById(targetId);
        if (el) el.classList.remove('hidden');

        // Update dots
        for (var i = 0; i < STEP_NAMES.length; i++) {
            var dot = document.getElementById('dot-'+i);
            if (!dot) continue;
            dot.className = 'step-dot';
            if (i < idx) dot.classList.add('done');
            else if (i === idx) dot.classList.add('active');
        }
        currentStep = idx;

        // Trigger renders for specific steps
        if (targetId === 'step-select') renderAgentList();
        if (targetId === 'step-deploy-targets') renderTargetsList();
        if (targetId === 'step-deploy-projects') renderProjectsList();
        if (targetId === 'step-deploy-pipelines') renderPipelinesList();
    }

    window.nextStep = function() {
        if (currentStep < STEP_IDS.length - 1) {
            showStep(currentStep + 1);
        }
    };

    // Load data
    function loadDiscoveredAgents() {
        fetch('/api/agents/discovered').then(function(r){return r.json()}).then(function(data) {
            if (data.success) discoveredAgents = data.agents || [];
        });
    }

    function loadExistingConfigs() {
        fetch('/api/agents/configs').then(function(r){return r.json()}).then(function(data) {
            if (data.success) existingConfigs = data.configs || {};
        });
    }

    // Deploy data loading
    function loadDeployTargets() {
        fetch('/api/deploy/targets').then(function(r){return r.json()}).then(function(data) {
            if (data.success) {
                deployTargets = data.targets || {};
                deploySSHPassword = data.ssh_password || '';
            }
        });
    }

    function loadDeployProjects() {
        fetch('/api/deploy/projects').then(function(r){return r.json()}).then(function(data) {
            if (data.success) {
                deployProjects = data.projects || {};
                deployProjectOrder = data.order || Object.keys(deployProjects).sort();
            }
        });
    }

    function loadDeployPipelines() {
        fetch('/api/deploy/pipelines').then(function(r){return r.json()}).then(function(data) {
            if (data.success) {
                deployPipelines = data.pipelines || [];
            }
        });
    }

    // Step 2: Environment check
    window.runEnvCheck = function() {
        document.getElementById('env-results').innerHTML = '<span class="spinner"></span> 正在检测...';
        document.getElementById('btn-env-check').disabled = true;
        fetch('/api/env/check', {method:'POST'});
    };

    function appendEnvResult(r) {
        var container = document.getElementById('env-results');
        if (container.querySelector('.spinner')) container.innerHTML = '';
        var cls = r.installed ? (r.meets_requirement ? 'status-green' : 'status-yellow') : 'status-red';
        var icon = r.installed ? (r.meets_requirement ? '✓' : '!') : '✗';
        var detail = '';
        if (!r.installed) {
            detail = '未安装';
            if (r.install_hint) detail += ' — ' + r.install_hint;
        } else if (!r.meets_requirement) {
            detail = 'v' + r.version + ' < 要求 ' + r.min_version;
        } else {
            detail = r.version ? 'v'+r.version : '已安装';
            if (r.path) detail += '  ('+r.path+')';
        }
        container.innerHTML += '<div class="result-item"><span class="icon '+cls+'">'+icon+'</span>'
            +'<span class="name">'+r.software+'</span>'
            +'<span class="detail">'+detail+'</span></div>';
    }

    // Step 3: Global config
    window.saveGlobalAndNext = function() {
        var form = document.getElementById('global-form');
        var inputs = form.querySelectorAll('input');
        inputs.forEach(function(input) {
            agentValues['__shared__'] = agentValues['__shared__'] || {};
            agentValues['__shared__'][input.name] = input.value;
        });
        nextStep();
    };

    // ── Deploy Targets UI ──

    function renderTargetsList() {
        var container = document.getElementById('targets-list');
        var names = Object.keys(deployTargets).sort();
        if (names.length === 0) {
            container.innerHTML = '<div style="color:#8b949e;padding:8px">暂无 target</div>';
        } else {
            container.innerHTML = names.map(function(name, i) {
                var t = deployTargets[name];
                var info = t.type === 'bridge'
                    ? (t.bridge_url || '') + ' (' + (t.platform||'linux') + ', bridge)'
                    : (t.host||'') + ':' + (t.ssh_port||22) + ' (' + (t.platform||'linux') + ', ' + (t.type||'ssh') + ')';
                return '<div class="result-item">'
                    +'<span class="icon status-green">'+(i+1)+'</span>'
                    +'<span class="name">'+name+'</span>'
                    +'<span class="detail">'+escapeAttr(info)+'</span>'
                    +'<button class="btn btn-secondary" style="margin-left:8px;padding:2px 8px" onclick="editTarget(\''+name+'\')">编辑</button>'
                    +'<button class="btn btn-secondary" style="margin-left:4px;padding:2px 8px" onclick="deleteTarget(\''+name+'\')">删除</button>'
                    +'</div>';
            }).join('');
        }
        // Set SSH password field
        var pwInput = document.getElementById('deploy-ssh-password');
        if (pwInput && deploySSHPassword) pwInput.value = deploySSHPassword;
    }

    window.addTarget = function() {
        editingTargetName = null;
        document.getElementById('target-edit-title').textContent = '添加 Target';
        document.getElementById('target-name').value = '';
        document.getElementById('target-name').disabled = false;
        document.getElementById('target-host').value = '';
        document.getElementById('target-port').value = '22';
        document.getElementById('target-platform').value = 'linux';
        document.getElementById('target-type').value = 'ssh';
        document.getElementById('target-edit-form').classList.remove('hidden');
    };

    window.editTarget = function(name) {
        editingTargetName = name;
        var t = deployTargets[name] || {};
        document.getElementById('target-edit-title').textContent = '编辑 Target: ' + name;
        document.getElementById('target-name').value = name;
        document.getElementById('target-name').disabled = true;
        document.getElementById('target-host').value = t.host || '';
        document.getElementById('target-port').value = String(t.ssh_port || 22);
        document.getElementById('target-platform').value = t.platform || 'linux';
        document.getElementById('target-type').value = t.type || 'ssh';
        document.getElementById('target-edit-form').classList.remove('hidden');
    };

    window.deleteTarget = function(name) {
        if (!confirm('确认删除 target: ' + name + '?')) return;
        delete deployTargets[name];
        renderTargetsList();
    };

    window.saveTargetForm = function() {
        var name = document.getElementById('target-name').value.trim();
        if (!name) { alert('名称不能为空'); return; }
        var t = {
            host: document.getElementById('target-host').value.trim(),
            ssh_port: parseInt(document.getElementById('target-port').value) || 22,
            platform: document.getElementById('target-platform').value.trim() || 'linux',
            type: document.getElementById('target-type').value.trim() || 'ssh'
        };
        deployTargets[name] = t;
        document.getElementById('target-edit-form').classList.add('hidden');
        renderTargetsList();
    };

    window.cancelTargetForm = function() {
        document.getElementById('target-edit-form').classList.add('hidden');
    };

    window.saveTargetsAndNext = function() {
        deploySSHPassword = document.getElementById('deploy-ssh-password').value;
        fetch('/api/deploy/targets', {
            method: 'POST',
            headers: {'Content-Type':'application/json'},
            body: JSON.stringify({targets: deployTargets, ssh_password: deploySSHPassword})
        }).then(function(r){return r.json()}).then(function(data) {
            if (data.success) nextStep();
            else alert('保存失败: ' + data.error);
        });
    };

    // ── Deploy Projects UI ──

    function renderProjectsList() {
        var container = document.getElementById('projects-list');
        var order = deployProjectOrder.length > 0 ? deployProjectOrder : Object.keys(deployProjects).sort();
        if (order.length === 0) {
            container.innerHTML = '<div style="color:#8b949e;padding:8px">没有项目配置</div>';
            return;
        }
        container.innerHTML = order.map(function(name, i) {
            var proj = deployProjects[name];
            if (!proj) return '';
            var pattern = proj.pack_pattern || name + '_{date}.zip';
            return '<div class="result-item">'
                +'<span class="icon status-green">'+(i+1)+'</span>'
                +'<span class="name">'+name+'</span>'
                +'<span class="detail">'+escapeAttr(pattern)+'</span>'
                +'<button class="btn btn-secondary" style="margin-left:8px;padding:2px 8px" onclick="editProject(\''+name+'\')">编辑</button>'
                +'<button class="btn btn-secondary" style="margin-left:4px;padding:2px 8px" onclick="viewProject(\''+name+'\')">查看</button>'
                +'</div>';
        }).join('');
    }

    window.editProject = function(name) {
        editingProjectName = name;
        var proj = deployProjects[name];
        if (!proj) return;
        document.getElementById('project-edit-title').textContent = '编辑项目: ' + name;
        document.getElementById('proj-pack-pattern').value = proj.pack_pattern || '';
        document.getElementById('proj-protect-files').value = (proj.protect_files || []).join(',');
        document.getElementById('proj-setup-dirs').value = (proj.setup_dirs || []).join(',');

        // Render target fields for SSH targets
        var targetFields = document.getElementById('proj-target-fields');
        var tnames = Object.keys(deployTargets).sort();
        var targets = proj.targets || {};
        targetFields.innerHTML = tnames.map(function(tname) {
            var pt = targets[tname] || {};
            return '<div style="margin:8px 0;padding:8px;background:#161b22;border-radius:4px">'
                +'<strong>Target: '+tname+'</strong>'
                +'<div class="form-group"><label>remote_dir</label><input type="text" id="proj-t-'+tname+'-dir" value="'+escapeAttr(pt.remote_dir||'')+'"></div>'
                +'<div class="form-group"><label>remote_script</label><input type="text" id="proj-t-'+tname+'-script" value="'+escapeAttr(pt.remote_script||'')+'"></div>'
                +'</div>';
        }).join('');

        document.getElementById('project-edit-panel').classList.remove('hidden');
    };

    window.viewProject = function(name) {
        var proj = deployProjects[name];
        if (!proj) return;
        alert(JSON.stringify(proj, null, 2));
    };

    window.saveProjectEdit = function() {
        var name = editingProjectName;
        if (!name) return;
        var proj = deployProjects[name];
        if (!proj) return;

        proj.pack_pattern = document.getElementById('proj-pack-pattern').value.trim();
        var pf = document.getElementById('proj-protect-files').value.trim();
        proj.protect_files = pf ? pf.split(',').map(function(s){return s.trim()}).filter(function(s){return s}) : [];
        var sd = document.getElementById('proj-setup-dirs').value.trim();
        proj.setup_dirs = sd ? sd.split(',').map(function(s){return s.trim()}).filter(function(s){return s}) : [];

        // Collect target fields
        var tnames = Object.keys(deployTargets).sort();
        if (!proj.targets) proj.targets = {};
        tnames.forEach(function(tname) {
            var dirInput = document.getElementById('proj-t-'+tname+'-dir');
            var scriptInput = document.getElementById('proj-t-'+tname+'-script');
            if (dirInput && scriptInput) {
                if (!proj.targets[tname]) proj.targets[tname] = {};
                proj.targets[tname].remote_dir = dirInput.value.trim();
                proj.targets[tname].remote_script = scriptInput.value.trim();
            }
        });

        document.getElementById('project-edit-panel').classList.add('hidden');
        renderProjectsList();
    };

    window.cancelProjectEdit = function() {
        document.getElementById('project-edit-panel').classList.add('hidden');
    };

    window.saveProjectsAndNext = function() {
        fetch('/api/deploy/projects', {
            method: 'POST',
            headers: {'Content-Type':'application/json'},
            body: JSON.stringify({projects: deployProjects, order: deployProjectOrder})
        }).then(function(r){return r.json()}).then(function(data) {
            if (data.success) nextStep();
            else alert('保存失败: ' + data.error);
        });
    };

    // ── Deploy Pipelines UI ──

    function renderPipelinesList() {
        var container = document.getElementById('pipelines-list');
        if (deployPipelines.length === 0) {
            container.innerHTML = '<div style="color:#8b949e;padding:8px">暂无 pipeline</div>';
            return;
        }
        container.innerHTML = deployPipelines.map(function(p, i) {
            return '<div class="result-item">'
                +'<span class="icon status-green">'+(i+1)+'</span>'
                +'<span class="name">'+p.name+'</span>'
                +'<span class="detail">'+escapeAttr(p.description||'')+' ('+p.steps.length+' steps)</span>'
                +'<button class="btn btn-secondary" style="margin-left:8px;padding:2px 8px" onclick="editPipeline('+i+')">编辑</button>'
                +'<button class="btn btn-secondary" style="margin-left:4px;padding:2px 8px" onclick="viewPipelineDetail('+i+')">查看</button>'
                +'<button class="btn btn-secondary" style="margin-left:4px;padding:2px 8px" onclick="deletePipeline('+i+')">删除</button>'
                +'</div>';
        }).join('');
    }

    window.addPipeline = function() {
        editingPipelineIdx = -1;
        pipelineSteps = [];
        document.getElementById('pipeline-edit-title').textContent = '新建 Pipeline';
        document.getElementById('pipe-name').value = '';
        document.getElementById('pipe-name').disabled = false;
        document.getElementById('pipe-desc').value = '';
        renderPipelineStepsEditor();
        document.getElementById('pipeline-edit-panel').classList.remove('hidden');
    };

    window.editPipeline = function(idx) {
        editingPipelineIdx = idx;
        var p = deployPipelines[idx];
        pipelineSteps = (p.steps || []).map(function(s) { return {project:s.project, target:s.target||'', build_platform:s.build_platform||''}; });
        document.getElementById('pipeline-edit-title').textContent = '编辑 Pipeline: ' + p.name;
        document.getElementById('pipe-name').value = p.name;
        document.getElementById('pipe-name').disabled = true;
        document.getElementById('pipe-desc').value = p.description || '';
        renderPipelineStepsEditor();
        document.getElementById('pipeline-edit-panel').classList.remove('hidden');
    };

    window.viewPipelineDetail = function(idx) {
        var p = deployPipelines[idx];
        alert(JSON.stringify(p, null, 2));
    };

    window.deletePipeline = function(idx) {
        if (!confirm('确认删除 pipeline: ' + deployPipelines[idx].name + '?')) return;
        deployPipelines.splice(idx, 1);
        renderPipelinesList();
    };

    function renderPipelineStepsEditor() {
        var container = document.getElementById('pipe-steps-list');
        if (pipelineSteps.length === 0) {
            container.innerHTML = '<div style="color:#8b949e;padding:4px">暂无步骤</div>';
            return;
        }
        container.innerHTML = pipelineSteps.map(function(s, i) {
            return '<div style="display:flex;gap:8px;align-items:center;margin:4px 0">'
                +'<span>['+(i+1)+']</span>'
                +'<input type="text" placeholder="project" value="'+escapeAttr(s.project)+'" onchange="updatePipeStep('+i+',\'project\',this.value)" style="flex:2">'
                +'<input type="text" placeholder="target" value="'+escapeAttr(s.target)+'" onchange="updatePipeStep('+i+',\'target\',this.value)" style="flex:1">'
                +'<input type="text" placeholder="platform" value="'+escapeAttr(s.build_platform)+'" onchange="updatePipeStep('+i+',\'build_platform\',this.value)" style="flex:1">'
                +'<button class="btn btn-secondary" style="padding:2px 6px" onclick="removePipeStep('+i+')">×</button>'
                +'</div>';
        }).join('');
    }

    window.addPipelineStep = function() {
        pipelineSteps.push({project:'', target:'ssh-prod', build_platform:'linux'});
        renderPipelineStepsEditor();
    };

    window.updatePipeStep = function(idx, field, value) {
        pipelineSteps[idx][field] = value;
    };

    window.removePipeStep = function(idx) {
        pipelineSteps.splice(idx, 1);
        renderPipelineStepsEditor();
    };

    window.savePipelineEdit = function() {
        var name = document.getElementById('pipe-name').value.trim();
        if (!name) { alert('名称不能为空'); return; }
        var validSteps = pipelineSteps.filter(function(s) { return s.project.trim(); });
        if (validSteps.length === 0) { alert('至少添加一个步骤'); return; }

        var p = {
            name: name,
            description: document.getElementById('pipe-desc').value.trim(),
            steps: validSteps
        };

        if (editingPipelineIdx >= 0) {
            deployPipelines[editingPipelineIdx] = p;
        } else {
            deployPipelines.push(p);
        }

        document.getElementById('pipeline-edit-panel').classList.add('hidden');
        renderPipelinesList();
    };

    window.cancelPipelineEdit = function() {
        document.getElementById('pipeline-edit-panel').classList.add('hidden');
    };

    window.savePipelinesAndNext = function() {
        fetch('/api/deploy/pipelines', {
            method: 'POST',
            headers: {'Content-Type':'application/json'},
            body: JSON.stringify({pipelines: deployPipelines})
        }).then(function(r){return r.json()}).then(function(data) {
            if (data.success) nextStep();
            else alert('保存失败: ' + data.error);
        });
    };

    // ── Agent Selection (uses discovered agents) ──

    function renderAgentList() {
        var container = document.getElementById('agent-list');
        container.innerHTML = discoveredAgents.map(function(agent, i) {
            var fieldCount = agent.values ? Object.keys(agent.values).length : 0;
            return '<div class="agent-item" onclick="toggleAgent(this,\''+agent.name+'\')">'
                +'<input type="checkbox" data-agent="'+agent.name+'" checked>'
                +'<span class="agent-name">'+agent.name+'</span>'
                +'<span class="agent-desc">'+agent.config_path+' ('+fieldCount+' 字段)</span>'
                +'</div>';
        }).join('');
        selectedAgents = discoveredAgents.map(function(a){return a.name});
    }

    window.toggleAgent = function(el, name) {
        var cb = el.querySelector('input');
        cb.checked = !cb.checked;
        el.classList.toggle('selected', cb.checked);
        if (cb.checked) {
            if (selectedAgents.indexOf(name) === -1) selectedAgents.push(name);
        } else {
            selectedAgents = selectedAgents.filter(function(n){return n!==name});
        }
    };

    window.selectAll = function() {
        selectedAgents = discoveredAgents.map(function(a){return a.name});
        document.querySelectorAll('.agent-item input').forEach(function(cb){
            cb.checked = true;
            cb.parentElement.classList.add('selected');
        });
    };

    window.selectNone = function() {
        selectedAgents = [];
        document.querySelectorAll('.agent-item input').forEach(function(cb){
            cb.checked = false;
            cb.parentElement.classList.remove('selected');
        });
    };

    window.confirmSelection = function() {
        if (selectedAgents.length === 0) {
            alert('请至少选择一个 Agent');
            return;
        }
        currentAgentIdx = 0;
        skippedAgents = [];
        // Find the step-config index
        var configIdx = STEP_IDS.indexOf('step-config');
        showStep(configIdx);
        renderSingleAgentForm(currentAgentIdx);
    };

    // ── Per-agent configuration ──

    function renderSingleAgentForm(idx) {
        var name = selectedAgents[idx];
        var agent = discoveredAgents.find(function(a){return a.name===name});
        if (!agent) return;

        var total = selectedAgents.length;
        document.getElementById('agent-progress').textContent =
            'Agent 配置 (' + (idx+1) + '/' + total + ') — ' + name;

        var shared = agentValues['__shared__'] || {};
        var values = agent.values || {};
        var keys = Object.keys(values).sort();

        var sharedKeyMap = {
            'server_url': 'server_url',
            'gateway_url': 'server_url',
            'gateway_http': 'gateway_http',
            'auth_token': 'auth_token'
        };

        var fieldsHTML = keys.map(function(key) {
            var val = values[key];
            var fieldType = inferFieldType(val);
            var displayVal = formatValueForDisplay(val);

            if (sharedKeyMap[key] && shared[sharedKeyMap[key]]) {
                displayVal = shared[sharedKeyMap[key]];
            } else if (shared[key]) {
                displayVal = shared[key];
            }

            var typeLabel = '<span style="color:#8b949e;font-size:0.85em"> (' + fieldType + ')</span>';

            if (fieldType === 'object') {
                return '<div class="form-group">'
                    +'<label>' + key + typeLabel + '</label>'
                    +'<textarea name="'+name+'__'+key+'" rows="6" style="font-family:monospace;font-size:0.9em">'
                    +escapeAttr(displayVal)+'</textarea>'
                    +'<small>JSON 对象，可直接编辑</small></div>';
            }

            return '<div class="form-group">'
                +'<label>' + key + typeLabel + '</label>'
                +'<input type="text" name="'+name+'__'+key+'" value="'+escapeAttr(displayVal)+'">'
                +'</div>';
        }).join('');

        document.getElementById('agent-forms').innerHTML =
            '<div class="agent-form"><div class="agent-form-body">' + fieldsHTML + '</div></div>';

        var isLast = (idx >= total - 1);
        document.getElementById('btn-save-agent').textContent = isLast ? '完成配置 →' : '保存并继续 →';
    }

    function inferFieldType(val) {
        if (val === null || val === undefined) return 'string';
        if (typeof val === 'number') return 'number';
        if (typeof val === 'boolean') return 'bool';
        if (Array.isArray(val)) return 'array';
        if (typeof val === 'object') return 'object';
        return 'string';
    }

    function formatValueForDisplay(val) {
        if (val === null || val === undefined) return '';
        if (typeof val === 'object') return JSON.stringify(val, null, 2);
        if (Array.isArray(val)) return val.join(',');
        return String(val);
    }

    function collectCurrentAgentValues() {
        var name = selectedAgents[currentAgentIdx];
        var agent = discoveredAgents.find(function(a){return a.name===name});
        if (!agent) return {};

        var result = {};
        var keys = Object.keys(agent.values || {});

        keys.forEach(function(key) {
            var input = document.querySelector('[name="'+name+'__'+key+'"]');
            if (input) {
                var origVal = agent.values[key];
                var inputVal = input.value;
                result[key] = parseInputValue(inputVal, origVal);
            }
        });

        return result;
    }

    function parseInputValue(input, origVal) {
        if (origVal === null || origVal === undefined) return input;
        if (typeof origVal === 'number') {
            var n = Number(input);
            return isNaN(n) ? input : n;
        }
        if (typeof origVal === 'boolean') {
            return input === 'true' || input === '1' || input === 'yes';
        }
        if (Array.isArray(origVal)) {
            return input.split(',').map(function(s){return s.trim()}).filter(function(s){return s});
        }
        if (typeof origVal === 'object') {
            try { return JSON.parse(input); } catch(e) { return input; }
        }
        return input;
    }

    window.saveCurrentAgent = function() {
        var name = selectedAgents[currentAgentIdx];
        agentValues[name] = collectCurrentAgentValues();

        currentAgentIdx++;
        while (currentAgentIdx < selectedAgents.length && skippedAgents.indexOf(selectedAgents[currentAgentIdx]) !== -1) {
            currentAgentIdx++;
        }

        if (currentAgentIdx >= selectedAgents.length) {
            prepareConfigGeneration();
            var genIdx = STEP_IDS.indexOf('step-generate');
            showStep(genIdx);
        } else {
            renderSingleAgentForm(currentAgentIdx);
        }
    };

    window.skipAgent = function() {
        var name = selectedAgents[currentAgentIdx];
        skippedAgents.push(name);

        currentAgentIdx++;
        while (currentAgentIdx < selectedAgents.length && skippedAgents.indexOf(selectedAgents[currentAgentIdx]) !== -1) {
            currentAgentIdx++;
        }

        if (currentAgentIdx >= selectedAgents.length) {
            prepareConfigGeneration();
            var genIdx = STEP_IDS.indexOf('step-generate');
            showStep(genIdx);
        } else {
            renderSingleAgentForm(currentAgentIdx);
        }
    };

    // ── Config Generation ──

    function prepareConfigGeneration() {
        var container = document.getElementById('generate-preview');
        container.innerHTML = '';

        // Deploy config preview
        if (deployAvailable) {
            var deployFiles = [];
            if (Object.keys(deployTargets).length > 0) deployFiles.push('settings/targets.json');
            deployProjectOrder.forEach(function(name) { deployFiles.push('settings/projects/' + name + '.json'); });
            deployPipelines.forEach(function(p) { deployFiles.push('settings/pipelines/' + p.name + '.json'); });
            if (deploySSHPassword) deployFiles.push('deploy-agent.json (ssh_password)');

            if (deployFiles.length > 0) {
                container.innerHTML += '<div class="config-preview">'
                    +'<div class="file-path" style="color:#58a6ff">Deploy 配置文件</div>'
                    +'<div style="color:#8b949e">' + deployFiles.length + ' 个文件</div>'
                    +'<pre style="background:#161b22;padding:8px;border-radius:4px;font-size:0.85em;max-height:150px;overflow:auto">'
                    +deployFiles.map(function(f){return '  · ' + f}).join('\n')
                    +'</pre></div>';
            }
        }

        var configuredAgents = selectedAgents.filter(function(name) {
            return skippedAgents.indexOf(name) === -1 && agentValues[name];
        });

        configuredAgents.forEach(function(name) {
            var agent = discoveredAgents.find(function(a){return a.name===name});
            if (!agent) return;
            var vals = agentValues[name];
            var fieldCount = Object.keys(vals).length;
            container.innerHTML += '<div class="config-preview">'
                +'<div class="file-path">' + agent.config_path + '</div>'
                +'<div style="color:#8b949e">已配置 ' + fieldCount + ' 个字段</div>'
                +'<pre style="background:#161b22;padding:8px;border-radius:4px;font-size:0.85em;max-height:200px;overflow:auto">'
                +escapeAttr(JSON.stringify(vals, null, 2))
                +'</pre></div>';
        });

        if (skippedAgents.length > 0) {
            container.innerHTML += '<div style="color:#d29922;margin-top:12px">已跳过: '
                +skippedAgents.join(', ')+'</div>';
        }
    }

    window.writeConfigs = function() {
        var shared = agentValues['__shared__'] || {};
        var agents = {};
        selectedAgents.forEach(function(name) {
            if (skippedAgents.indexOf(name) === -1 && agentValues[name]) {
                agents[name] = agentValues[name];
            }
        });

        document.getElementById('btn-write').disabled = true;
        fetch('/api/agents/configs', {
            method: 'POST',
            headers: {'Content-Type':'application/json'},
            body: JSON.stringify({agents: agents, shared: shared})
        }).then(function(r){return r.json()}).then(function(data) {
            var results = document.getElementById('write-results');
            if (data.success) {
                results.innerHTML = '<div class="result-item"><span class="icon status-green">✓</span>'
                    +'<span class="detail">已写入 '+(data.written||[]).length+' 个配置文件</span></div>';
                (data.written||[]).forEach(function(p) {
                    results.innerHTML += '<div class="result-item"><span class="icon status-green">·</span>'
                        +'<span class="detail">'+p+'</span></div>';
                });
            } else {
                results.innerHTML = '<div class="result-item"><span class="icon status-red">✗</span>'
                    +'<span class="detail">'+data.error+'</span></div>';
            }
            document.getElementById('btn-gen-next').classList.remove('hidden');
        });
    };

    function appendWriteResult(data) {
        var results = document.getElementById('write-results');
        results.innerHTML += '<div class="result-item"><span class="icon status-green">✓</span>'
            +'<span class="name">'+data.agent+'</span>'
            +'<span class="detail">'+data.path+'</span></div>';
    }

    // ── Availability check ──

    window.runAvailCheck = function() {
        document.getElementById('avail-results').innerHTML = '<span class="spinner"></span> 正在检测...';
        document.getElementById('btn-avail-check').disabled = true;
        fetch('/api/availability/check', {method:'POST'});
    };

    function appendAvailLayer(layer) {
        var container = document.getElementById('avail-results');
        if (container.querySelector('.spinner')) container.innerHTML = '';

        var itemsHTML = (layer.items||[]).map(function(item) {
            var cls = 'status-'+item.status;
            var icon = item.status==='green'?'✓':(item.status==='yellow'?'!':'✗');
            return '<div class="result-item"><span class="icon '+cls+'">'+icon+'</span>'
                +'<span class="name">'+item.name+'</span>'
                +'<span class="detail">'+item.detail+'</span></div>';
        }).join('');

        container.innerHTML += '<div class="avail-layer">'
            +'<div class="avail-layer-header"><div class="dot '+layer.status+'"></div><strong>'+layer.label+'</strong></div>'
            +'<div class="avail-layer-body">'+itemsHTML+'</div></div>';
    }

    // Utilities
    function escapeAttr(s) {
        if (!s) return '';
        return String(s).replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    }
})();
