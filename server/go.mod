module github.com/okitz/mqtt-log-pipeline/server

go 1.24

replace github.com/okitz/mqtt-log-pipeline/api => ../api

require (
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/okitz/mqtt-log-pipeline/api v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.10.0
	github.com/tysonmote/gommap v0.0.3
	tinygo.org/x/drivers v0.31.0
	tinygo.org/x/tinyfs v0.5.0
)

require (
	github.com/aperturerobotics/protobuf-go-lite v0.9.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
