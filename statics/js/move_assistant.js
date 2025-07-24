// 智能助手悬浮按钮拖拽功能
function initAssistantButtonDrag() {
    const assistantBtn = document.querySelector('.assistant-floating-btn');
    if (!assistantBtn) return;
    
    let isDragging = false;
    let hasMovedDuringDrag = false;
    let startX = 0;
    let startY = 0;
    let currentX = 0;
    let currentY = 0;
    let initialX = 0;
    let initialY = 0;
    
    // 禁用默认的拖拽行为
    assistantBtn.draggable = false;
    
    // 获取当前位置
    function getCurrentPosition() {
        const rect = assistantBtn.getBoundingClientRect();
        return {
            x: rect.left + rect.width / 2,
            y: rect.top + rect.height / 2
        };
    }
    
    // 设置位置
    function setPosition(x, y) {
        const winWidth = window.innerWidth;
        const winHeight = window.innerHeight;
        const btnWidth = assistantBtn.offsetWidth;
        const btnHeight = assistantBtn.offsetHeight;
        
        // 限制在可视区域内
        const minX = btnWidth / 2;
        const maxX = winWidth - btnWidth / 2;
        const minY = btnHeight / 2;
        const maxY = winHeight - btnHeight / 2;
        
        x = Math.max(minX, Math.min(maxX, x));
        y = Math.max(minY, Math.min(maxY, y));
        
        assistantBtn.style.left = (x - btnWidth / 2) + 'px';
        assistantBtn.style.top = (y - btnHeight / 2) + 'px';
        assistantBtn.style.right = 'auto';
        assistantBtn.style.bottom = 'auto';
    }
    
    // 开始拖拽
    function startDrag(clientX, clientY) {
        isDragging = true;
        hasMovedDuringDrag = false;
        
        const pos = getCurrentPosition();
        initialX = pos.x;
        initialY = pos.y;
        startX = clientX;
        startY = clientY;
        currentX = pos.x;
        currentY = pos.y;
        
        assistantBtn.style.transition = 'none';
        assistantBtn.style.cursor = 'grabbing';
        assistantBtn.style.userSelect = 'none';
        
        // 防止页面滚动
        document.body.style.overflow = 'hidden';
    }
    
    // 拖拽中
    function duringDrag(clientX, clientY) {
        if (!isDragging) return;
        
        const deltaX = clientX - startX;
        const deltaY = clientY - startY;
        
        // 如果移动距离超过阈值，标记为已移动
        if (Math.abs(deltaX) > 5 || Math.abs(deltaY) > 5) {
            hasMovedDuringDrag = true;
        }
        
        currentX = initialX + deltaX;
        currentY = initialY + deltaY;
        
        setPosition(currentX, currentY);
    }
    
    // 结束拖拽
    function endDrag() {
        if (!isDragging) return;
        
        isDragging = false;
        assistantBtn.style.transition = 'all 0.3s ease';
        assistantBtn.style.cursor = 'pointer';
        assistantBtn.style.userSelect = '';
        
        // 恢复页面滚动
        document.body.style.overflow = '';
        
        // 吸附到边缘
        setTimeout(() => {
            snapToEdge();
        }, 50);
    }
    
    // 吸附到边缘
    function snapToEdge() {
        const winWidth = window.innerWidth;
        const btnWidth = assistantBtn.offsetWidth;
        const rect = assistantBtn.getBoundingClientRect();
        const centerX = rect.left + rect.width / 2;
        
        let targetX;
        if (centerX < winWidth / 2) {
            // 吸附到左边
            targetX = btnWidth / 2 + 15;
        } else {
            // 吸附到右边
            targetX = winWidth - btnWidth / 2 - 15;
        }
        
        assistantBtn.style.left = (targetX - btnWidth / 2) + 'px';
    }
    
    // 阻止点击事件（如果发生了拖拽）
    assistantBtn.addEventListener('click', function(e) {
        if (hasMovedDuringDrag) {
            e.preventDefault();
            e.stopPropagation();
            return false;
        }
    });
    
    // 鼠标事件
    assistantBtn.addEventListener('mousedown', function(e) {
        e.preventDefault();
        startDrag(e.clientX, e.clientY);
    });
    
    document.addEventListener('mousemove', function(e) {
        duringDrag(e.clientX, e.clientY);
    });
    
    document.addEventListener('mouseup', function(e) {
        endDrag();
    });
    
    // 触摸事件
    assistantBtn.addEventListener('touchstart', function(e) {
        //e.preventDefault();
        const touch = e.touches[0];
        startDrag(touch.clientX, touch.clientY);
    });
    
    document.addEventListener('touchmove', function(e) {
        if (!isDragging) return;
	if (hasMovedDuringDrag){
        	e.preventDefault();
	}
        const touch = e.touches[0];
        duringDrag(touch.clientX, touch.clientY);
    }, { passive: false });
    
    document.addEventListener('touchend', function(e) {
        endDrag();
    });
    
    // 窗口大小改变时重新定位
    window.addEventListener('resize', function() {
        if (!isDragging) {
            snapToEdge();
        }
    });
}

// 自动初始化（当DOM加载完成时）
document.addEventListener('DOMContentLoaded', function() {
    // 延迟一点时间确保其他脚本已加载
    setTimeout(() => {
        initAssistantButtonDrag();
    }, 100);
}); 
