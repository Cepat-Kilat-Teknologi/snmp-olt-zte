# Build arguments for multi-arch support
ARG TARGETPLATFORM
ARG BUILDPLATFORM

# Development stage with hot reload
FROM golang:1.26-alpine AS dev

# Install air for hot reload
RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Copy dependency files first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=1.0.0" \
    -o /go/bin/app \
    ./cmd/api

# Production stage - minimal distroless image
FROM gcr.io/distroless/static-debian12 AS prod

# Labels for image metadata
LABEL maintainer="Cepat Kilat Teknologi"
LABEL description="SNMP OLT Monitoring Service for ZTE C320"
LABEL version="1.0.0"

# Environment
ENV APP_ENV=production

# Copy binary from dev stage
COPY --from=dev /go/bin/app /app

# No config file needed - all configuration from environment variables
# Board/PON OID mappings are generated dynamically using mathematical formulas

# Expose port
EXPOSE 8081

# Run as non-root user (distroless nonroot user)
USER nonroot:nonroot

# Entrypoint
ENTRYPOINT ["/app"]
