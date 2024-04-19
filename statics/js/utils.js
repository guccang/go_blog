
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
