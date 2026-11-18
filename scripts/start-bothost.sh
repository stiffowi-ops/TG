#!/usr/bin/env bash
set -Eeuo pipefail

# Bothost may mount /app. Runtime data is stored there, while application
# binaries and static assets remain outside the mount in the Docker image.
export WEB_DIST=/opt/miniapp/web
export ADDR="0.0.0.0:${PORT:-8080}"
export DATA_DIR="${DATA_DIR:-/app/runtime}"

# Only one integrated process polls Telegram.
if [[ -z "${BOT_TOKEN:-}" ]]; then
  export BOT_TOKEN="${TEST_BOT_TOKEN:-${TELEGRAM_BOT_TOKEN:-${API_TOKEN:-}}}"
fi
if [[ -z "${MINIAPP_URL:-}" && -n "${DOMAIN:-}" ]]; then
  domain="${DOMAIN#https://}"
  domain="${domain#http://}"
  domain="${domain%%/*}"
  export MINIAPP_URL="https://${domain}"
fi

echo "[miniapp] Starting integrated application (HTTP + Telegram)." >&2
if [[ -z "${BOT_TOKEN:-}" ]]; then
  echo "[miniapp] BOT_TOKEN not configured: private actions disabled." >&2
fi
if [[ -z "${ADMIN_IDS:-}" ]]; then
  echo "[miniapp] ADMIN_IDS not configured: admin access disabled." >&2
fi
exec /usr/local/bin/miniapp-server
