module fullstack-ws-server

go 1.22

toolchain go1.24.3

require (
	github.com/enesunal-m/azrealtime v1.2.0
	github.com/gorilla/websocket v1.5.3
)

require (
	github.com/klauspost/compress v1.10.3 // indirect
	nhooyr.io/websocket v1.8.7 // indirect
)

replace github.com/enesunal-m/azrealtime => ../../..
