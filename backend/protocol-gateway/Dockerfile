FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /protocol-gateway ./cmd/gateway

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /protocol-gateway /protocol-gateway
EXPOSE 8081
ENTRYPOINT ["/protocol-gateway"]
