module github.com/okitz/mqtt-log-pipeline/agent

go 1.24

replace github.com/okitz/mqtt-log-pipeline/api => ../api

require (
	github.com/eclipse/paho.mqtt.golang v1.5.0
	github.com/okitz/mqtt-log-pipeline/api v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
)
