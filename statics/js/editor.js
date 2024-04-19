
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
