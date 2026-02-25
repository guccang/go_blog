module agent

go 1.21

toolchain go1.24.1

require (
	control v0.0.0
	email v0.0.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.0
	llm v0.0.0
	mcp v0.0.0
	module v0.0.0
	mylog v0.0.0
	statistics v0.0.0
)

require (
	auth v0.0.0 // indirect
	config v0.0.0 // indirect
	view v0.0.0 // indirect
)

replace (
	auth => ../auth
	config => ../config
	control => ../control
	email => ../email
	http => ../http
	ioutils => ../ioutils
	llm => ../llm
	mcp => ../mcp
	module => ../module
	mylog => ../mylog
	persistence => ../persistence
	statistics => ../statistics
	view => ../view
)
