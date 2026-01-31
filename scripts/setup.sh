#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Council Arena Setup ==="

# Check prerequisites
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Aborting." >&2; exit 1; }
command -v node >/dev/null 2>&1 || { echo "Node.js is required but not installed. Aborting." >&2; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "npm is required but not installed. Aborting." >&2; exit 1; }

echo "Go version: $(go version)"
echo "Node version: $(node --version)"
echo "npm version: $(npm --version)"

# Generate secrets.go if it doesn't exist
SECRETS_FILE="backend/internal/config/secrets.go"
if [ ! -f "$SECRETS_FILE" ]; then
    echo ""
    echo "=== Generating secrets.go ==="
    echo "Enter your GitHub OAuth App credentials:"
    echo "(Create an OAuth App at https://github.com/settings/developers)"
    echo "(Set callback URL to: http://localhost:8080/auth/callback)"
    echo ""

    read -p "GitHub Client ID: " CLIENT_ID
    read -p "GitHub Client Secret: " CLIENT_SECRET

    if [ -z "$CLIENT_ID" ] || [ -z "$CLIENT_SECRET" ]; then
        echo "Error: Both Client ID and Client Secret are required."
        exit 1
    fi

    cat > "$SECRETS_FILE" << EOF
package config

const (
	DefaultGitHubClientID     = "${CLIENT_ID}"
	DefaultGitHubClientSecret = "${CLIENT_SECRET}"
)
EOF
    echo "Created $SECRETS_FILE"
fi

# Setup backend
echo ""
echo "=== Setting up backend ==="
cd backend
go mod download
go mod tidy
echo "Backend dependencies installed."

# Setup frontend
echo ""
echo "=== Setting up frontend ==="
cd ../frontend
npm install
echo "Frontend dependencies installed."

# Create data directory
cd ..
mkdir -p data

echo ""
echo "=== Setup complete! ==="
echo ""
echo "To start development:"
echo "  Terminal 1: cd backend && go run ./cmd/council"
echo "  Terminal 2: cd frontend && npm run dev"
echo ""
echo "Then open http://localhost:5173"
