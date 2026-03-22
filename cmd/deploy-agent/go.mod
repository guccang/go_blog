module deploy-agent

go 1.24.0

toolchain go1.24.10

require (
	deploygen v0.0.0
	agentbase v0.0.0
	github.com/pkg/sftp v1.13.10
	github.com/zalando/go-keyring v0.2.6
	golang.org/x/crypto v0.48.0
	golang.org/x/term v0.40.0
	uap v0.0.0
)

require (
	al.essio.dev/pkg/shellescape v1.5.1 // indirect
	github.com/danieljoos/wincred v1.2.2 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace (
	deploygen => ../common/deploygen
	agentbase => ../common/agentbase
	uap => ../common/uap
)
