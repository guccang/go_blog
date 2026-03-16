module codegen-agent

go 1.24.0

toolchain go1.24.10

require (
	agentbase v0.0.0
	uap v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace (
	agentbase => ../common/agentbase
	uap => ../common/uap
)
