function isMobileDevice(){
	// 使用多种方法综合判断是否为移动设备
	const userAgent = navigator.userAgent.toLowerCase();
	const mobileKeywords = ['android', 'iphone', 'ipod', 'ipad', 'windows phone', 'mobile', 'tablet'];
	const isMobileUserAgent = mobileKeywords.some(keyword => userAgent.includes(keyword));
	
	// 使用屏幕尺寸作为辅助判断依据（通常认为小于768px宽度的是移动设备）
	const isMobileSize = window.innerWidth < 768;
	
	// 检查触摸支持
	const hasTouchSupport = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
	
	// 旧的orientation检查作为辅助判断
	const hasOrientation = typeof window.orientation !== 'undefined';
	
	// 综合多种判断标准做决策，至少满足两个条件才认为是移动设备
	let mobileProbability = 0;
	if (isMobileUserAgent) mobileProbability++;
	if (isMobileSize) mobileProbability++;
	if (hasTouchSupport) mobileProbability++;
	if (hasOrientation) mobileProbability++;
	
	const isMobile = mobileProbability >= 2;
	
	console.log("Device detection: userAgent:", isMobileUserAgent, 
		"| size:", isMobileSize, 
		"| touch:", hasTouchSupport, 
		"| orientation:", hasOrientation,
		"| result:", isMobile ? "mobile" : "pc");
	
	return isMobile;
}

function isPCDevice(){
	return !isMobileDevice();
}

function PageHistoryBack(){
	document.addEventListener('keydown', function(event) {
		console.log(`key=${event.key},code=${event.code}`);
        if (event.ctrlKey && event.key === "ArrowLeft"){
                javascript:history.back(-1);
        }
        if (event.ctrlKey && event.key === "ArrowRight"){
                javascript:history.forward();
        }
    });
}
