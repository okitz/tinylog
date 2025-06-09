.PHONY: protoc build tinybuild clean test size


PROTOC_GEN := $(GOPATH)/bin/protoc-gen-go-lite
BINARY=app
AMD64_OUT := build/amd64/$(BINARY)
TINYGO_OUT=build/tinygo/$(BINARY).uf2
BROKER_IP ?= tcp://mosquitto:1883

protoc:
	protoc --plugin protoc-gen-go-lite="${PROTOC_GEN}" \
		api/*/*.proto \
		--go-lite_out=. --go-lite_opt=features=marshal+unmarshal+size+json \
		--go-lite_opt=paths=source_relative

build:
	mkdir -p build/amd64
	go build -ldflags "-X main.brokerAddr=${BROKER_IP}" -o $(AMD64_OUT) ./cmd

tinybuild:
	mkdir -p build/tinygo
	tinygo build -ldflags "-X main.brokerAddr=${BROKER_IP}" -o $(TINYGO_OUT) -target=pico-w ./cmd

size:
	@echo "AMD binary size:"; ls -lh $(AMD64_OUT)
	@echo "TinyGo binary size:"; ls -lh $(TINYGO_OUT)
	
clean:
	# ビルド成果物を削除(dockerフォルダは除く)
	rm -rf build/tinygo/*
	rm -rf build/amd64/*

test:
	go test -v ./...