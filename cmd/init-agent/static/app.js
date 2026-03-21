// Init Agent - Web Wizard Frontend
(function() {
    'use strict';

    const STEP_IDS = ['step-welcome','step-env','step-global','step-select','step-config','step-generate','step-avail'];
    const STEP_NAMES = ['欢迎','环境检测','全局配置','Agent 选择','Agent 配置','配置生成','可用性面板'];
    let currentStep = 0;
    let schemas = [];
    let existingConfigs = {};
    let selectedAgents = [];
    let agentValues = {};
    let ws = null;

    // Initialize
    document.addEventListener('DOMContentLoaded', function() {
        renderStepDots();
        connectWS();
        loadSchemas();
        loadExistingConfigs();
        showStep(0);
    });

    // WebSocket connection
    function connectWS() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(proto + '//' + location.host + '/ws');
        ws.onmessage = function(e) {
            const msg = JSON.parse(e.data);
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
        const container = document.getElementById('steps');
        container.innerHTML = STEP_NAMES.map(function(name, i) {
            return '<div class="step-dot" id="dot-'+i+'" title="'+name+'">'+(i+1)+'</div>';
        }).join('');
    }

    function showStep(idx) {
        STEP_IDS.forEach(function(id, i) {
            var el = document.getElementById(id);
            if (i === idx) el.classList.remove('hidden');
            else el.classList.add('hidden');
        });
        // Update dots
        for (var i = 0; i < STEP_NAMES.length; i++) {
            var dot = document.getElementById('dot-'+i);
            dot.className = 'step-dot';
            if (i < idx) dot.classList.add('done');
            else if (i === idx) dot.classList.add('active');
        }
        currentStep = idx;
    }

    window.nextStep = function() {
        if (currentStep < STEP_IDS.length - 1) {
            showStep(currentStep + 1);
            if (currentStep === 3) renderAgentList();
        }
    };

    // Load data
    function loadSchemas() {
        fetch('/api/agents/schemas').then(function(r){return r.json()}).then(function(data) {
            if (data.success) schemas = data.schemas;
        });
    }

    function loadExistingConfigs() {
        fetch('/api/agents/configs').then(function(r){return r.json()}).then(function(data) {
            if (data.success) existingConfigs = data.configs || {};
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

    // Step 4: Agent selection
    function renderAgentList() {
        var container = document.getElementById('agent-list');
        container.innerHTML = schemas.map(function(s, i) {
            var hasConfig = existingConfigs[s.name] ? '<span class="badge">已有配置</span>' : '';
            return '<div class="agent-item" onclick="toggleAgent(this,\''+s.name+'\')">'
                +'<input type="checkbox" data-agent="'+s.name+'" checked>'
                +'<span class="agent-name">'+s.name+'</span>'
                +'<span class="agent-desc">'+s.description+'</span>'
                +hasConfig+'</div>';
        }).join('');
        // Default: all selected
        selectedAgents = schemas.map(function(s){return s.name});
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
        selectedAgents = schemas.map(function(s){return s.name});
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
        renderAgentForms();
        nextStep();
    };

    // Step 5: Agent config forms
    function renderAgentForms() {
        var container = document.getElementById('agent-forms');
        var shared = agentValues['__shared__'] || {};

        container.innerHTML = selectedAgents.map(function(name) {
            var schema = schemas.find(function(s){return s.name===name});
            if (!schema) return '';
            var existing = existingConfigs[name] || {};
            var fields = (schema.fields || []).filter(function(f){return !f.shared});

            var fieldsHTML = fields.map(function(f) {
                if (f.type === 7) return ''; // Skip Map type
                var val = existing[f.key] || (f.default_value != null ? String(f.default_value) : '');
                var reqClass = f.required ? ' required' : '';
                return '<div class="form-group"><label class="'+reqClass+'">'+f.label+'</label>'
                    +'<input type="text" name="'+name+'__'+f.key+'" value="'+escapeAttr(val)+'">'
                    +'<small>'+f.description+'</small></div>';
            }).join('');

            return '<div class="agent-form">'
                +'<div class="agent-form-header" onclick="toggleForm(this)"><h3>'+name+'</h3><span>'+schema.description+'</span></div>'
                +'<div class="agent-form-body">'+fieldsHTML+'</div></div>';
        }).join('');
    }

    window.toggleForm = function(header) {
        var body = header.nextElementSibling;
        body.classList.toggle('hidden');
    };

    // Step 6: Save and generate configs
    window.saveConfigs = function() {
        var container = document.getElementById('generate-preview');
        container.innerHTML = '';

        selectedAgents.forEach(function(name) {
            var schema = schemas.find(function(s){return s.name===name});
            if (!schema) return;
            var vals = {};
            var inputs = document.querySelectorAll('input[name^="'+name+'__"]');
            inputs.forEach(function(input) {
                var key = input.name.replace(name+'__','');
                if (input.value) vals[key] = input.value;
            });
            agentValues[name] = vals;

            var path = schema.dir + '/' + schema.config_file_name;
            container.innerHTML += '<div class="config-preview"><div class="file-path">'+path+'</div>'
                +'Fields: '+Object.keys(vals).length+' configured</div>';
        });

        nextStep();
    };

    window.writeConfigs = function() {
        var shared = agentValues['__shared__'] || {};
        var agents = {};
        selectedAgents.forEach(function(name) {
            agents[name] = agentValues[name] || {};
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

    // Step 7: Availability check
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
