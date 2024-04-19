
//
// markdown 渲染器
//
let rendererMD = new marked.Renderer();
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




