module account

go 1.21

replace config => ../config
replace mylog => ../../../common/mylog
replace module => ../module

require (
	config v0.0.0
	mylog v0.0.0
	module v0.0.0
)
