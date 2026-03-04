module llm

go 1.24.0

toolchain go1.24.10

require (
	auth v0.0.0
	codegen v0.0.0
	control v0.0.0
	mcp v0.0.0
	module v0.0.0
	mylog v0.0.0
)

require (
	config v0.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	uap v0.0.0 // indirect
	view v0.0.0 // indirect
)

replace (
	auth => ../auth
	codegen => ../codegen
	config => ../config
	control => ../control
	mcp => ../mcp
	module => ../module
	mylog => ../mylog
	uap => ../uap
	view => ../view
)
