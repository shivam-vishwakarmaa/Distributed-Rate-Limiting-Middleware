# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o rate-limiter-server cmd/server/main.go
RUN go build -o demo-backend demo/backend/main.go

# Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/rate-limiter-server .
COPY --from=builder /app/demo-backend .
COPY config.yaml .

CMD ["./rate-limiter-server"]
