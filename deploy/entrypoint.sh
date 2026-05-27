#!/bin/sh
set -e

# ============================================================
# 从环境变量生成 config.yaml
# 所有配置集中管理在 docker-compose.yaml 的 environment 中
# ============================================================

CONFIG_FILE="/app/config.yaml"

# CORS allowed_origins: 逗号分隔的环境变量转 YAML 列表
CORS_ORIGINS_YAML=""
if [ -n "${CORS_ORIGINS:-}" ]; then
    CORS_ORIGINS_YAML=$(echo "$CORS_ORIGINS" | tr ',' '\n' | sed 's/^/    - "/;s/$/"/')
fi

cat > "$CONFIG_FILE" << EOF
server:
  port: ${SERVER_PORT:-8080}
  mode: ${SERVER_MODE:-release}
  read_timeout: ${SERVER_READ_TIMEOUT:-30}
  write_timeout: ${SERVER_WRITE_TIMEOUT:-30}
  idle_timeout: ${SERVER_IDLE_TIMEOUT:-60}

database:
  type: ${DB_TYPE:-mysql}
  host: ${DB_HOST:-localhost}
  port: ${DB_PORT:-3306}
  username: ${DB_USERNAME:-root}
  password: ${DB_PASSWORD:-}
  database: ${DB_NAME:-kafka_management}
  ssl_mode: ${DB_SSL_MODE:-disable}
  max_open_conns: ${DB_MAX_OPEN_CONNS:-50}
  max_idle_conns: ${DB_MAX_IDLE_CONNS:-10}
  conn_max_lifetime: ${DB_CONN_MAX_LIFETIME:-3600}

jwt:
  secret: ${JWT_SECRET:-change-this-secret-key}
  access_token_expire: ${JWT_ACCESS_TOKEN_EXPIRE:-3600}
  refresh_token_expire: ${JWT_REFRESH_TOKEN_EXPIRE:-604800}
  issuer: ${JWT_ISSUER:-kafka-management-platform}

encryption:
  key: ${ENCRYPTION_KEY:-}

log:
  level: ${LOG_LEVEL:-info}
  format: ${LOG_FORMAT:-json}
  output_path: ${LOG_OUTPUT:-stdout}

victoriametrics:
  write_url: ${VM_WRITE_URL:-http://localhost:8428/insert/0/prometheus}
  query_url: ${VM_QUERY_URL:-http://localhost:8428/select/0/prometheus}
  enabled: ${VM_ENABLED:-true}

syncworker:
  interval: ${SYNC_INTERVAL:-30}

collector:
  concurrency: ${COLLECTOR_CONCURRENCY:-10}
  interval: ${COLLECTOR_INTERVAL:-30}

session:
  idle_timeout: ${SESSION_IDLE_TIMEOUT:-15}

cookie:
  domain: "${COOKIE_DOMAIN:-}"
  secure: ${COOKIE_SECURE:-false}
  same_site: "${COOKIE_SAME_SITE:-Lax}"
  path: "${COOKIE_PATH:-/}"

cors:
  allowed_origins:
${CORS_ORIGINS_YAML}
EOF

echo "[entrypoint] config.yaml generated at $CONFIG_FILE"
echo "[entrypoint] starting: $@"
exec "$@"
