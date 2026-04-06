FROM golang:1.25 AS builder

WORKDIR /build

# Copy both modules — gateway-service depends on user-service via replace directive
COPY user-service/ ./user-service/
COPY gateway-service/ ./gateway-service/

WORKDIR /build/gateway-service

RUN go build -o /gateway ./cmd/...

FROM debian:bookworm-slim

RUN useradd -r -u 1001 gateway

COPY --from=builder /gateway /gateway

USER gateway

ENTRYPOINT ["/gateway"]
