module mcp

go 1.24.0

toolchain go1.24.10

require (
	auth v0.0.0
	config v0.0.0
	golang.org/x/net v0.50.0
	golang.org/x/text v0.34.0
	mylog v0.0.0
	view v0.0.0
)

require github.com/google/uuid v1.6.0 // indirect

replace mylog => ../mylog

replace config => ../config

replace view => ../view

replace auth => ../auth
