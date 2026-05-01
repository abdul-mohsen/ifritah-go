#!/usr/bin/env bash
# =============================================================================
# Seed the three demo accounts (admin / manager / employee) used by the
# frontend RBAC e2e suite. Idempotent: safe to run repeatedly.
#
# Reads the same env vars as the main app from .env in the repo root:
#   HOST=<host>:<port>   DBUSER   PASSWORD   DBNAME
#
# Requires:
#   * mysql client (e.g. apt: mysql-client, brew: mysql-client)
#   * htpasswd     (e.g. apt: apache2-utils, brew: httpd) for bcrypt hashing
#
# Usage:
#   ./scripts/seed-demo-users.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "error: $ENV_FILE not found" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a

: "${HOST:?HOST not set}"
: "${DBUSER:?DBUSER not set}"
: "${PASSWORD:?PASSWORD not set}"
: "${DBNAME:?DBNAME not set}"

DB_HOST="${HOST%%:*}"
DB_PORT="${HOST##*:}"
[[ "$DB_HOST" == "$DB_PORT" ]] && DB_PORT=3306

for bin in mysql htpasswd; do
  command -v "$bin" >/dev/null || { echo "error: $bin not installed" >&2; exit 1; }
done

# Generate a Go-bcrypt-compatible hash ($2a$) for a plaintext password.
bcrypt_hash() {
  htpasswd -bnBC 10 "" "$1" | tr -d ':\n' | sed 's/^\$2y/\$2a/'
}

ADMIN_HASH="$(bcrypt_hash admin)"
MANAGER_HASH="$(bcrypt_hash manager)"
EMPLOYEE_HASH="$(bcrypt_hash employee)"

mysql_run() {
  MYSQL_PWD="$PASSWORD" mysql \
    --protocol=TCP \
    -h "$DB_HOST" -P "$DB_PORT" \
    -u "$DBUSER" "$DBNAME" \
    --default-character-set=utf8mb4 \
    "$@"
}

mysql_run <<SQL
SET NAMES utf8mb4;

-- ---- demo users (idempotent upsert) -------------------------------------
INSERT INTO user (username, full_name, password, role, is_active) VALUES
  ('admin',    'Demo Admin',    '${ADMIN_HASH}',    'admin',    1),
  ('manager',  'Demo Manager',  '${MANAGER_HASH}',  'manager',  1),
  ('employee', 'Demo Employee', '${EMPLOYEE_HASH}', 'employee', 1)
ON DUPLICATE KEY UPDATE
  full_name = VALUES(full_name),
  password  = VALUES(password),
  role      = VALUES(role),
  is_active = 1;

-- ---- view-only permissions for the employee account ---------------------
-- Admin/manager bypass row grants by role.
INSERT INTO user_permission (user_id, resource, can_view, can_add, can_edit, can_delete)
SELECT u.id, r.resource, 1, 0, 0, 0
FROM user u
CROSS JOIN (
  SELECT 'invoices'  AS resource UNION ALL
  SELECT 'products'           UNION ALL
  SELECT 'clients'            UNION ALL
  SELECT 'orders'             UNION ALL
  SELECT 'stores'             UNION ALL
  SELECT 'suppliers'
) r
WHERE u.username = 'employee'
ON DUPLICATE KEY UPDATE can_view = 1;
SQL

echo "[seed] demo users ensured: admin / manager / employee"
