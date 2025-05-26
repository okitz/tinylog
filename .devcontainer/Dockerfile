FROM golang:1.24.3


RUN apt-get update && apt-get install -y \
    git nano unzip mosquitto-clients \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app


ARG PROTOC_VERSION=26.1
ARG ARCH=x86_64  # or aarch_64 など、適宜選択

RUN curl -LO https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-${ARCH}.zip && \
    unzip protoc-${PROTOC_VERSION}-linux-${ARCH}.zip -d /usr/local && \
    rm protoc-${PROTOC_VERSION}-linux-${ARCH}.zip

    curl -Lo protoc.zip "https://github.com/protocolbuffers/protobuf/releases/latest/download/protoc-${PROTOC_VERSION}-linux-x86_64.zip"
