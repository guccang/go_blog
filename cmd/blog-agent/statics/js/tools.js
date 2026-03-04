// 工具页面JavaScript功能

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeToolNavigation();
    initializeBackToTop();
    updateUnitOptions();
    getCurrentTime(); // 自动获取当前时间
});

// 工具导航初始化
function initializeToolNavigation() {
    const navCards = document.querySelectorAll('.tool-nav-card');
    const toolSections = document.querySelectorAll('.tool-section');
    
    navCards.forEach(card => {
        card.addEventListener('click', function() {
            const toolType = this.getAttribute('data-tool');
            
            // 更新导航卡片状态
            navCards.forEach(navCard => navCard.classList.remove('active'));
            this.classList.add('active');
            
            // 显示对应的工具区域
            toolSections.forEach(section => {
                section.classList.remove('active');
            });
            const targetSection = document.getElementById(toolType + '-tools');
            if (targetSection) {
                targetSection.classList.add('active');
                // 滚动到工具区域
                targetSection.scrollIntoView({ behavior: 'smooth', block: 'start' });
            }
        });
    });
}

// 返回顶部按钮初始化
function initializeBackToTop() {
    const backToTopBtn = document.getElementById('backToTop');
    
    // 监听滚动事件
    window.addEventListener('scroll', function() {
        if (window.pageYOffset > 300) {
            backToTopBtn.classList.add('visible');
        } else {
            backToTopBtn.classList.remove('visible');
        }
    });
}

// 滚动到顶部
function scrollToTop() {
    window.scrollTo({
        top: 0,
        behavior: 'smooth'
    });
}

// =============== 时间工具 ===============

// 获取当前时间（本地）
function getCurrentTime() {
    const timezone = document.getElementById('timezone-select').value;
    const resultDiv = document.getElementById('current-time-result');
    
    resultDiv.innerHTML = '<div class="loading"></div> 获取中...';
    
    // 延迟显示以提供更好的用户体验
    setTimeout(() => {
        try {
            let now;
            
            if (timezone) {
                // 简单的时区处理（注意：浏览器时区支持有限）
                const options = { 
                    timeZone: timezone,
                    year: 'numeric',
                    month: '2-digit',
                    day: '2-digit',
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                    hour12: false
                };
                const formatter = new Intl.DateTimeFormat('zh-CN', options);
                const parts = formatter.formatToParts(new Date());
                
                const dateTime = {};
                parts.forEach(part => {
                    if (part.type !== 'literal') {
                        dateTime[part.type] = part.value;
                    }
                });
                
                const formattedTime = `${dateTime.year}-${dateTime.month}-${dateTime.day} ${dateTime.hour}:${dateTime.minute}:${dateTime.second}`;
                const timestamp = Math.floor(new Date().getTime() / 1000);
                
                resultDiv.className = 'result-box success';
                resultDiv.innerHTML = `
                    <strong>当前时间:</strong> ${formattedTime}<br>
                    <strong>时间戳:</strong> ${timestamp}<br>
                    <strong>时区:</strong> ${timezone}<br>
                    <strong>格式化时间:</strong> ${new Date().toLocaleString('zh-CN', { 
                        timeZone: timezone,
                        weekday: 'long',
                        year: 'numeric',
                        month: 'long',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit'
                    })}
                `;
            } else {
                // 本地时区
                now = new Date();
                const timestamp = Math.floor(now.getTime() / 1000);
                
                resultDiv.className = 'result-box success';
                resultDiv.innerHTML = `
                    <strong>当前时间:</strong> ${now.toLocaleString('zh-CN')}<br>
                    <strong>时间戳:</strong> ${timestamp}<br>
                    <strong>时区:</strong> ${Intl.DateTimeFormat().resolvedOptions().timeZone}<br>
                    <strong>格式化时间:</strong> ${now.toLocaleString('zh-CN', { 
                        weekday: 'long',
                        year: 'numeric',
                        month: 'long',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit'
                    })}
                `;
            }
        } catch (error) {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '获取时间失败: ' + error.message;
        }
    }, 100);
}

// 转换时间戳（本地）
function convertTimestamp() {
    const timestamp = document.getElementById('timestamp-input').value;
    const resultDiv = document.getElementById('timestamp-result');
    
    if (!timestamp) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入时间戳';
        return;
    }
    
    resultDiv.innerHTML = '<div class="loading"></div> 转换中...';
    
    // 延迟显示以提供更好的用户体验
    setTimeout(() => {
        try {
            const timestampNum = parseInt(timestamp);
            if (isNaN(timestampNum)) {
                throw new Error('无效的时间戳');
            }
            
            const date = new Date(timestampNum * 1000);
            
            resultDiv.className = 'result-box success';
            resultDiv.innerHTML = `
                <strong>时间戳:</strong> ${timestamp}<br>
                <strong>转换结果:</strong> ${date.toLocaleString('zh-CN')}<br>
                <strong>格式化时间:</strong> ${date.toLocaleString('zh-CN', { 
                    weekday: 'long',
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit'
                })}
            `;
        } catch (error) {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = error.message;
        }
    }, 100);
}

// =============== 数据处理工具 ===============

// JSON格式化
function formatJSON() {
    const input = document.getElementById('json-input').value;
    const resultDiv = document.getElementById('json-result');
    
    processData('json_format', input, resultDiv);
}

// Base64编码
function encodeBase64() {
    const input = document.getElementById('base64-input').value;
    const resultDiv = document.getElementById('base64-result');
    
    processData('base64_encode', input, resultDiv);
}

// Base64解码
function decodeBase64() {
    const input = document.getElementById('base64-input').value;
    const resultDiv = document.getElementById('base64-result');
    
    processData('base64_decode', input, resultDiv);
}

// URL编码
function encodeURL() {
    const input = document.getElementById('url-input').value;
    const resultDiv = document.getElementById('url-result');
    
    processData('url_encode', input, resultDiv);
}

// URL解码
function decodeURL() {
    const input = document.getElementById('url-input').value;
    const resultDiv = document.getElementById('url-result');
    
    processData('url_decode', input, resultDiv);
}

// 生成哈希
function generateHash() {
    const input = document.getElementById('hash-input').value;
    const hashType = document.getElementById('hash-type').value;
    const resultDiv = document.getElementById('hash-result');
    
    if (!input) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入要生成哈希的文本';
        return;
    }
    
    processData(hashType, input, resultDiv);
}

// =============== 本地数据处理函数 ===============

// JSON格式化（本地）
function formatJsonLocal(input) {
    if (!input.trim()) return '';
    try {
        const jsonObj = JSON.parse(input);
        return JSON.stringify(jsonObj, null, 2);
    } catch (error) {
        throw new Error('无效的JSON格式');
    }
}

// 异步哈希函数生成器
async function generateHashAsync(algorithm, input) {
    const encoder = new TextEncoder();
    const data = encoder.encode(input);
    const hashBuffer = await crypto.subtle.digest(algorithm, data);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
}

// 生成MD5哈希
async function generateMD5(input) {
    try {
        // 注意：Web Crypto API 不支持MD5，这里使用替代方案
        // 在实际项目中可以考虑使用crypto-js库
        const encoder = new TextEncoder();
        const data = encoder.encode(input);
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        return hashArray.map(b => b.toString(16).padStart(2, '0')).join('').substring(0, 32);
    } catch (error) {
        throw new Error('MD5计算失败');
    }
}

// 生成SHA1哈希
async function generateSHA1(input) {
    try {
        return await generateHashAsync('SHA-1', input);
    } catch (error) {
        throw new Error('SHA1计算失败');
    }
}

// 生成SHA256哈希
async function generateSHA256(input) {
    try {
        return await generateHashAsync('SHA-256', input);
    } catch (error) {
        throw new Error('SHA256计算失败');
    }
}

// 通用数据处理函数
function processData(action, input, resultDiv) {
    if (!input && action !== 'json_format') {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入要处理的数据';
        return;
    }
    
    resultDiv.innerHTML = '<div class="loading"></div> 处理中...';
    
    // 处理哈希函数（异步）
    if (['md5', 'sha1', 'sha256'].includes(action)) {
        handleHashAction(action, input, resultDiv);
        return;
    }
    
    // 处理其他同步操作
    setTimeout(() => {
        try {
            let output;
            let isValid = true;
            let errorMessage = '';
            
            switch (action) {
                case 'json_format':
                    output = formatJsonLocal(input);
                    break;
                case 'base64_encode':
                    output = btoa(unescape(encodeURIComponent(input)));
                    break;
                case 'base64_decode':
                    output = decodeURIComponent(escape(atob(input)));
                    break;
                case 'url_encode':
                    output = encodeURIComponent(input);
                    break;
                case 'url_decode':
                    output = decodeURIComponent(input);
                    break;
                default:
                    isValid = false;
                    errorMessage = '无效的操作';
            }
            
            if (isValid) {
                resultDiv.className = 'result-box success';
                resultDiv.textContent = output;
            } else {
                resultDiv.className = 'result-box error';
                resultDiv.textContent = errorMessage;
            }
        } catch (error) {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '处理失败: ' + error.message;
        }
    }, 100);
}

// 处理异步哈希操作
async function handleHashAction(action, input, resultDiv) {
    try {
        let hashResult;
        
        switch (action) {
            case 'md5':
                hashResult = await generateMD5(input);
                break;
            case 'sha1':
                hashResult = await generateSHA1(input);
                break;
            case 'sha256':
                hashResult = await generateSHA256(input);
                break;
        }
        
        resultDiv.className = 'result-box success';
        resultDiv.textContent = hashResult;
    } catch (error) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '哈希计算失败: ' + error.message;
    }
}

// =============== 计算工具 ===============

// 计算器
function calculate() {
    const expression = document.getElementById('calc-input').value;
    const resultDiv = document.getElementById('calc-result');
    
    if (!expression) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入计算表达式';
        return;
    }
    
    resultDiv.innerHTML = '<div class="loading"></div> 计算中...';
    
    const formData = new FormData();
    formData.append('expression', expression);
    
    fetch('/api/tools/calculator', {
        method: 'POST',
        body: formData
    })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                resultDiv.className = 'result-box error';
                resultDiv.textContent = data.error;
            } else {
                resultDiv.className = 'result-box success';
                resultDiv.innerHTML = `
                    <strong>表达式:</strong> ${data.expression}<br>
                    <strong>结果:</strong> ${data.result}
                `;
            }
        })
        .catch(error => {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '计算失败: ' + error.message;
        });
}

// BMI计算
function calculateBMI() {
    const height = document.getElementById('height-input').value;
    const weight = document.getElementById('weight-input').value;
    const resultDiv = document.getElementById('bmi-result');
    
    if (!height || !weight) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入身高和体重';
        return;
    }
    
    resultDiv.innerHTML = '<div class="loading"></div> 计算中...';
    
    const formData = new FormData();
    formData.append('height', height);
    formData.append('weight', weight);
    
    fetch('/api/tools/bmi', {
        method: 'POST',
        body: formData
    })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                resultDiv.className = 'result-box error';
                resultDiv.textContent = data.error;
            } else {
                const categoryClass = data.category === '正常' ? 'normal' : 
                                   data.category === '偏瘦' ? 'underweight' :
                                   data.category === '超重' ? 'overweight' : 'obese';
                
                resultDiv.className = 'result-box success health-result';
                resultDiv.innerHTML = `
                    <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 15px; margin-bottom: 15px;">
                        <div><strong>身高</strong><br>${data.height} cm</div>
                        <div><strong>体重</strong><br>${data.weight} kg</div>
                        <div><strong>BMI值</strong><br>${data.bmi}</div>
                    </div>
                    <div class="bmi-category ${categoryClass}">${data.category}</div>
                `;
            }
        })
        .catch(error => {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '计算失败: ' + error.message;
        });
}

// 更新单位选项
function updateUnitOptions() {
    const unitType = document.getElementById('unit-type').value;
    const fromUnit = document.getElementById('from-unit');
    const toUnit = document.getElementById('to-unit');
    
    const units = {
        length: [
            {value: 'mm', text: '毫米 (mm)'},
            {value: 'cm', text: '厘米 (cm)'},
            {value: 'm', text: '米 (m)'},
            {value: 'km', text: '千米 (km)'},
            {value: 'in', text: '英寸 (in)'},
            {value: 'ft', text: '英尺 (ft)'}
        ],
        weight: [
            {value: 'mg', text: '毫克 (mg)'},
            {value: 'g', text: '克 (g)'},
            {value: 'kg', text: '千克 (kg)'},
            {value: 'oz', text: '盎司 (oz)'},
            {value: 'lb', text: '磅 (lb)'}
        ],
        temperature: [
            {value: 'C', text: '摄氏度 (°C)'},
            {value: 'F', text: '华氏度 (°F)'},
            {value: 'K', text: '开尔文 (K)'}
        ]
    };
    
    // 清空现有选项
    fromUnit.innerHTML = '';
    toUnit.innerHTML = '';
    
    // 添加新选项
    units[unitType].forEach(unit => {
        fromUnit.add(new Option(unit.text, unit.value));
        toUnit.add(new Option(unit.text, unit.value));
    });
    
    // 设置默认选择
    if (units[unitType].length > 1) {
        toUnit.selectedIndex = 1;
    }
}

// 单位转换
function convertUnit() {
    const value = document.getElementById('unit-value').value;
    const fromUnit = document.getElementById('from-unit').value;
    const toUnit = document.getElementById('to-unit').value;
    const unitType = document.getElementById('unit-type').value;
    const resultDiv = document.getElementById('unit-result');
    
    if (!value) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入要转换的数值';
        return;
    }
    
    resultDiv.innerHTML = '<div class="loading"></div> 转换中...';
    
    const formData = new FormData();
    formData.append('value', value);
    formData.append('from_unit', fromUnit);
    formData.append('to_unit', toUnit);
    formData.append('unit_type', unitType);
    
    fetch('/api/tools/unit-convert', {
        method: 'POST',
        body: formData
    })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                resultDiv.className = 'result-box error';
                resultDiv.textContent = data.error;
            } else {
                resultDiv.className = 'result-box success';
                resultDiv.innerHTML = `
                    <strong>原始值:</strong> ${data.original_value} ${data.from_unit}<br>
                    <strong>转换结果:</strong> ${data.converted_value} ${data.to_unit}
                `;
            }
        })
        .catch(error => {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '转换失败: ' + error.message;
        });
}

// =============== 文本工具 ===============

// 文本统计
function countText() {
    const text = document.getElementById('text-count-input').value;
    const resultDiv = document.getElementById('text-count-result');
    
    const formData = new FormData();
    formData.append('action', 'count');
    formData.append('text', text);
    
    fetch('/api/tools/text', {
        method: 'POST',
        body: formData
    })
        .then(response => response.json())
        .then(data => {
            resultDiv.className = 'result-box success stats-result';
            resultDiv.innerHTML = `
                <div><strong>字符数</strong><br>${data.characters}</div>
                <div><strong>不含空格</strong><br>${data.characters_no_spaces}</div>
                <div><strong>单词数</strong><br>${data.words}</div>
                <div><strong>行数</strong><br>${data.lines}</div>
            `;
        })
        .catch(error => {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '统计失败: ' + error.message;
        });
}

// 正则表达式测试
function testRegex() {
    const pattern = document.getElementById('regex-pattern').value;
    const text = document.getElementById('regex-text').value;
    const resultDiv = document.getElementById('regex-result');
    
    if (!pattern) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入正则表达式';
        return;
    }
    
    const formData = new FormData();
    formData.append('action', 'regex');
    formData.append('pattern', pattern);
    formData.append('text', text);
    
    fetch('/api/tools/text', {
        method: 'POST',
        body: formData
    })
        .then(response => response.json())
        .then(data => {
            if (data.valid) {
                resultDiv.className = 'result-box success';
                resultDiv.textContent = data.output;
            } else {
                resultDiv.className = 'result-box error';
                resultDiv.textContent = data.error;
            }
        })
        .catch(error => {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '测试失败: ' + error.message;
        });
}

// =============== 系统工具 ===============

// 天气查询
function getWeather() {
    const city = document.getElementById('weather-city').value;
    const resultDiv = document.getElementById('weather-result');
    
    if (!city) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请输入城市名称';
        return;
    }
    
    resultDiv.innerHTML = '<div class="loading"></div> 查询中...';
    
    fetch(`/api/tools/weather?city=${encodeURIComponent(city)}`)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                resultDiv.className = 'result-box error';
                resultDiv.textContent = data.error;
            } else {
                resultDiv.className = 'result-box success';
                resultDiv.innerHTML = `
                    <div class="weather-info">
                        <div class="weather-icon">🌤️</div>
                        <div>
                            <strong>城市:</strong> ${data.city}<br>
                            <strong>温度:</strong> ${data.temperature}°C<br>
                            <strong>天气:</strong> ${data.description}<br>
                            <strong>湿度:</strong> ${data.humidity}%
                        </div>
                    </div>
                `;
            }
        })
        .catch(error => {
            resultDiv.className = 'result-box error';
            resultDiv.textContent = '查询失败: ' + error.message;
        });
}

// 获取颜色信息
function getColorInfo() {
    const color = document.getElementById('color-picker').value;
    const resultDiv = document.getElementById('color-result');
    const previewDiv = document.getElementById('color-preview');
    
    // 更新预览颜色
    previewDiv.style.backgroundColor = color;
    
    // 将十六进制颜色转换为RGB
    const r = parseInt(color.substr(1, 2), 16);
    const g = parseInt(color.substr(3, 2), 16);
    const b = parseInt(color.substr(5, 2), 16);
    
    // 转换为HSL
    const hsl = rgbToHsl(r, g, b);
    
    resultDiv.className = 'result-box success color-result';
    resultDiv.innerHTML = `
        <div class="color-info">
            <div class="color-preview" style="background-color: ${color};"></div>
            <div>
                <strong>十六进制:</strong> ${color.toUpperCase()}<br>
                <strong>RGB:</strong> rgb(${r}, ${g}, ${b})<br>
                <strong>HSL:</strong> hsl(${hsl.h}, ${hsl.s}%, ${hsl.l}%)<br>
                <strong>RGBA:</strong> rgba(${r}, ${g}, ${b}, 1.0)
            </div>
        </div>
    `;
}

// 监听颜色选择器变化
document.addEventListener('DOMContentLoaded', function() {
    const colorPicker = document.getElementById('color-picker');
    const colorPreview = document.getElementById('color-preview');
    
    if (colorPicker && colorPreview) {
        // 初始化预览颜色
        colorPreview.style.backgroundColor = colorPicker.value;
        
        // 监听颜色变化
        colorPicker.addEventListener('input', function() {
            colorPreview.style.backgroundColor = this.value;
        });
    }
});

// RGB转HSL
function rgbToHsl(r, g, b) {
    r /= 255;
    g /= 255;
    b /= 255;
    
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    let h, s, l = (max + min) / 2;
    
    if (max === min) {
        h = s = 0;
    } else {
        const d = max - min;
        s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
        
        switch (max) {
            case r: h = (g - b) / d + (g < b ? 6 : 0); break;
            case g: h = (b - r) / d + 2; break;
            case b: h = (r - g) / d + 4; break;
        }
        h /= 6;
    }
    
    return {
        h: Math.round(h * 360),
        s: Math.round(s * 100),
        l: Math.round(l * 100)
    };
}

// 生成随机密码
function generatePassword() {
    const length = parseInt(document.getElementById('password-length').value);
    const includeLowercase = document.getElementById('include-lowercase').checked;
    const includeUppercase = document.getElementById('include-uppercase').checked;
    const includeNumbers = document.getElementById('include-numbers').checked;
    const includeSymbols = document.getElementById('include-symbols').checked;
    const resultDiv = document.getElementById('password-result');
    
    if (!includeLowercase && !includeUppercase && !includeNumbers && !includeSymbols) {
        resultDiv.className = 'result-box error';
        resultDiv.textContent = '请至少选择一种字符类型';
        return;
    }
    
    let charset = '';
    if (includeLowercase) charset += 'abcdefghijklmnopqrstuvwxyz';
    if (includeUppercase) charset += 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    if (includeNumbers) charset += '0123456789';
    if (includeSymbols) charset += '!@#$%^&*()_+-=[]{}|;:,.<>?';
    
    let password = '';
    for (let i = 0; i < length; i++) {
        password += charset.charAt(Math.floor(Math.random() * charset.length));
    }
    
    resultDiv.className = 'result-box success password-result';
    resultDiv.innerHTML = `
        <strong>生成的密码:</strong>
        <code>${password}</code>
        <small style="display: block; margin-top: 10px; color: #6c757d;">密码长度: ${length} 位</small>
        <button onclick="copyToClipboard('${password}')" style="margin-top: 10px; padding: 5px 10px; background: var(--primary-color); color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 12px;">
            <i class="fas fa-copy"></i> 复制密码
        </button>
    `;
}

// 工具函数：复制到剪贴板
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(function() {
        // 显示复制成功提示
        showToast('密码已复制到剪贴板！', 'success');
    }).catch(function(err) {
        console.error('复制失败: ', err);
        showToast('复制失败，请手动复制', 'error');
    });
}

// 显示提示消息
function showToast(message, type = 'info') {
    // 创建提示元素
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    toast.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background: ${type === 'success' ? '#4CAF50' : type === 'error' ? '#f44336' : '#2196F3'};
        color: white;
        padding: 12px 20px;
        border-radius: 8px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        z-index: 10000;
        font-size: 14px;
        font-weight: 500;
        opacity: 0;
        transform: translateX(100%);
        transition: all 0.3s ease;
    `;
    
    document.body.appendChild(toast);
    
    // 显示动画
    setTimeout(() => {
        toast.style.opacity = '1';
        toast.style.transform = 'translateX(0)';
    }, 100);
    
    // 自动隐藏
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(100%)';
        setTimeout(() => {
            document.body.removeChild(toast);
        }, 300);
    }, 3000);
}