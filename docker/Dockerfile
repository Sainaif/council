# Stage 1: Build frontend
FROM node:25-alpine AS frontend-builder

WORKDIR /app/frontend

# Install dependencies
COPY frontend/package*.json ./
RUN npm ci

# Build frontend
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.25-alpine AS backend-builder

# Build arguments for secrets
ARG GITHUB_CLIENT_ID
ARG GITHUB_CLIENT_SECRET

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Download Go modules
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source
COPY backend/ ./

# Generate secrets.go from build args
RUN printf 'package config\n\nconst (\n\tDefaultGitHubClientID     = "%s"\n\tDefaultGitHubClientSecret = "%s"\n)\n' \
    "${GITHUB_CLIENT_ID}" "${GITHUB_CLIENT_SECRET}" > ./internal/config/secrets.go

# Build backend
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o council ./cmd/council

# Stage 3: Final image (using Debian for glibc compatibility with Copilot CLI)
FROM debian:bookworm-slim

# Install runtime dependencies and GitHub Copilot CLI
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata git curl wget \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL https://gh.io/copilot-install | bash \
    && copilot --version

# Create non-root user
RUN groupadd --system council && useradd --system --gid council council

WORKDIR /app

# Copy built artifacts
COPY --from=backend-builder /app/council .
COPY --from=frontend-builder /app/frontend/dist ./static

# Create data directory
RUN mkdir -p /data && chown -R council:council /data /app

# Switch to non-root user
USER council

# Environment defaults
ENV DATABASE_PATH=/data/council.db
ENV PORT=8080
ENV ENV=production

EXPOSE 8080

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./council"]
