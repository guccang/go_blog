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
                submitContent();
            }
        });
    });
    
    function submitContent() {
        const account = document.getElementById('account').value;
        const pwd = document.getElementById("pwd").value;
        const errorMessage = document.getElementById("error-message");
        
        if (!account || !pwd) {
            errorMessage.style.display = 'block';
            errorMessage.textContent = '请输入账号和密码';
            return;
        }
        
        // Show loading state on button
        const loginButton = document.querySelector('.login-button');
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
                }
            }
        };

        const formData = new FormData();
        formData.append('account', account);
        formData.append('password', pwd);
        xhr.open('POST', '/login', true);
        xhr.send(formData);
    }