//
// markdown 渲染器
//
let rendererMD = new marked.Renderer();

function mdToHtml(markdownString){
	  marked.setOptions({
          renderer: rendererMD,
          gfm: true,
          tables: true,
          breaks: false,
          pedantic: false,
          sanitize: false,
          smartLists: true,
          smartypants: false,
		      highlight: function (code) {
            return hljs.highlightAuto(code).value;
          }
        });
	
        return marked.parse(markdownString);
}

function mdRender(markdownString){
	      marked.setOptions({
          renderer: rendererMD,
          gfm: true,
          tables: true,
          breaks: false,
          pedantic: false,
          sanitize: false,
          smartLists: true,
          smartypants: false,
		  highlight: function (code) {
            return hljs.highlightAuto(code).value;
          }
        });
		//console.log(markdownString)
		document.getElementById('md').innerHTML = marked.parse(markdownString);
}
	

function checkTime() {
    // 获取当前时间
    var currentTime = new Date();

    // 获取当前小时
    var currentHour = currentTime.getHours();
   // 获取当前分钟
    var currentMinutes = currentTime.getMinutes();
    // 获取当前秒数
    var currentSeconds = currentTime.getSeconds();

    // 判断是否过了21点
    if (currentHour == 18 && currentMinutes>= 55 || currentHour>18) {
        console.log("已过了21点！");
        // 在这里可以执行相应的操作
		var body = document.getElementById('body')
		body.setAttribute("class", "th_black"); 
    } else {
        console.log("还没到21点！");
        // 在这里可以执行相应的操作
    }
}

// 每隔一段时间执行一次checkTime函数
var intervalId = setInterval(checkTime, 5000);


//
// aes-cbc  crypt-js
// 加密
function aesEncrypt(data, key) {
  if (key.length > 32) {
    key = key.slice(0, 32);
  }
  var cypherKey = CryptoJS.enc.Utf8.parse(key);
  CryptoJS.pad.ZeroPadding.pad(cypherKey, 4);

  var iv = CryptoJS.SHA256(key).toString();
  var cfg = { iv: CryptoJS.enc.Utf8.parse(iv) };
  return CryptoJS.AES.encrypt(data, cypherKey, cfg).toString();
}

// 解密
function aesDecrypt(data,key){
  if (key.length > 32) {
    key = key.slice(0, 32);
  }
	var cypherKey = CryptoJS.enc.Utf8.parse(key);
	CryptoJS.pad.ZeroPadding.pad(cypherKey, 4);
    var iv = CryptoJS.SHA256(key).toString();
	var cfg = { iv: CryptoJS.enc.Utf8.parse(iv) };
	var decrypt = CryptoJS.AES.decrypt({ciphertext:CryptoJS.enc.Base64.parse(data)},cypherKey,cfg)
	var txt = decrypt.toString(CryptoJS.enc.Utf8)
	return txt
}


var data = "小红帽"
var keys = [
  "Guccang@123456",
  "1234", 
  "16bit secret key", 
  "16bit secret key1234567", 
  "16bit secret key12345678",
  "16bit secret key16bit secret ke",
  "16bit secret key16bit secret key",
  "16bit secret key16bit secret key1",
]
// 加密解密测试
function aesTest(){
	for (let i = 0; i < keys.length; i++) {
		var en = aesEncrypt(data, keys[i])
		console.log("en",en)
		var de = aesDecrypt(en,keys[i])
		console.log("de",de)
	}
}

// 在文档加载完成后初始化文本编辑器功能
document.addEventListener('DOMContentLoaded', function() {
    initAutoResizeTextareas();
    
    // 页面加载后立即调整所有文本框的高度
    const textareas = document.querySelectorAll('textarea');
    textareas.forEach(textarea => {
        setTimeout(() => {
            adjustTextareaHeight(textarea);
        }, 100);
    });
});

// 初始化自动调整大小的文本框
function initAutoResizeTextareas() {
    // 获取所有需要自动调整大小的文本框
    const textareas = document.querySelectorAll('textarea');
    
    textareas.forEach(textarea => {
        // 初始调整大小
        adjustTextareaHeight(textarea);
        
        // 添加输入事件监听器
        textarea.addEventListener('input', function() {
            adjustTextareaHeight(this);
        });
        
        // 添加焦点事件监听器
        textarea.addEventListener('focus', function() {
            adjustTextareaHeight(this);
        });
        
        // 添加点击事件监听器（以防其他交互可能导致内容变化）
        textarea.addEventListener('click', function() {
            adjustTextareaHeight(this);
        });
    });
    
    // 监听预览按钮点击事件，当切换回编辑模式时重新调整文本框高度
    const previewButtons = document.querySelectorAll('.preview-btn');
    previewButtons.forEach(button => {
        button.addEventListener('click', function() {
            // 使用setTimeout确保DOM更新后再调整高度
            setTimeout(() => {
                const textareas = document.querySelectorAll('textarea');
                textareas.forEach(textarea => {
                    if (textarea.offsetParent !== null) { // 只处理可见的文本框
                        adjustTextareaHeight(textarea);
                    }
                });
            }, 100);
        });
    });
}

// 调整文本框高度以适应内容
function adjustTextareaHeight(textarea) {
    return;

    if (!textarea || textarea.offsetParent === null) return; // 如果文本框不存在或不可见，则退出
    
    // 保存滚动条位置
    const scrollPos = window.pageYOffset || document.documentElement.scrollTop;
    
    // 重置高度，让我们准确计算实际内容的高度
    textarea.style.height = 'auto';
    
    // 如果内容高度低于最小高度，则使用最小高度
    const minHeight = getComputedStyle(textarea).getPropertyValue('min-height');
    const contentHeight = textarea.scrollHeight;
    const minHeightPx = parseInt(minHeight) || 100; // 如果无法获取min-height，则默认为100px
    
    // 设置新高度（取内容高度和最小高度的较大值）
    textarea.style.height = Math.max(contentHeight, minHeightPx) + 'px';
    
    // 恢复滚动条位置，防止页面跳动
    window.scrollTo(0, scrollPos);
}

// 公开函数以允许其他脚本调用
window.adjustAllTextareas = function() {
    const textareas = document.querySelectorAll('textarea');
    textareas.forEach(textarea => {
        adjustTextareaHeight(textarea);
    });
};

// Markdown 预览功能
function markdownToHtml(markdownText) {
    // 使用 marked 库将 Markdown 转换为 HTML
    return marked.parse(markdownText);
}
