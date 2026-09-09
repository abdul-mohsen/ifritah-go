#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

: "${MYSQL_IMAGE:?MYSQL_IMAGE is required}"
MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-root}"
database="ifritah_migration_test"

mysql_client() {
	docker run --rm -i --network host \
		-e "MYSQL_PWD=${MYSQL_ROOT_PASSWORD}" \
		"$MYSQL_IMAGE" \
		mysql --protocol=TCP \
			--connect-timeout=10 \
			-h "$MYSQL_HOST" \
			-P "$MYSQL_PORT" \
			-u root \
			"$@"
}

extract_first_mysql_heredoc() {
	local file="$1"
	awk '
		!capturing && $0 ~ /^[[:space:]]*run_mysql[[:space:]]+<<SQLEOF[[:space:]]*$/ {
			capturing = 1
			next
		}
		capturing && $0 ~ /^[[:space:]]*SQLEOF[[:space:]]*$/ {
			exit
		}
		capturing { print }
	' "$file"
}

verify_deployment_database_sql() {
	local deployment_dir="$1"
	local master_db="ci_master"
	local tenant_db="tenant_ci_mysql"
	local tenant_user="usr_ci_mysql"
	local tenant_host="%"
	local tenant_password="ci-password"
	local setup_sql create_sql

	setup_sql="$(extract_first_mysql_heredoc "$deployment_dir/scripts/setup.sh")"
	[ -n "$setup_sql" ] || {
		echo "Could not extract master database SQL from setup.sh." >&2
		return 1
	}
	setup_sql="$(printf '%s\n' "$setup_sql" |
		sed \
			-e "s/\${MYSQL_MASTER_DB}/${master_db}/g" \
			-e 's/\\`/`/g')"
	printf '%s\n' "$setup_sql" | mysql_client

	# Exercise the same compatibility helpers used by setup.sh and the
	# tenant-provenance upgrade path, including partially missing columns.
	# shellcheck disable=SC1091
	source "$deployment_dir/scripts/lib.sh"
	run_mysql() {
		mysql_client "$@"
	}
	ensure_tenant_provenance_columns "$master_db"
	ensure_tenant_audit_columns "$master_db"

	create_sql="$(extract_first_mysql_heredoc "$deployment_dir/scripts/create-tenant.sh")"
	[ -n "$create_sql" ] || {
		echo "Could not extract tenant database SQL from create-tenant.sh." >&2
		return 1
	}
	create_sql="$(printf '%s\n' "$create_sql" |
		sed \
			-e "s/\${TENANT_DB_NAME}/${tenant_db}/g" \
			-e "s/\${TENANT_DB_USER}/${tenant_user}/g" \
			-e "s/\${MYSQL_TENANT_HOST}/${tenant_host}/g" \
			-e "s/\${TENANT_DB_PASS}/${tenant_password}/g" \
			-e 's/\\`/`/g')"
	if printf '%s\n' "$create_sql" | grep -Fq '${'; then
		echo "Unresolved shell variables remain in create-tenant database SQL." >&2
		return 1
	fi

	for pass in 1 2; do
		echo "Applying create-tenant database SQL (replay ${pass}/2)"
		printf '%s\n' "$create_sql" | mysql_client
	done

	mysql_client -N -B -e \
		"SELECT COUNT(*) FROM mysql.user WHERE user='${tenant_user}' AND host='${tenant_host}'" |
		grep -qx '1'
	mysql_client -N -B -e \
		"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='${tenant_db}'" |
		grep -qx '1'
}

echo "Testing MySQL server version with client image ${MYSQL_IMAGE}"
mysql_client -N -B -e 'SELECT VERSION()'

mysql_client -e "DROP DATABASE IF EXISTS \`${database}\`; CREATE DATABASE \`${database}\`;"
mysql_client "$database" < pkg/db/schema/schema.sql
mysql_client < pkg/db/schema/car_part.sql

mapfile -t migrations < <(find pkg/db/migrations -maxdepth 1 -type f -name '*.sql' -print | sort)
mapfile -t runtime_migrations < <(find pkg/db/runtime_migrations -maxdepth 1 -type f -name '*.sql' -print | sort)

if [ "${#migrations[@]}" -eq 0 ]; then
	echo "No backend migrations were found." >&2
	exit 1
fi

for pass in 1 2; do
	echo "Applying backend migrations (replay ${pass}/2)"
	for migration in "${migrations[@]}"; do
		echo "  $(basename "$migration")"
		mysql_client "$database" < "$migration"
	done

	echo "Applying runtime migrations (replay ${pass}/2)"
	for migration in "${runtime_migrations[@]}"; do
		echo "  $(basename "$migration")"
		mysql_client "$database" < "$migration"
	done
done

mysql_client "$database" -e '
  SELECT 1
    FROM information_schema.tables
   WHERE table_schema = DATABASE()
     AND table_name = "branch_sequence";
  SELECT 1
    FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = "purchase_bill_product"
     AND column_name = "cost_price";
  SELECT 1
    FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = "purchase_bill_product"
     AND column_name = "shelf_number";
'

if [ -n "${DEPLOYMENT_DIR:-}" ]; then
	echo "Testing deployment database-creation SQL from ${DEPLOYMENT_DIR}"
	verify_deployment_database_sql "$DEPLOYMENT_DIR"
else
	echo "DEPLOYMENT_DIR is unset; skipping deployment database-creation SQL."
fi

echo "MySQL schema and migration replay passed for ${MYSQL_IMAGE}"
