compile:
	protoc --plugin protoc-gen-go-lite="${GOPATH}/bin/protoc-gen-go-lite" \
	api/*.proto --go-lite_out=. --go-lite_opt=features=marshal+unmarshal+size+json \
	 --go-lite_opt=paths=source_relative

test:
	go test -race ./...

tinybuild:
	tinygo build -target=challenger-rp2040 -o=./server/main.uf2 ./server/main.go 