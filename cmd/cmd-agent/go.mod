module cmd-agent

go 1.24.0

toolchain go1.24.10

require (
	deploygen v0.0.0
	mylog v0.0.0
	uap v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
)

replace (
	deploygen => ../common/deploygen
	mylog => ../common/mylog
	uap => ../common/uap
)
