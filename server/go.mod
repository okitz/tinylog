module github.com/okitz/mqtt-log-pipeline/server

go 1.24

replace github.com/okitz/mqtt-log-pipeline/api => ../api

require (
	github.com/okitz/mqtt-log-pipeline/api v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.10.0
	github.com/tysonmote/gommap v0.0.3
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
