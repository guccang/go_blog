module go_blog

go 1.20

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

replace cooperation => ./pkgs/cooperation

replace statistics => ./pkgs/statistics

replace todolist => ./pkgs/todolist

replace yearplan => ./pkgs/yearplan


require (
	blog v0.0.0-00010101000000-000000000000
	comment v0.0.0-00010101000000-000000000000
	config v0.0.0-00010101000000-000000000000
	control v0.0.0-00010101000000-000000000000
	cooperation v0.0.0-00010101000000-000000000000
	http v0.0.0-00010101000000-000000000000
	ioutils v0.0.0-00010101000000-000000000000
	login v0.0.0-00010101000000-000000000000
	module v0.0.0-00010101000000-000000000000
	mylog v0.0.0-00010101000000-000000000000
	persistence v0.0.0-00010101000000-000000000000
	search v0.0.0-00010101000000-000000000000
	share v0.0.0-00010101000000-000000000000
	statistics v0.0.0-00010101000000-000000000000
	todolist v0.0.0-00010101000000-000000000000
	yearplan v0.0.0-00010101000000-000000000000
	view v0.0.0-00010101000000-000000000000
)

require (
	auth v0.0.0-00010101000000-000000000000 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.30.0 // indirect
)
