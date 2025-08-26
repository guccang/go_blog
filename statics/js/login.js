    document.addEventListener('DOMContentLoaded', function() {
        // Create background animation bubbles
        const bgAnimation = document.getElementById('background-animation');
        const bubbleCount = 20;
        
        for(let i = 0; i < bubbleCount; i++) {
            const bubble = document.createElement('span');
            const size = Math.random() * 20 + 5;
            
            bubble.style.width = size + 'px';
            bubble.style.height = size + 'px';
            bubble.style.left = Math.random() * 100 + '%';
            bubble.style.bottom = Math.random() * 50 + '%';
            bubble.style.animationDelay = Math.random() * 2 + 's';
            bubble.style.animationDuration = Math.random() * 10 + 8 + 's';
            
            bgAnimation.appendChild(bubble);
        }
        
        // Focus the account input
        document.getElementById('account').focus();
        
        // Add enter key support
        document.addEventListener('keydown', function(event) {
            if (event.key === "Enter") {
                const currentTab = getCurrentTab();
                if (currentTab === 'password') {
                    submitContent();
                } else if (currentTab === 'sms') {
                    submitSMS();
                } else if (currentTab === 'register') {
                    submitRegister();
                }
            }
        });
    });
    
    // MD5 implementation for device_id generation
    function md5(str) {
        function md5cycle(x, k) {
            var a = x[0], b = x[1], c = x[2], d = x[3];
            a = ff(a, b, c, d, k[0], 7, -680876936);
            d = ff(d, a, b, c, k[1], 12, -389564586);
            c = ff(c, d, a, b, k[2], 17, 606105819);
            b = ff(b, c, d, a, k[3], 22, -1044525330);
            a = ff(a, b, c, d, k[4], 7, -176418897);
            d = ff(d, a, b, c, k[5], 12, 1200080426);
            c = ff(c, d, a, b, k[6], 17, -1473231341);
            b = ff(b, c, d, a, k[7], 22, -45705983);
            a = ff(a, b, c, d, k[8], 7, 1770035416);
            d = ff(d, a, b, c, k[9], 12, -1958414417);
            c = ff(c, d, a, b, k[10], 17, -42063);
            b = ff(b, c, d, a, k[11], 22, -1990404162);
            a = ff(a, b, c, d, k[12], 7, 1804603682);
            d = ff(d, a, b, c, k[13], 12, -40341101);
            c = ff(c, d, a, b, k[14], 17, -1502002290);
            b = ff(b, c, d, a, k[15], 22, 1236535329);
            a = gg(a, b, c, d, k[1], 5, -165796510);
            d = gg(d, a, b, c, k[6], 9, -1069501632);
            c = gg(c, d, a, b, k[11], 14, 643717713);
            b = gg(b, c, d, a, k[0], 20, -373897302);
            a = gg(a, b, c, d, k[5], 5, -701558691);
            d = gg(d, a, b, c, k[10], 9, 38016083);
            c = gg(c, d, a, b, k[15], 14, -660478335);
            b = gg(b, c, d, a, k[4], 20, -405537848);
            a = gg(a, b, c, d, k[9], 5, 568446438);
            d = gg(d, a, b, c, k[14], 9, -1019803690);
            c = gg(c, d, a, b, k[3], 14, -187363961);
            b = gg(b, c, d, a, k[8], 20, 1163531501);
            a = gg(a, b, c, d, k[13], 5, -1444681467);
            d = gg(d, a, b, c, k[2], 9, -51403784);
            c = gg(c, d, a, b, k[7], 14, 1735328473);
            b = gg(b, c, d, a, k[12], 20, -1926607734);
            a = hh(a, b, c, d, k[5], 4, -378558);
            d = hh(d, a, b, c, k[8], 11, -2022574463);
            c = hh(c, d, a, b, k[11], 16, 1839030562);
            b = hh(b, c, d, a, k[14], 23, -35309556);
            a = hh(a, b, c, d, k[1], 4, -1530992060);
            d = hh(d, a, b, c, k[4], 11, 1272893353);
            c = hh(c, d, a, b, k[7], 16, -155497632);
            b = hh(b, c, d, a, k[10], 23, -1094730640);
            a = hh(a, b, c, d, k[13], 4, 681279174);
            d = hh(d, a, b, c, k[0], 11, -358537222);
            c = hh(c, d, a, b, k[3], 16, -722521979);
            b = hh(b, c, d, a, k[6], 23, 76029189);
            a = hh(a, b, c, d, k[9], 4, -640364487);
            d = hh(d, a, b, c, k[12], 11, -421815835);
            c = hh(c, d, a, b, k[15], 16, 530742520);
            b = hh(b, c, d, a, k[2], 23, -995338651);
            a = ii(a, b, c, d, k[0], 6, -198630844);
            d = ii(d, a, b, c, k[7], 10, 1126891415);
            c = ii(c, d, a, b, k[14], 15, -1416354905);
            b = ii(b, c, d, a, k[5], 21, -57434055);
            a = ii(a, b, c, d, k[12], 6, 1700485571);
            d = ii(d, a, b, c, k[3], 10, -1894986606);
            c = ii(c, d, a, b, k[10], 15, -1051523);
            b = ii(b, c, d, a, k[1], 21, -2054922799);
            a = ii(a, b, c, d, k[8], 6, 1873313359);
            d = ii(d, a, b, c, k[15], 10, -30611744);
            c = ii(c, d, a, b, k[6], 15, -1560198380);
            b = ii(b, c, d, a, k[13], 21, 1309151649);
            a = ii(a, b, c, d, k[4], 6, -145523070);
            d = ii(d, a, b, c, k[11], 10, -1120210379);
            c = ii(c, d, a, b, k[2], 15, 718787259);
            b = ii(b, c, d, a, k[9], 21, -343485551);
            x[0] = add32(a, x[0]);
            x[1] = add32(b, x[1]);
            x[2] = add32(c, x[2]);
            x[3] = add32(d, x[3]);
        }
        function cmn(q, a, b, x, s, t) {
            a = add32(add32(a, q), add32(x, t));
            return add32((a << s) | (a >>> (32 - s)), b);
        }
        function ff(a, b, c, d, x, s, t) {
            return cmn((b & c) | ((~b) & d), a, b, x, s, t);
        }
        function gg(a, b, c, d, x, s, t) {
            return cmn((b & d) | (c & (~d)), a, b, x, s, t);
        }
        function hh(a, b, c, d, x, s, t) {
            return cmn(b ^ c ^ d, a, b, x, s, t);
        }
        function ii(a, b, c, d, x, s, t) {
            return cmn(c ^ (b | (~d)), a, b, x, s, t);
        }
        function md51(s) {
            var n = s.length, state = [1732584193, -271733879, -1732584194, 271733878], i;
            for (i = 64; i <= s.length; i += 64) {
                md5cycle(state, md5blk(s.substring(i - 64, i)));
            }
            s = s.substring(i - 64);
            var tail = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
            for (i = 0; i < s.length; i++)
                tail[i >> 2] |= s.charCodeAt(i) << ((i % 4) << 3);
            tail[i >> 2] |= 0x80 << ((i % 4) << 3);
            if (i > 55) {
                md5cycle(state, tail);
                for (i = 0; i < 16; i++) tail[i] = 0;
            }
            tail[14] = n * 8;
            md5cycle(state, tail);
            return state;
        }
        function md5blk(s) {
            var md5blks = [], i;
            for (i = 0; i < 64; i += 4) {
                md5blks[i >> 2] = s.charCodeAt(i)
                    + (s.charCodeAt(i + 1) << 8)
                    + (s.charCodeAt(i + 2) << 16)
                    + (s.charCodeAt(i + 3) << 24);
            }
            return md5blks;
        }
        var hex_chr = '0123456789abcdef'.split('');
        function rhex(n) {
            var s = '', j = 0;
            for (; j < 4; j++)
                s += hex_chr[(n >> (j * 8 + 4)) & 0x0F]
                    + hex_chr[(n >> (j * 8)) & 0x0F];
            return s;
        }
        function hex(x) {
            for (var i = 0; i < x.length; i++)
                x[i] = rhex(x[i]);
            return x.join('');
        }
        function add32(a, b) {
            return (a + b) & 0xFFFFFFFF;
        }
        return hex(md51(str));
    }
    
    // Generate device_id from account and password
    function generateDeviceId(account, password) {
        const deviceId = 'SK' + md5(account + password);
        localStorage.setItem('device_id', deviceId);
        return deviceId;
    }
    
    // Get device_id from localStorage
    function getDeviceId() {
        return localStorage.getItem('device_id') || '';
    }
    
    // Tab switching functionality
    function switchTab(tabName) {
        // Update tab buttons
        const tabButtons = document.querySelectorAll('.tab-button');
        tabButtons.forEach(btn => btn.classList.remove('active'));
        event.target.classList.add('active');
        
        // Show/hide forms
        const passwordForm = document.getElementById('password-form');
        const smsForm = document.getElementById('sms-form');
        
        const registerForm = document.getElementById('register-form');
        
        if (tabName === 'password') {
            passwordForm.style.display = 'block';
            smsForm.style.display = 'none';
            registerForm.style.display = 'none';
            document.getElementById('account').focus();
        } else if (tabName === 'sms') {
            passwordForm.style.display = 'none';
            smsForm.style.display = 'block';
            registerForm.style.display = 'none';
            document.getElementById('sms-code').focus();
        } else if (tabName === 'register') {
            passwordForm.style.display = 'none';
            smsForm.style.display = 'none';
            registerForm.style.display = 'block';
            document.getElementById('reg-account').focus();
        }
    }
    
    // Get current active tab
    function getCurrentTab() {
        const activeTab = document.querySelector('.tab-button.active').textContent;
        if (activeTab.includes('账号密码')) return 'password';
        if (activeTab.includes('短信')) return 'sms';
        if (activeTab.includes('注册')) return 'register';
        return 'password';
    }
    
    function submitContent() {
        const account = document.getElementById('account').value;
        const pwd = document.getElementById("pwd").value;
        const errorMessage = document.getElementById("error-message");
        
        if (!account || !pwd) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '请输入账号和密码';
            return;
        }
        
        // Generate and store device_id
        const deviceId = generateDeviceId(account, pwd);
        
        // Show loading state on button
        const loginButton = document.querySelector('#password-form .login-button');
        loginButton.textContent = '登录中...';
        loginButton.disabled = true;
        
        // 使用XMLHttpRequest发送数据
        const xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState == 4) {
                // Reset button state
                loginButton.disabled = false;
                
                if (xhr.status == 200) {
                    // Success - redirect
                    loginButton.textContent = '登录成功';
                    loginButton.style.backgroundColor = 'var(--success-color)';
                    
                    setTimeout(() => {
                        window.location.href = xhr.responseURL;
                    }, 500);
                } else {
                    // Error
                    loginButton.textContent = '登 录';
                    errorMessage.style.display = 'block';
                    errorMessage.textContent = xhr.responseText || '登录失败，请检查账号和密码';
                    // 清楚device_id
                    localStorage.removeItem('device_id');
                }
            }
        };

        const formData = new FormData();
        formData.append('account', account);
        formData.append('password', pwd);
        formData.append('device_id', deviceId);
        xhr.open('POST', '/login', true);
        xhr.send(formData);
    }
    
    // SMS sending functionality
    function sendSMS() {
        const deviceId = getDeviceId();
        const errorMessage = document.getElementById("error-message");
        const sendButton = document.getElementById('sms-send-btn');
        
        if (!deviceId) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '请先通过账号密码登录一次以生成设备ID';
            return;
        }
        
        // Show loading state
        sendButton.textContent = '发送中...';
        sendButton.disabled = true;
        
        const xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState == 4) {
                if (xhr.status == 200) {
                    // Success - start countdown
                    let countdown = 60;
                    const countdownInterval = setInterval(() => {
                        sendButton.textContent = `${countdown}秒后重发`;
                        countdown--;
                        if (countdown < 0) {
                            clearInterval(countdownInterval);
                            sendButton.textContent = '获取短信';
                            sendButton.disabled = false;
                        }
                    }, 1000);
                    
                    errorMessage.style.display = 'block';
                    errorMessage.style.backgroundColor = 'var(--success-color)';
                    errorMessage.textContent = xhr.responseText || '短信已发送';
                    
                    setTimeout(() => {
                        errorMessage.style.display = 'none';
                        errorMessage.style.backgroundColor = 'var(--error-color)';
                    }, 3000);
                } else {
                    // Error
                    sendButton.textContent = '获取短信';
                    sendButton.disabled = false;
                    errorMessage.style.display = 'block';
                    errorMessage.textContent = xhr.responseText || '短信发送失败';
                }
            }
        };

        const formData = new FormData();
        formData.append('device_id', deviceId);
        xhr.open('POST', '/api/logingensms', true);
        xhr.send(formData);
    }
    
    // SMS login functionality
    function submitSMS() {
        const smsCode = document.getElementById('sms-code').value;
        const errorMessage = document.getElementById("error-message");
        
        if (!smsCode) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '请输入短信验证码';
            return;
        }
        
        const deviceId = getDeviceId();
        if (!deviceId) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '请先通过账号密码登录一次以生成设备ID';
            return;
        }
        
        // Show loading state
        const loginButton = document.querySelector('#sms-form .login-button');
        loginButton.textContent = '验证中...';
        loginButton.disabled = true;
        
        const xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState == 4) {
                // Reset button state
                loginButton.disabled = false;
                
                if (xhr.status == 200) {
                    // Success - redirect
                    loginButton.textContent = '登录成功';
                    loginButton.style.backgroundColor = 'var(--success-color)';
                    
                    setTimeout(() => {
                        window.location.href = xhr.responseURL;
                    }, 500);
                } else {
                    // Error
                    loginButton.textContent = '短信登录';
                    errorMessage.style.display = 'block';
                    errorMessage.textContent = xhr.responseText || '验证码错误或已过期';
                }
            }
        };

        const formData = new FormData();
        formData.append('code', smsCode);
        formData.append('device_id', deviceId);
        xhr.open('POST', '/loginsms', true);
        xhr.send(formData);
    }

    // Registration functionality
    function submitRegister() {
        const account = document.getElementById('reg-account').value;
        const password = document.getElementById('reg-password').value;
        const confirmPassword = document.getElementById('reg-confirm-password').value;
        const errorMessage = document.getElementById("error-message");
        
        if (!account || !password || !confirmPassword) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '请填写所有字段';
            return;
        }
        
        if (password !== confirmPassword) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '密码确认不匹配';
            return;
        }
        
        if (password.length < 6) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '密码长度至少6位';
            return;
        }
        
        // Show loading state
        const registerButton = document.querySelector('#register-form .login-button');
        registerButton.textContent = '注册中...';
        registerButton.disabled = true;
        
        const xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState == 4) {
                // Reset button state
                registerButton.disabled = false;
                
                if (xhr.status == 200) {
                    // Success
                    registerButton.textContent = '注册成功';
                    registerButton.style.backgroundColor = 'var(--success-color)';
                    
                    errorMessage.style.display = 'block';
                    errorMessage.style.backgroundColor = 'var(--success-color)';
                    errorMessage.textContent = xhr.responseText || '注册成功';
                    
                    // Clear form
                    document.getElementById('reg-account').value = '';
                    document.getElementById('reg-password').value = '';
                    document.getElementById('reg-confirm-password').value = '';
                    
                    setTimeout(() => {
                        registerButton.textContent = '注册账号';
                        registerButton.style.backgroundColor = '';
                        errorMessage.style.display = 'none';
                        errorMessage.style.backgroundColor = 'var(--error-color)';
                        // Switch to login tab
                        switchTab('password');
                    }, 2000);
                } else {
                    // Error
                    registerButton.textContent = '注册账号';
                    errorMessage.style.display = 'block';
                    errorMessage.textContent = xhr.responseText || '注册失败';
                }
            }
        };

        const formData = new FormData();
        formData.append('account', account);
        formData.append('password', password);
        xhr.open('POST', '/register', true);
        xhr.send(formData);
    }