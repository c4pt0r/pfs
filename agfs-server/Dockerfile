# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o agfs-server cmd/server/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/agfs-server .

# Copy configuration files to root
COPY --from=builder /build/config.example.yaml /config.example.yaml
COPY --from=builder /build/config.example.yaml /config.yaml

# Create directory for localfs mount
RUN mkdir -p /data

# Expose default port
EXPOSE 8080

# Run the server
ENTRYPOINT ["./agfs-server"]
CMD ["-c", "/config.yaml"]
