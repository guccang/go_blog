module wechat

go 1.20

require (
	config v0.0.0
	mylog v0.0.0
)

replace config => ../config

replace mylog => ../mylog

replace persistence => ../persistence

replace ioutils => ../ioutils
