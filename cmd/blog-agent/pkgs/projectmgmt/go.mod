module projectmgmt

go 1.24.0

toolchain go1.24.10

require (
	blog v0.0.0
	module v0.0.0
	mylog v0.0.0
)

require (
	auth v0.0.0 // indirect
	config v0.0.0 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.39.1 // indirect
	ioutils v0.0.0 // indirect
	persistence v0.0.0 // indirect
)

replace auth => ../auth

replace blog => ../blog

replace config => ../config

replace ioutils => ../ioutils

replace module => ../module

replace mylog => ../../../common/mylog

replace persistence => ../persistence
