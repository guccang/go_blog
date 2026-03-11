module mcp

go 1.24.0

toolchain go1.24.10

require (
	config v0.0.0
	mylog v0.0.0
	statistics v0.0.0
)

replace mylog => ../../../common/mylog

replace config => ../config

replace statistics => ../statistics
