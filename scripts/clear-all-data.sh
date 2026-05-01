#!/usr/bin/env bash
# =============================================================================
# Truncate every base table in the configured database. DESTRUCTIVE — wipes
# all rows. Schema (tables, views, indexes, FKs) is left intact.
#
# Reads the same env vars as the main app from .env in the repo root:
#   HOST=<host>:<port>   DBUSER   PASSWORD   DBNAME
#
# Refuses to run unless the caller passes --yes. Refuses to touch anything
# whose name starts with "prod" / "production" by default.
#
# Usage:
#   ./scripts/clear-all-data.sh --yes
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env}"

if [[ "${1:-}" != "--yes" ]]; then
  echo "Refusing to wipe data without explicit --yes flag." >&2
  echo "Usage: $0 --yes" >&2
  exit 2
fi

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

case "${DBNAME,,}" in
  prod*|production*)
    echo "error: refusing to truncate database '$DBNAME' (looks like production)" >&2
    exit 1
    ;;
esac

DB_HOST="${HOST%%:*}"
DB_PORT="${HOST##*:}"
[[ "$DB_HOST" == "$DB_PORT" ]] && DB_PORT=3306

command -v mysql >/dev/null || { echo "error: mysql client not installed" >&2; exit 1; }

mysql_run() {
  MYSQL_PWD="$PASSWORD" mysql \
    --protocol=TCP \
    -h "$DB_HOST" -P "$DB_PORT" \
    -u "$DBUSER" "$DBNAME" \
    --default-character-set=utf8mb4 \
    --skip-column-names --batch \
    "$@"
}

# List base tables (excludes views) in the active database.
mapfile -t TABLES < <(
  mysql_run -e "
    SELECT table_name
      FROM information_schema.tables
     WHERE table_schema = DATABASE()
       AND table_type = 'BASE TABLE'
     ORDER BY table_name;"
)

if [[ ${#TABLES[@]} -eq 0 ]]; then
  echo "no tables in '$DBNAME' — nothing to do."
  exit 0
fi

echo "[clear] truncating ${#TABLES[@]} tables in '$DBNAME' on $DB_HOST:$DB_PORT…"

# Build one batched statement: disable FK checks, truncate, re-enable.
{
  echo "SET FOREIGN_KEY_CHECKS = 0;"
  for t in "${TABLES[@]}"; do
    echo "TRUNCATE TABLE \`${t}\`;"
  done
  echo "SET FOREIGN_KEY_CHECKS = 1;"
} | mysql_run

echo "[clear] done."
