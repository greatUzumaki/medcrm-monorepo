#!/usr/bin/env bash
# Build the landing and upload it to the VPS.
# Usage: DEPLOY_HOST=1.2.3.4 scripts/deploy.sh   (from apps/landing)
set -euo pipefail
: "${DEPLOY_HOST:?set DEPLOY_HOST to the VPS IP or hostname}"
KEY="${DEPLOY_KEY:-$HOME/.ssh/openmedai}"

SITE_URL=https://openmedai.ru npx -y pnpm@12.18.0 build
# --delete removes files of old builds; hashed assets are immutable so stale ones are safe to drop.
rsync -az --delete -e "ssh -i $KEY" dist/ "deploy@$DEPLOY_HOST:/srv/landing/"
echo "Deployed to https://openmedai.ru"
