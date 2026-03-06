# Multi-stage build for LightStack
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN make build

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 lightstack && \
    adduser -D -u 1000 -G lightstack lightstack

# Copy binary and config
COPY --from=builder /build/bin/lightstack /usr/local/bin/lightstack
COPY --from=builder /build/config /etc/lightstack/config
COPY --from=builder /build/migrations /etc/lightstack/migrations

# Set ownership
RUN chown -R lightstack:lightstack /etc/lightstack

# Switch to non-root user
USER lightstack

# Expose ports
EXPOSE 5000 8774 9696 8776 9292

# Run
ENTRYPOINT ["/usr/local/bin/lightstack"]
CMD ["--config", "/etc/lightstack/config/lightstack.yaml", "--migrations", "/etc/lightstack/migrations"]
