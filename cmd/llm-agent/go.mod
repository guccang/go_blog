module llm-agent

go 1.24.0

require (
	deploygen v0.0.0
	agentbase v0.0.0
	uap v0.0.0
)

replace uap => ../common/uap

replace agentbase => ../common/agentbase

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace deploygen => ../common/deploygen
