module github.com/directedbits/recur/plugins/webhook

go 1.25.0

require (
	github.com/directedbits/recur v0.0.0
	golang.org/x/time v0.15.0
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// TODO: remove replace directive once module path is a real, fetchable URL
replace github.com/directedbits/recur => ../..
