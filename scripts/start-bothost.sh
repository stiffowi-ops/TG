#!/usr/bin/env bash
set -Eeuo pipefail

# Bothost's /app mount can replace files stored there. Always serve the
# frontend from the image's own filesystem, not the mounted directory.
export WEB_DIST=/opt/miniapp/web
export ADDR="0.0.0.0:${PORT:-8080}"

# Bothost sets BOT_TOKEN from the Telegram bot configuration. Keep the
# TEST_BOT_TOKEN override for standalone environments and local testing.
if [[ -z "${TEST_BOT_TOKEN:-}" ]]; then
  export TEST_BOT_TOKEN="${BOT_TOKEN:-${TELEGRAM_BOT_TOKEN:-${API_TOKEN:-}}}"
fi

# Bothost supplies DOMAIN when an HTTPS domain is attached to the service.
if [[ -z "${MINIAPP_URL:-}" && -n "${DOMAIN:-}" ]]; then
  domain="${DOMAIN#https://}"
  domain="${domain#http://}"
  domain="${domain%%/*}"
  export MINIAPP_URL="https://${domain}"
fi

if [[ -z "${TEST_BOT_TOKEN:-}" ]]; then
  echo "[miniapp] No Telegram bot token: starting HTTP application only." >&2
  exec /usr/local/bin/miniapp-server
fi

if [[ -z "${MINIAPP_URL:-}" ]]; then
  echo "[miniapp] HTTPS domain is not set yet: HTTP app runs, Telegram bot waits for DOMAIN or MINIAPP_URL." >&2
  exec /usr/local/bin/miniapp-server
fi

if [[ ! "${MINIAPP_URL}" =~ ^https://[^/]+ ]]; then
  echo "[miniapp] MINIAPP_URL must be a public HTTPS URL." >&2
  exit 1
fi

echo "[miniapp] Starting HTTP application and separate Telegram test bot." >&2
/usr/local/bin/miniapp-server &
server_pid=$!
/usr/local/bin/miniapp-testbot &
bot_pid=$!

shutdown() {
  trap - TERM INT
  kill -TERM "${server_pid}" "${bot_pid}" 2>/dev/null || true
  wait "${server_pid}" 2>/dev/null || true
  wait "${bot_pid}" 2>/dev/null || true
}
trap 'shutdown; exit 0' TERM INT

# If either process exits, stop its sibling and let Bothost restart the
# container rather than silently leaving a "running" broken bot.
exit_status=0
wait -n "${server_pid}" "${bot_pid}" || exit_status=$?
echo "[miniapp] A service exited (status=${exit_status}); stopping the container." >&2
shutdown
exit 1
