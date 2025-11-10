# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o owlmail ./cmd/owlmail

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1000 owlmail && \
    adduser -D -u 1000 -G owlmail owlmail

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/owlmail /app/owlmail

# Copy web static files
COPY --from=builder /build/web /app/web

# Create mail storage directory
RUN mkdir -p /app/mail && \
    chown -R owlmail:owlmail /app

# Switch to non-root user
USER owlmail

# Expose ports
# 1025: SMTP port
# 1080: Web API port
EXPOSE 1025 1080

# Set default environment variables
ENV OWLMAIL_SMTP_PORT=1025
ENV OWLMAIL_WEB_PORT=1080
ENV OWLMAIL_SMTP_HOST=0.0.0.0
ENV OWLMAIL_WEB_HOST=0.0.0.0
ENV OWLMAIL_MAIL_DIR=/app/mail

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:1080/healthz || exit 1

# Start application
ENTRYPOINT ["/app/owlmail"]

