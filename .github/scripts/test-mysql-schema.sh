#!/usr/bin/env bash
# Replays the legacy backend SQL twice on the MySQL server selected by MYSQL_IMAGE. The goose
# baseline is verified by the Migrations workflow. Deployment SQL belongs to the deployment repo.
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

require_one_row() {
	local description="$1"
	local query="$2"
	local count
	count="$(mysql_client -N -B "$database" -e "$query" </dev/null)"
	if [ "$count" != "1" ]; then
		echo "Missing ${description}; the check query returned '${count}'." >&2
		exit 1
	fi
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

require_one_row "branch_sequence table" \
	"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'branch_sequence'"
require_one_row "purchase_bill_product.cost_price column" \
	"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'purchase_bill_product' AND column_name = 'cost_price'"
require_one_row "purchase_bill_product.shelf_number column" \
	"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'purchase_bill_product' AND column_name = 'shelf_number'"

echo "Backend migration replay passed on ${MYSQL_IMAGE}."
