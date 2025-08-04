module tools

go 1.21

replace mylog => ../mylog
replace view => ../view

require (
	mylog v0.0.0
	view v0.0.0
)