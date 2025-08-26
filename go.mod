module go_blog

go 1.21

replace core => ./pkgs/core

replace module => ./pkgs/module

replace control => ./pkgs/control

replace view => ./pkgs/view

replace http => ./pkgs/http

replace mylog => ./pkgs/mylog

replace config => ./pkgs/config

replace persistence => ./pkgs/persistence

replace ioutils => ./pkgs/ioutils

replace auth => ./pkgs/auth

replace login => ./pkgs/login

replace comment => ./pkgs/comment

replace blog => ./pkgs/blog

replace email => ./pkgs/email

replace encryption => ./pkgs/encryption

replace search => ./pkgs/search

replace share => ./pkgs/share

replace statistics => ./pkgs/statistics

replace todolist => ./pkgs/todolist

replace yearplan => ./pkgs/yearplan

replace exercise => ./pkgs/exercise

replace reading => ./pkgs/reading

replace lifecountdown => ./pkgs/lifecountdown

replace mcp => ./pkgs/mcp

replace llm => ./pkgs/llm

replace sms => ./pkgs/sms

replace constellation => ./pkgs/constellation

replace tools => ./pkgs/tools

replace skill => ./pkgs/skill

require (
	blog v0.0.0
	comment v0.0.0
	config v0.0.0
	control v0.0.0
	http v0.0.0
	ioutils v0.0.0
	llm v0.0.0
	login v0.0.0
	mcp v0.0.0
	module v0.0.0
	mylog v0.0.0
	persistence v0.0.0
	search v0.0.0
	share v0.0.0
	sms v0.0.0
	statistics v0.0.0
	tools v0.0.0
	view v0.0.0
	auth v0.0.0
	constellation v0.0.0
	exercise v0.0.0
	lifecountdown v0.0.0
	reading v0.0.0
	todolist v0.0.0
	yearplan v0.0.0
	core v0.0.0
	skill v0.0.0

	github.com/bytedance/sonic v1.11.6 // indirect
	github.com/bytedance/sonic/loader v0.1.1 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.10.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.20.0 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.7 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.30.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
