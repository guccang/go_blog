// 粒子效果配置
document.addEventListener('DOMContentLoaded', function() {
    // 初始化粒子效果
    particlesJS('particles-js', {
        particles: {
            number: {
                value: 80,
                density: {
                    enable: true,
                    value_area: 800
                }
            },
            color: {
                value: ["#ff0080", "#ff8c00", "#40e0d0", "#ffff00"]
            },
            shape: {
                type: "circle",
                stroke: {
                    width: 0,
                    color: "#000000"
                }
            },
            opacity: {
                value: 0.5,
                random: true,
                anim: {
                    enable: true,
                    speed: 1,
                    opacity_min: 0.1,
                    sync: false
                }
            },
            size: {
                value: 3,
                random: true,
                anim: {
                    enable: true,
                    speed: 2,
                    size_min: 0.1,
                    sync: false
                }
            },
            line_linked: {
                enable: true,
                distance: 150,
                color: "#ffffff",
                opacity: 0.2,
                width: 1
            },
            move: {
                enable: true,
                speed: 2,
                direction: "none",
                random: true,
                straight: false,
                out_mode: "out",
                bounce: false,
                attract: {
                    enable: false,
                    rotateX: 600,
                    rotateY: 1200
                }
            }
        },
        interactivity: {
            detect_on: "canvas",
            events: {
                onhover: {
                    enable: true,
                    mode: "repulse"
                },
                onclick: {
                    enable: true,
                    mode: "push"
                },
                resize: true
            },
            modes: {
                grab: {
                    distance: 400,
                    line_linked: {
                        opacity: 1
                    }
                },
                bubble: {
                    distance: 400,
                    size: 40,
                    duration: 2,
                    opacity: 8,
                    speed: 3
                },
                repulse: {
                    distance: 100,
                    duration: 0.4
                },
                push: {
                    particles_nb: 4
                },
                remove: {
                    particles_nb: 2
                }
            }
        },
        retina_detect: true
    });
    
    // 2026年倒计时
    function updateCountdown() {
        const now = new Date();
        const targetDate = new Date('2026-01-01T00:00:00');
        const timeDiff = targetDate - now;
        
        if (timeDiff > 0) {
            const days = Math.floor(timeDiff / (1000 * 60 * 60 * 24));
            const hours = Math.floor((timeDiff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
            const minutes = Math.floor((timeDiff % (1000 * 60 * 60)) / (1000 * 60));
            const seconds = Math.floor((timeDiff % (1000 * 60)) / 1000);
            
            document.getElementById('days').textContent = days.toString().padStart(2, '0');
            document.getElementById('hours').textContent = hours.toString().padStart(2, '0');
            document.getElementById('minutes').textContent = minutes.toString().padStart(2, '0');
            document.getElementById('seconds').textContent = seconds.toString().padStart(2, '0');
            
            // 添加数字变化动画
            animateNumber('days', days);
            animateNumber('hours', hours);
            animateNumber('minutes', minutes);
            animateNumber('seconds', seconds);
        } else {
            // 2026年已到
            document.getElementById('days').textContent = '00';
            document.getElementById('hours').textContent = '00';
            document.getElementById('minutes').textContent = '00';
            document.getElementById('seconds').textContent = '00';
            
            // 显示庆祝消息
            const countdownElement = document.querySelector('.countdown h3');
            if (countdownElement) {
                countdownElement.innerHTML = '<i class="fas fa-calendar-alt"></i> 2026年已到来！';
                countdownElement.style.color = '#ff8c00';
                countdownElement.style.animation = 'textGlow 1s infinite alternate';
            }
        }
    }
    
    function animateNumber(elementId, newValue) {
        const element = document.getElementById(elementId);
        if (element) {
            element.style.transform = 'scale(1.2)';
            setTimeout(() => {
                element.style.transform = 'scale(1)';
            }, 300);
        }
    }
    
    // 初始更新
    updateCountdown();
    
    // 每秒更新一次
    setInterval(updateCountdown, 1000);
    
    // 添加鼠标跟随效果
    const container = document.querySelector('.container');
    if (container) {
        container.addEventListener('mousemove', function(e) {
            const cards = document.querySelectorAll('.card, .feature, .countdown');
            cards.forEach(card => {
                const rect = card.getBoundingClientRect();
                const x = e.clientX - rect.left;
                const y = e.clientY - rect.top;
                
                const centerX = rect.width / 2;
                const centerY = rect.height / 2;
                
                const rotateY = (x - centerX) / 25;
                const rotateX = (centerY - y) / 25;
                
                card.style.transform = `perspective(1000px) rotateX(${rotateX}deg) rotateY(${rotateY}deg)`;
            });
        });
        
        container.addEventListener('mouseleave', function() {
            const cards = document.querySelectorAll('.card, .feature, .countdown');
            cards.forEach(card => {
                card.style.transform = 'perspective(1000px) rotateX(0) rotateY(0)';
            });
        });
    }
    
    // 添加点击祝福语特效
    const blessing = document.querySelector('.blessing');
    if (blessing) {
        blessing.addEventListener('click', function() {
            // 创建粒子爆炸效果
            for (let i = 0; i < 20; i++) {
                createParticle(
                    blessing.getBoundingClientRect().left + blessing.offsetWidth / 2,
                    blessing.getBoundingClientRect().top + blessing.offsetHeight / 2
                );
            }
            
            // 添加文字放大效果
            blessing.style.transform = 'scale(1.2)';
            setTimeout(() => {
                blessing.style.transform = 'scale(1)';
            }, 300);
        });
    }
    
    function createParticle(x, y) {
        const particle = document.createElement('div');
        particle.style.position = 'fixed';
        particle.style.left = x + 'px';
        particle.style.top = y + 'px';
        particle.style.width = '10px';
        particle.style.height = '10px';
        particle.style.borderRadius = '50%';
        particle.style.backgroundColor = getRandomColor();
        particle.style.pointerEvents = 'none';
        particle.style.zIndex = '9999';
        
        document.body.appendChild(particle);
        
        const angle = Math.random() * Math.PI * 2;
        const speed = 2 + Math.random() * 3;
        const vx = Math.cos(angle) * speed;
        const vy = Math.sin(angle) * speed;
        
        let opacity = 1;
        const animate = () => {
            x += vx;
            y += vy;
            opacity -= 0.02;
            
            particle.style.left = x + 'px';
            particle.style.top = y + 'px';
            particle.style.opacity = opacity;
            
            if (opacity > 0) {
                requestAnimationFrame(animate);
            } else {
                document.body.removeChild(particle);
            }
        };
        
        animate();
    }
    
    function getRandomColor() {
        const colors = ['#ff0080', '#ff8c00', '#40e0d0', '#ffff00'];
        return colors[Math.floor(Math.random() * colors.length)];
    }
    
    // 添加页面加载完成动画
    setTimeout(() => {
        document.body.classList.add('loaded');
    }, 500);
    
    // 控制台欢迎消息
    console.log('%c🎉 欢迎万秀彬同志！ 🎉', 'color: #ff0080; font-size: 18px; font-weight: bold;');
    console.log('%c祝福2026健康强壮，力能鼎牛！', 'color: #ff8c00; font-size: 14px;');
});