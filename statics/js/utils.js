
function isMobileDevice(){
	if (typeof window.orientation != "undefined"){
		console.log("is mobile")
		return  true
	}else{
		console.log("is pc")
		return false
	}
}

function isPCDevice(){
	return isMobileDevice() == 0
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
