#!/bin/sh
# sqlc reads the goose migrations for the main schema. This copies the sqlc inputs, adds a probe
# migration with an Up and a Down section, and checks that the Up column reaches the generated
# model. The Down section must be ignored.
set -eu

root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

cp "$root/sqlc.yaml" "$work/"
mkdir -p "$work/pkg"
cp -R "$root/pkg/db" "$work/pkg/"

cat > "$work/pkg/db/goose/99999_probe.sql" <<'EOF'
-- +goose Up
ALTER TABLE account ADD COLUMN probe_marker INT NULL;

-- +goose Down
ALTER TABLE account DROP COLUMN probe_marker;
EOF

cd "$work"
sqlc generate

if ! grep -q 'ProbeMarker' pkg/db/gen/models.go; then
  echo "sqlc did not generate the probe migration's column." >&2
  exit 1
fi
echo "sqlc reads the goose migrations: the Up column reached the model and the Down was ignored."
