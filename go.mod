module go_blog

go 1.20

replace module => ./go_blog_pkgs/module

replace control => ./go_blog_pkgs/control

replace view => ./go_blog_pkgs/view

replace http => ./go_blog_pkgs/http

replace mylog => ./go_blog_pkgs/mylog

replace config => ./go_blog_pkgs/config

replace persistence => ./go_blog_pkgs/persistence

replace ioutils => ./go_blog_pkgs/ioutils

replace auth => ./go_blog_pkgs/auth

replace login => ./go_blog_pkgs/login

replace comment => ./go_blog_pkgs/comment

replace blog => ./go_blog_pkgs/blog

require (
	blog v0.0.0-00010101000000-000000000000
	comment v0.0.0-00010101000000-000000000000
	config v0.0.0-00010101000000-000000000000
	control v0.0.0-00010101000000-000000000000
	http v0.0.0-00010101000000-000000000000
	ioutils v0.0.0-00010101000000-000000000000
	login v0.0.0-00010101000000-000000000000
	module v0.0.0-00010101000000-000000000000
	mylog v0.0.0-00010101000000-000000000000
	persistence v0.0.0-00010101000000-000000000000
	view v0.0.0-00010101000000-000000000000
)

require (
	auth v0.0.0-00010101000000-000000000000 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.30.0 // indirect
)
