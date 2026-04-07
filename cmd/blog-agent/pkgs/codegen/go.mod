module codegen

go 1.24.0

toolchain go1.24.10

require (
	agentbase v0.0.0
	config v0.0.0
	github.com/gorilla/websocket v1.5.0
	mylog v0.0.0
	uap v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace (
	agentbase => ../../../common/agentbase
	config => ../config
	mylog => ../../../common/mylog
	uap => ../../../common/uap
)
