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

# Stage 3: Final image
FROM alpine:3.23

# Install runtime dependencies including GitHub CLI for Copilot
RUN apk add --no-cache ca-certificates tzdata git curl bash \
    && curl -sL https://github.com/cli/cli/releases/download/v2.77.0/gh_2.77.0_linux_amd64.tar.gz | tar xz -C /tmp \
    && mv /tmp/gh_2.77.0_linux_amd64/bin/gh /usr/local/bin/ \
    && rm -rf /tmp/gh_2.77.0_linux_amd64 \
    && chmod +x /usr/local/bin/gh

# Create non-root user
RUN addgroup -S council && adduser -S council -G council

WORKDIR /app

# Copy built artifacts
COPY --from=backend-builder /app/council .
COPY --from=frontend-builder /app/frontend/dist ./static

# Create data directory and set up Copilot CLI
RUN mkdir -p /data && chown -R council:council /data /app

# Switch to non-root user
USER council

# Install GitHub Copilot extension (requires gh to be available)
RUN gh extension install github-copilot || true

# Environment defaults
ENV DATABASE_PATH=/data/council.db
ENV PORT=8080
ENV ENV=production
# GitHub token should be provided at runtime for Copilot authentication
# ENV GITHUB_TOKEN=

EXPOSE 8080

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./council"]
