#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
umask 077
[ ! -e .env ] || { echo '.env existe déjà ; les identifiants ne seront pas écrasés.' >&2; exit 1; }
command -v openssl >/dev/null
for key in POSTGRES_PASSWORD APP_DB_PASSWORD ADMIN_TOKEN READ_TOKEN METRICS_TOKEN; do
  value=$(openssl rand -hex 32)
  printf '%s=%s\n' "$key" "$value" >> .env
  if [ "$key" = METRICS_TOKEN ]; then mkdir -p .secrets; printf '%s' "$value" > .secrets/metrics_token; fi
done
printf 'HTTP_PORT=8080\nDB_POOL_MAX=10\nRATE_LIMIT_RPS=100\nRATE_LIMIT_BURST=200\nCORS_ORIGIN=\nOTEL_EXPORTER_OTLP_TRACES_ENDPOINT=\nDATABASE_URL=\n' >> .env
echo 'Configuration locale confidentielle créée. Démarrez avec docker compose up --build -d.'
