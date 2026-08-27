#!/usr/bin/env bash
# Deploy the eventfulranges paint demo to Fly.io.
#
#   ./deploy-fly.sh
#
# Backend: private, Flycast-only app (eventfulranges-app), built from this
# directory (the repo root) because the demo module replaces the library with
# the parent module. Config: ./fly.toml, build: ./Dockerfile.
# Proxy: public oauth2-proxy front door (eventfulranges), in demo/paint/proxy.
#
# Admin: the backend gates /admin by ADMIN_EMAILS (a comma-separated list of
# emails). Set ADMIN_EMAILS in the environment before deploying to store it as
# a Fly secret, or set the secret once by hand.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v fly &> /dev/null; then
  echo "error: fly CLI is not installed - https://fly.io/docs/getting-started/installing-flyctl/" >&2
  exit 1
fi
if ! fly auth whoami &> /dev/null; then
  echo "error: not logged in to Fly.io - run: fly auth login" >&2
  exit 1
fi

BACKEND_APP="eventfulranges-app"
PROXY_APP="eventfulranges"

# --- ensure the apps exist --------------------------------------------------
ensure_app() {
  local app="$1"
  if ! fly apps list 2>/dev/null | awk '{print $1}' | grep -qx "$app"; then
    fly apps create "$app" --org personal --yes
  fi
}
ensure_app "$BACKEND_APP"
ensure_app "$PROXY_APP"

# --- oauth2-proxy secrets ---------------------------------------------------
# The cookie secret is generated once; the GitHub OAuth client credentials are
# optional here (they can also be set by hand with `fly secrets set`).
secret_names() { fly secrets list -a "$PROXY_APP" 2>/dev/null | awk 'NR>1 {print $1}'; }

if ! secret_names | grep -qx "OAUTH2_PROXY_COOKIE_SECRET"; then
  # oauth2-proxy base64-URL-decodes this value, so it must be URL-safe base64
  # of exactly 16/24/32 bytes. `openssl rand -base64` uses standard base64
  # (+ and /), which oauth2-proxy rejects and then treats as a raw 44-byte
  # string, so generate URL-safe base64 instead.
  fly secrets set -a "$PROXY_APP" "OAUTH2_PROXY_COOKIE_SECRET=$(python3 -c 'import base64, os; print(base64.urlsafe_b64encode(os.urandom(32)).decode())')"
fi
if [ -n "${OAUTH2_PROXY_CLIENT_ID:-}" ] && ! secret_names | grep -qx "OAUTH2_PROXY_CLIENT_ID"; then
  fly secrets set -a "$PROXY_APP" "OAUTH2_PROXY_CLIENT_ID=$OAUTH2_PROXY_CLIENT_ID"
fi
if [ -n "${OAUTH2_PROXY_CLIENT_SECRET:-}" ] && ! secret_names | grep -qx "OAUTH2_PROXY_CLIENT_SECRET"; then
  fly secrets set -a "$PROXY_APP" "OAUTH2_PROXY_CLIENT_SECRET=$OAUTH2_PROXY_CLIENT_SECRET"
fi

# --- admin access -----------------------------------------------------------
# The admin area is gated by a comma-separated list of emails, set as a secret
# on the backend. Provide it via ADMIN_EMAILS (or set it by hand with
# `fly secrets set -a eventfulranges-app ADMIN_EMAILS=...`).
if [ -n "${ADMIN_EMAILS:-}" ] && ! fly secrets list -a "$BACKEND_APP" 2>/dev/null | awk 'NR>1 {print $1}' | grep -qx "ADMIN_EMAILS"; then
  fly secrets set -a "$BACKEND_APP" "ADMIN_EMAILS=$ADMIN_EMAILS"
fi

# --- backend ----------------------------------------------------------------
# Private, Flycast-only app: allocate a private IPv6 and no public IPs, per
# https://fly.io/docs/blueprints/autostart-internal-apps/
echo "deploying backend: $BACKEND_APP"
fly deploy --app "$BACKEND_APP" --flycast --no-public-ips --ha=false

# --- proxy -----------------------------------------------------------------
echo "deploying proxy: $PROXY_APP"
fly deploy --app "$PROXY_APP" demo/paint/proxy

echo ""
echo "done. public URL: https://eventfulranges.fly.dev/"
