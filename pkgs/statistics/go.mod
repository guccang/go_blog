module statistics

go 1.24.0

toolchain go1.24.10

require (
	blog v0.0.0
	comment v0.0.0
	exercise v0.0.0
	module v0.0.0
	mylog v0.0.0
	reading v0.0.0
	taskbreakdown v0.0.0
	todolist v0.0.0
	yearplan v0.0.0
)

require (
	account v0.0.0-00010101000000-000000000000 // indirect
	auth v0.0.0-00010101000000-000000000000 // indirect
	config v0.0.0 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.39.1 // indirect
	ioutils v0.0.0-00010101000000-000000000000 // indirect
	persistence v0.0.0-00010101000000-000000000000 // indirect
)

replace blog => ../blog

replace comment => ../comment

replace exercise => ../exercise

replace module => ../module

replace mylog => ../mylog

replace todolist => ../todolist

replace reading => ../reading

replace yearplan => ../yearplan

replace taskbreakdown => ../taskbreakdown

replace config => ../config

replace persistence => ../persistence

replace ioutils => ../ioutils

replace account => ../account

replace auth => ../auth

replace view => ../view
