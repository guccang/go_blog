module codegen-agent

go 1.24.0

toolchain go1.24.10

require uap v0.0.0

replace uap => ../blog-agent/pkgs/uap

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
)
