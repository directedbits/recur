module github.com/directedbits/recur/plugins/mqtt

go 1.25.0

require (
	github.com/directedbits/recur v0.0.0
	github.com/eclipse/paho.mqtt.golang v1.5.1
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// TODO: remove replace directive once module path is a real, fetchable URL
replace github.com/directedbits/recur => ../..
