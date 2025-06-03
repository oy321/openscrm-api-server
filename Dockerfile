# Build stage
FROM golang:1.19-alpine AS builder

WORKDIR /app

# Install git and ca-certificates (needed for private repos and SSL)
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
ENV GOPROXY=https://goproxy.cn
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o api-server .

# Final stage
FROM alpine:latest

# Install ca-certificates for SSL
RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/api-server .

# Copy configuration files
COPY --from=builder /app/conf ./conf

# Copy any required static files
COPY --from=builder /app/docs ./docs

# Create non-root user
RUN adduser -D -s /bin/sh appuser
RUN chown -R appuser:appuser /app
USER appuser

EXPOSE 9001

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:9001/health || exit 1

CMD ["./api-server"] 