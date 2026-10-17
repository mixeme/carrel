module carrel.dev/carrel/client

go 1.27

toolchain go1.27.1

require (
	carrel.dev/carrel/server v0.0.0
	carrel.dev/typecase v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.58.0
)

require golang.org/x/text v0.42.0 // indirect

replace carrel.dev/carrel/server => ../server

replace carrel.dev/typecase => ../../../typecase

tool carrel.dev/typecase/cmd/frame2go
