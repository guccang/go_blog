module blog-agent

go 1.24.0

toolchain go1.24.10

replace core => ./pkgs/core

replace module => ./pkgs/module

replace service => ./pkgs/service

replace http => ./pkgs/http

replace mylog => ../common/mylog

replace config => ./pkgs/config

replace persistence => ./pkgs/persistence

replace ioutils => ./pkgs/ioutils

replace auth => ./pkgs/auth

replace login => ./pkgs/login

replace blog => ./pkgs/blog

replace search => ./pkgs/search

replace share => ./pkgs/share

replace exercise => ./pkgs/exercise

replace reading => ./pkgs/reading

replace tools => ./pkgs/tools

replace account => ./pkgs/account

replace goal => ./pkgs/goal

require (
	auth v0.0.0
	blog v0.0.0
	config v0.0.0
	exercise v0.0.0
	goal v0.0.0
	http v0.0.0
	ioutils v0.0.0
	login v0.0.0
	module v0.0.0
	mylog v0.0.0
	persistence v0.0.0
	reading v0.0.0
	search v0.0.0
	share v0.0.0
	tools v0.0.0
	service v0.0.0
)

require (
	account v0.0.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/sys v0.41.0 // indirect
	modernc.org/libc v1.66.10 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.40.0 // indirect
)
