module cmd-agent

go 1.24.0

toolchain go1.24.10

require (
	codegen v0.0.0
	config v0.0.0
	deploygen v0.0.0
	mylog v0.0.0
)

require (
	agentbase v0.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	uap v0.0.0 // indirect
)

replace (
	agentbase => ../common/agentbase
	codegen => ../blog-agent/pkgs/codegen
	config => ../blog-agent/pkgs/config
	deploygen => ../common/deploygen
	mylog => ../common/mylog
	uap => ../common/uap
)
