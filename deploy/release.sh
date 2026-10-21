#!/usr/bin/env bash
set -e

# ==============================================================================
# Sea Value Job Service - Build & Upload to S3 / Cloudflare R2
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Read Version input (argument or prompt)
VERSION="${1:-}"
if [ -z "${VERSION}" ]; then
  read -p "Enter Release Version (e.g., v1.0.0 or press Enter for 'latest'): " input_ver
  VERSION="${input_ver:-latest}"
fi

echo "=================================================="
echo "  Building & Uploading Sea Value Job Service [${VERSION}]"
echo "=================================================="

# 1. Build Binaries
mkdir -p "${SCRIPT_DIR}"
echo "🔨 Building Linux amd64 binary (for Intel/AMD Linux Server)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "${SCRIPT_DIR}/sea-value-job-linux-amd64" "${ROOT_DIR}/cmd"

echo "🔨 Building Linux arm64 binary (for ARM64 / Raspberry Pi)..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "${SCRIPT_DIR}/sea-value-job-linux-arm64" "${ROOT_DIR}/cmd"

echo "🔨 Building Windows amd64 binary (for Windows Server/PC)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "${SCRIPT_DIR}/sea-value-job-windows-amd64.exe" "${ROOT_DIR}/cmd"

echo "🔨 Building Windows installer .exe..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "${SCRIPT_DIR}/install.exe" "${SCRIPT_DIR}/installer"

echo "✅ Build completed successfully."

# 2. Upload using Go S3 Uploader (No aws cli required!)
echo "☁️ Uploading releases to S3 / Cloudflare R2 (releases/custom/sea-value-job/${VERSION}/)..."
(cd "${ROOT_DIR}" && go run "${SCRIPT_DIR}/uploader/main.go" -version "${VERSION}")

PUBLIC_STORAGE_URL="${RELEASE_S3_PUBLIC_ENDPOINT:-https://storage.centaccess.com}"

echo ""
echo "=================================================="
echo "  🎉 Release Deployment Complete! [${VERSION}]"
echo "=================================================="
echo "• Install Command (for Ubuntu/Linux):"
echo "  curl -fsSL ${PUBLIC_STORAGE_URL}/releases/custom/sea-value-job/${VERSION}/install.sh | sudo bash"
echo ""
echo "• Install on Windows:"
echo "  Download & Run: ${PUBLIC_STORAGE_URL}/releases/custom/sea-value-job/${VERSION}/install.exe"
echo "=================================================="
