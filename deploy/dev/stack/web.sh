#!/usr/bin/env bash
set -euo pipefail

port=${DHOW_DEV_WEB_PORT:-5173}
tools=/pnpm/tools
want=$(node -p "require('./package.json').packageManager.split('@')[1].split('+')[0]")

if [[ "$(pnpm --version 2>/dev/null || true)" != "$want" ]]; then
  printf '[dev] installing pnpm %s\n' "$want"
  npm install --global --prefix "$tools" --no-fund --no-audit "pnpm@$want"
fi

printf '[dev] installing the web dependencies\n'
pnpm install --frozen-lockfile --store-dir /pnpm/store --config.confirmModulesPurge=false

printf '[dev] starting Vite on http://localhost:%s\n' "$port"
exec pnpm exec vite --host 0.0.0.0 --port "$port" --strictPort
