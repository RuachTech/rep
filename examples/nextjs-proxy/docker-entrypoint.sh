#!/bin/sh
# docker-entrypoint.sh
#
# Starts the Next.js server in the background on :3000,
# then the REP gateway in proxy mode on :8080 in the foreground.
#
# The gateway receives REP_PUBLIC_* / REP_SENSITIVE_* / REP_GATEWAY_*
# from the container environment (Kubernetes ConfigMap / Secret / Deployment env).

set -e

echo "[entrypoint] Starting Next.js server on :3000"
NODE_ENV=production node /app/server.js &
NEXTJS_PID=$!

# Brief pause to let Next.js bind before the gateway starts proxying
sleep 1

echo "[entrypoint] Starting REP gateway on :${REP_GATEWAY_PORT:-8080} (proxy → localhost:3000)"
exec /usr/local/bin/rep-gateway \
  --mode proxy \
  --port "${REP_GATEWAY_PORT:-8080}" \
  --upstream "localhost:3000"
