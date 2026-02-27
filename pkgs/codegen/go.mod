module codegen

go 1.24.0

toolchain go1.24.10

require (
	config v0.0.0
	mylog v0.0.0
)

replace (
	config => ../config
	mylog => ../mylog
)
