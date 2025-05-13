module github.com/okitz/mqtt-log-pipeline/agent

go 1.24.3

require github.com/eclipse/paho.mqtt.golang v1.5.0

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
)

replace github.com/okitz/mqtt-log-pipeline/common => ../common

require github.com/okitz/mqtt-log-pipeline/common v0.0.0
