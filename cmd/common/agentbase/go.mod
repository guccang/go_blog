module agentbase

go 1.24.0

require (
	golang.org/x/net v0.50.0
	golang.org/x/text v0.34.0
	uap v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
)

replace uap => ../uap
