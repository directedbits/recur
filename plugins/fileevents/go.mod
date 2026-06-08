module github.com/directedbits/recur/plugins/fileevents

go 1.25.0

// TODO: remove replace directive once module path is a real, fetchable URL
replace github.com/directedbits/recur => ../..

require (
	github.com/directedbits/recur v0.0.0-00010101000000-000000000000
	github.com/helshabini/fsbroker v1.0.3
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
