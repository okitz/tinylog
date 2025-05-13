FROM golang:1.24.3

WORKDIR /app

RUN apt-get update && apt-get install -y git nano && rm -rf /var/lib/apt/lists/*

CMD ["sh", "-c", "go mod tidy && go run main.go"]
