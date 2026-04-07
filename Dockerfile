# Build arguments
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG APP_VERSION=dev

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

# Build argument available in this stage
ARG APP_VERSION

# Build binary with version from git tag
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${APP_VERSION}" \
    -o /go/bin/app \
    ./cmd/api

# Production stage - minimal distroless image
FROM gcr.io/distroless/static-debian12 AS prod

ARG APP_VERSION

# Labels for image metadata (version from build arg)
LABEL maintainer="Cepat Kilat Teknologi"
LABEL description="SNMP OLT Monitoring Service for ZTE C320"
LABEL version="${APP_VERSION}"

# Environment
ENV APP_ENV=production

# Copy binary from dev stage
COPY --from=dev /go/bin/app /app

# Expose port
EXPOSE 8081

# Run as non-root user (distroless nonroot user)
USER nonroot:nonroot

# Entrypoint
ENTRYPOINT ["/app"]
