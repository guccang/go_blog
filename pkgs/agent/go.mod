module agent

go 1.18

require (
	control v0.0.0
	llm v0.0.0
	mcp v0.0.0
	module v0.0.0
	mylog v0.0.0
	statistics v0.0.0
	github.com/gorilla/websocket v1.5.0
)

replace (
	control => ../control
	llm => ../llm
	mcp => ../mcp
	module => ../module
	mylog => ../mylog
	statistics => ../statistics
)
