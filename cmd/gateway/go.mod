module gateway

go 1.24.0

replace uap => ../../pkgs/uap

require uap v0.0.0

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
)
