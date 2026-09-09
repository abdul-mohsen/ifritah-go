#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
migrations="$root/pkg/db/migrations"

require_text() {
	local file="$1" text="$2"
	if ! grep -Fq -- "$text" "$file"; then
		echo "$file is missing required migration-contract text: $text" >&2
		exit 1
	fi
}

branch_sequence="$migrations/0005_branch_sequence_counters.sql"
require_text "$branch_sequence" "CREATE TABLE IF NOT EXISTS \`branch_sequence\`"

if grep -Eq '^[[:space:]]*CREATE TABLE[[:space:]]+`?branch_sequence`?[[:space:]]*\(' "$branch_sequence"; then
	echo "$branch_sequence contains an unguarded branch_sequence table creation" >&2
	exit 1
fi

if grep -REn '^[[:space:]]*CREATE TABLE[[:space:]]+' "$migrations" |
	grep -Evq 'CREATE TABLE[[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS'; then
	echo "migrations must guard every direct CREATE TABLE statement" >&2
	exit 1
fi

if grep -REiq 'ADD[[:space:]]+COLUMN[[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS' "$migrations"; then
	echo "migrations must not use unsupported ADD COLUMN IF NOT EXISTS syntax" >&2
	exit 1
fi

for column in cost_price shelf_number; do
	require_text "$migrations/0005_purchase_bill_product_item_fields.sql" \
		"column_name = '${column}'"
done

echo "MySQL migration contract passed"
