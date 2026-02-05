# Multi-stage Dockerfile for TCC-Matter Bridge

# Stage 1: Build Go backend
FROM golang:1.22-alpine AS go-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build with version info
ARG VERSION=dev
ARG BUILD_DATE
RUN CGO_ENABLED=1 go build \
    -ldflags "-X github.com/stephens/tcc-bridge/internal/web.Version=${VERSION} \
              -X github.com/stephens/tcc-bridge/internal/web.BuildDate=${BUILD_DATE}" \
    -o tcc-bridge ./cmd/server

# Stage 2: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /build

# Copy frontend files
COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Stage 3: Build Matter bridge
FROM node:20-alpine AS matter-builder

WORKDIR /build

# Copy matter bridge files
COPY matter-bridge/package*.json ./
RUN npm ci

COPY matter-bridge/ ./
RUN npm run build

# Stage 4: Final runtime image
FROM node:20-alpine

# Install runtime dependencies
RUN apk add --no-cache \
    sqlite-libs \
    ca-certificates \
    tzdata

WORKDIR /app

# Copy Go binary
COPY --from=go-builder /build/tcc-bridge /app/bin/tcc-bridge

# Copy frontend build
COPY --from=frontend-builder /build/dist /app/web/dist

# Copy Matter bridge
COPY --from=matter-builder /build/dist /app/matter-bridge/dist
COPY --from=matter-builder /build/node_modules /app/matter-bridge/node_modules
COPY --from=matter-builder /build/package.json /app/matter-bridge/

# Create data directories
RUN mkdir -p /app/data/.tcc-bridge /app/data/.matter && \
    chown -R node:node /app

# Switch to app user
USER node

# Expose ports
EXPOSE 8080 5540

# Set environment variables
ENV TCC_DATA_DIR=/app/data/.tcc-bridge
ENV MATTER_DATA_DIR=/app/data/.matter
ENV MATTER_BRIDGE_DIR=/app/matter-bridge

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/status || exit 1

# Run the application
CMD ["/app/bin/tcc-bridge"]
