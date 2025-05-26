FROM golang:1.24.3 AS builder
WORKDIR /app
COPY . .
RUN go build -o server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/server /usr/local/bin/server
ENTRYPOINT ["server"]