compile:
	protoc --plugin protoc-gen-go-lite="${GOPATH}/bin/protoc-gen-go-lite" \
	api/*.proto --go-lite_out=. --go-lite_opt=features=marshal+unmarshal+size+equal+clone \
	 --go-lite_opt=paths=source_relative

test:
	go test -race ./...