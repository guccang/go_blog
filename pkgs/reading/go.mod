module reading

go 1.21

replace module => ../module
replace persistence => ../persistence
replace mylog => ../mylog

require (
	module v0.0.0-00010101000000-000000000000
	persistence v0.0.0-00010101000000-000000000000
	mylog v0.0.0-00010101000000-000000000000
) 