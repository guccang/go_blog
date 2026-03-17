module acp-agent

go 1.21

require (
	github.com/gorilla/websocket v1.5.0
)

replace agentbase => ../common/agentbase
replace uap => ../common/uap
