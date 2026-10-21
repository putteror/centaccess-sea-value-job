#!/usr/bin/env bash
set -e

# ==============================================================================
# Sea Value Job Service - Automated Linux Installer
# ==============================================================================

INSTALL_DIR="/opt/centaccess/sea-value-job"
SERVICE_NAME="sea-value-job"
JOB_VERSION="${JOB_VERSION:-latest}"
BASE_STORAGE_URL="${STORAGE_URL:-https://storage.centaccess.com}/releases/custom/sea-value-job/${JOB_VERSION}"

echo "=================================================="
echo "  Installing Sea Value Job Service"
echo "=================================================="

# 1. ตรวจสอบสิทธิ์ Root
if [ "$EUID" -ne 0 ]; then
  echo "❌ Error: Please run this script with sudo or as root."
  exit 1
fi

# 2. ตรวจสอบ Architecture อัตโนมัติ (amd64 / arm64)
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)
    TARGET_ARCH="amd64"
    ;;
  aarch64|arm64)
    TARGET_ARCH="arm64"
    ;;
  *)
    echo "❌ Error: Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

BINARY_NAME="sea-value-job-linux-${TARGET_ARCH}"
echo "🔍 Detected System Architecture: $ARCH -> Target: $TARGET_ARCH"

# 3. เตรียมโฟลเดอร์สำหรับติดตั้ง
echo "📁 Creating installation directory at $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR"

# 4. หยุด Service เก่า (ถ้ากำลัง run อยู่) เพื่อปลดล็อค Binary ก่อน Overwrite
if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
  echo "⏹ Stopping existing ${SERVICE_NAME} service before update..."
  systemctl stop "${SERVICE_NAME}"
fi

# 5. ดาวน์โหลด หรือ ย้าย Binary
rm -f "$INSTALL_DIR/sea-value-job"

if [ -f "./$BINARY_NAME" ]; then
  echo "📦 Found local binary: ./$BINARY_NAME. Installing..."
  cp "./$BINARY_NAME" "$INSTALL_DIR/sea-value-job"
elif [ -f "./sea-value-job" ]; then
  echo "📦 Found local binary: ./sea-value-job. Installing..."
  cp "./sea-value-job" "$INSTALL_DIR/sea-value-job"
else
  DOWNLOAD_URL="${BASE_STORAGE_URL}/${BINARY_NAME}"
  echo "🌐 Local binary not found. Downloading from: $DOWNLOAD_URL ..."
  curl -fsSL -L -o "$INSTALL_DIR/sea-value-job" "$DOWNLOAD_URL" || {
    echo "❌ Error: Failed to download binary from $DOWNLOAD_URL"
    exit 1
  }
fi

chmod +x "$INSTALL_DIR/sea-value-job"

# 6. สร้างไฟล์ .env เริ่มต้น (ถ้ายังไม่มี)
if [ ! -f "$INSTALL_DIR/.env" ]; then
  echo "⚙️ Creating default .env at $INSTALL_DIR/.env..."

  cat << EOF > "$INSTALL_DIR/.env"
# CentAccess API Endpoints
APP_API_URL=http://localhost:3001
APP_OVERTIME_URL=http://localhost:3001

# Authentication: Option A - Static JWT Bearer Token / API Key
APP_API_KEY=

# Authentication: Option B - Username / Password for Automated JWT Login & Refresh
APP_USERNAME=sea_value_admin
APP_PASSWORD=P@ssw0rd!234

# Optional: Tenant UUID & HMAC Secret Key (for M2M Push Integration)
APP_TENANT_ID=
APP_SECRET_KEY=

# HTTP Web UI & Management Port
HTTP_PORT=9090

# Optional: Custom paths for config and devices JSON files
CONFIG_FILE_PATH=./config.json
DEVICES_FILE_PATH=./devices.json
EOF
  echo "✅ Generated default .env at $INSTALL_DIR/.env"
else
  echo "ℹ️ Existing .env found at $INSTALL_DIR/.env. Skipping overwrite."
fi

# 7. สร้าง Systemd Service Unit
echo "🛠 Creating Systemd service (/etc/systemd/system/${SERVICE_NAME}.service)..."
cat << EOF > /etc/systemd/system/${SERVICE_NAME}.service
[Unit]
Description=Sea Value Integration Job Service
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/sea-value-job
EnvironmentFile=${INSTALL_DIR}/.env
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

# 8. Reload & Start Service
echo "🚀 Starting ${SERVICE_NAME} service..."
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"

echo ""
echo "=================================================="
echo "  ✅ Sea Value Job Service Installed Successfully!"
echo "=================================================="
echo "• Install Path : $INSTALL_DIR"
echo "• Config File  : $INSTALL_DIR/.env"
echo "• Web Portal   : http://localhost:9090"
echo "• Check Status : sudo systemctl status $SERVICE_NAME"
echo "• View Logs    : sudo journalctl -u $SERVICE_NAME -f"
echo "=================================================="
