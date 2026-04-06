module obs-agent

go 1.25.0

require (
	deploygen v0.0.0
	downloadticket v0.0.0
	obsstore v0.0.0
)

require (
	github.com/huaweicloud/huaweicloud-sdk-go-obs v3.25.9+incompatible // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace deploygen => ../common/deploygen

replace downloadticket => ../common/downloadticket

replace obsstore => ../common/obsstore
