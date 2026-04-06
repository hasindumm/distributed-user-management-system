FROM golang:1.25 AS builder

WORKDIR /build

# Copy both modules — user-service depends on its own module
# gateway-service also needs user-service via replace directive
# but here we only need user-service itself
COPY user-service/ ./user-service/

WORKDIR /build/user-service

RUN go build -o /user-service ./cmd/...

FROM debian:bookworm-slim

# Create non-root user for security
RUN useradd -r -u 1001 userservice

# Copy compiled binary
COPY --from=builder /user-service /user-service

# Copy migrations — user-service runs these at startup
# main.go calls: migrate.New("file://db/migrations", ...)
# binary runs from / so db/migrations resolves to /db/migrations
COPY --from=builder /build/user-service/db/migrations /db/migrations

USER userservice

ENTRYPOINT ["/user-service"]
