#!/bin/sh
# Read-only package diagnostic; no credential values, tokens, or bodies are emitted.
set -eu

ROOT=${WORKBRIDGE_RELAY_ROOT:-/var/packages/WorkBridgeRelay}
VAR=${SYNOPKG_PKGVAR:-"$ROOT/var"}
HEALTH_URL_FILE="$VAR/health-url"
WORKBRIDGE="$ROOT/target/bin/workbridge-mcp"
TUNNEL="$ROOT/target/bin/tunnel-client-runtime"
CONFIG="$VAR/workbridge-relay.json"

if [ ! -x "$WORKBRIDGE" ] || [ ! -x "$TUNNEL" ]; then
    printf '%s\n' 'runtime=missing_binary'
    exit 2
fi
if [ ! -f "$CONFIG" ] || [ ! -r "$CONFIG" ]; then
    printf '%s\n' 'config=missing_or_unreadable'
    exit 2
fi
if [ ! -r "$VAR/tunnel-id" ] || [ ! -r "$VAR/runtime-api-key" ]; then
    printf '%s\n' 'credentials=missing_or_unreadable'
    exit 2
fi
if [ -L "$HEALTH_URL_FILE" ] || [ ! -r "$HEALTH_URL_FILE" ]; then
    printf '%s\n' 'tunnel=not_observed'
    exit 2
fi
URL=$(cat "$HEALTH_URL_FILE")
case "$URL" in
    http://127.0.0.1:*) ;;
    *) printf '%s\n' 'tunnel=invalid_local_health_url'; exit 2 ;;
esac
PORT=${URL##*:}
case "$PORT" in
    ''|*[!0-9]*) printf '%s\n' 'tunnel=invalid_local_health_port'; exit 2 ;;
esac
[ "$PORT" -ge 1 ] 2>/dev/null && [ "$PORT" -le 65535 ] 2>/dev/null || {
    printf '%s\n' 'tunnel=invalid_local_health_port'
    exit 2
}
if ! command -v curl >/dev/null 2>&1; then
    printf '%s\n' 'tunnel=probe_unavailable'
    exit 2
fi
if ! curl --noproxy '*' --silent --show-error --fail --max-time 3 "$URL/healthz" >/dev/null 2>&1; then
    printf '%s\n' 'tunnel=unhealthy'
    exit 2
fi
printf '%s\n' 'tunnel=healthy'
if curl --noproxy '*' --silent --show-error --fail --max-time 3 "$URL/readyz" >/dev/null 2>&1; then
    printf '%s\n' 'tunnel=ready'
    printf '%s\n' 'mcp=not_independently_verified'
    exit 0
fi
printf '%s\n' 'tunnel=not_ready'
exit 2
