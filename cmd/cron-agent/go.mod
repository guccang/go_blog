module cron-agent

go 1.24.0

require (
	deploygen v0.0.0
	agentbase v0.0.0
	github.com/google/uuid v1.6.0
	github.com/robfig/cron/v3 v3.0.1
	uap v0.0.0
)

require (
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace (
	deploygen => ../common/deploygen
	agentbase => ../common/agentbase
	uap => ../common/uap
)
