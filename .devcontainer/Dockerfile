FROM golang:1.24.3


RUN apt-get update && apt-get install -y \
    git nano mosquitto-clients protobuf-compiler\
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
