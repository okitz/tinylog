module github.com/okitz/mqtt-log-pipeline/server

go 1.24

replace github.com/okitz/mqtt-log-pipeline/api => ../api

require (
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/google/uuid v1.6.0
	github.com/okitz/mqtt-log-pipeline/api v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.10.0
	tinygo.org/x/tinyfs v0.5.0
)

require (
	github.com/aperturerobotics/json-iterator-lite v1.0.0 // indirect
	github.com/aperturerobotics/protobuf-go-lite v0.9.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/tools v0.1.1 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
