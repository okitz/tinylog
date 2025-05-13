FROM golang:1.22

WORKDIR /app

RUN apt-get update && apt-get install -y git nano mosquitto-clients && rm -rf /var/lib/apt/lists/*
ENV GO111MODULE=on
