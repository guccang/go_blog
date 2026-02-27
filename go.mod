module go_blog

go 1.24.0

toolchain go1.24.10

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

replace account => ./pkgs/account

replace gomoku => ./pkgs/gomoku

replace linkup => ./pkgs/linkup

replace finance => ./pkgs/finance

replace tetris => ./pkgs/tetris

replace minesweeper => ./pkgs/minesweeper

replace fruitcrush => ./pkgs/fruitcrush

replace taskbreakdown => ./pkgs/taskbreakdown

replace agent => ./pkgs/agent

replace wechat => ./pkgs/wechat

replace codegen => ./pkgs/codegen

require (
	agent v0.0.0
	auth v0.0.0
	blog v0.0.0
	comment v0.0.0
	config v0.0.0
	control v0.0.0
	exercise v0.0.0
	http v0.0.0
	ioutils v0.0.0
	llm v0.0.0
	login v0.0.0
	mcp v0.0.0
	module v0.0.0
	mylog v0.0.0
	persistence v0.0.0
	reading v0.0.0
	search v0.0.0
	share v0.0.0
	sms v0.0.0
	statistics v0.0.0
	tools v0.0.0
	view v0.0.0
)

require (
	account v0.0.0 // indirect
	constellation v0.0.0 // indirect
	email v0.0.0 // indirect
	finance v0.0.0 // indirect
	fruitcrush v0.0.0 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.39.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gomoku v0.0.0 // indirect
	lifecountdown v0.0.0 // indirect
	linkup v0.0.0 // indirect
	minesweeper v0.0.0 // indirect
	skill v0.0.0 // indirect
	taskbreakdown v0.0.0 // indirect
	tetris v0.0.0 // indirect
	todolist v0.0.0 // indirect
	wechat v0.0.0 // indirect
	codegen v0.0.0 // indirect
	yearplan v0.0.0 // indirect
)
