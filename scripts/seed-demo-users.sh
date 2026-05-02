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

ADMIN_HASH="$(bcrypt_hash admin123)"
MANAGER_HASH="$(bcrypt_hash manager123)"
EMPLOYEE_HASH="$(bcrypt_hash employee123)"

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

-- ---- demo company + stores (idempotent) ---------------------------------
-- A user with no company_id has no accessible stores, which makes every
-- list endpoint that scopes by store return 400. Attach the three demo
-- accounts to a single "Demo Co" with two stores so the e2e suite can
-- exercise multi-branch flows.
SET @company_id = (SELECT id FROM company WHERE name = 'Demo Co' LIMIT 1);
INSERT INTO company (name, vat_number)
SELECT 'Demo Co', '300000000000003'
WHERE @company_id IS NULL;
SET @company_id = (SELECT id FROM company WHERE name = 'Demo Co' LIMIT 1);

UPDATE user SET company_id = @company_id
 WHERE username IN ('admin','manager','employee');

INSERT INTO store (company_id, name)
SELECT @company_id, 'Demo Store 1'
WHERE NOT EXISTS (
  SELECT 1 FROM store WHERE company_id = @company_id AND name = 'Demo Store 1'
);
INSERT INTO store (company_id, name)
SELECT @company_id, 'Demo Store 2'
WHERE NOT EXISTS (
  SELECT 1 FROM store WHERE company_id = @company_id AND name = 'Demo Store 2'
);

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

-- ---- demo dataset for e2e fixtures (idempotent) -------------------------
-- The Playwright suite needs at least one row in several tables to
-- exercise list/edit/search flows. Each block below is keyed on a stable
-- natural identifier so re-running the script never duplicates rows.
--
-- merchant_id in this codebase is the creator user.id (see
-- pkg/handlers/cash_vouchar.go: getMerchantID == GetSessionInfo.id), so
-- everything seeded here is owned by the admin demo account.
SET @admin_id    = (SELECT id FROM user  WHERE username = 'admin' LIMIT 1);
SET @store_id    = (SELECT id FROM store WHERE company_id = @company_id AND name = 'Demo Store 1' LIMIT 1);

-- 3 products in Demo Store 1 with realistic stock for stock-enforcement tests.
-- UNIQUE (article_id, store_id) makes the upsert idempotent.
INSERT INTO product (article_id, store_id, name, price, quantity, min_stock, cost_price)
VALUES
  (9001, @store_id, 'OEM Filter A',     45.00,  20.000, 5,  20.00),
  (9002, @store_id, 'Battery Pack B',  320.00,  10.000, 3, 220.00),
  (9003, @store_id, 'Spark Plug Set',   60.00,   5.000, 5,  25.00)
ON DUPLICATE KEY UPDATE
  name       = VALUES(name),
  price      = VALUES(price),
  quantity   = VALUES(quantity),
  min_stock  = VALUES(min_stock),
  cost_price = VALUES(cost_price);

-- 1 client (vat_number is NOT NULL; format check is on company.vat_number,
-- not client.vat_number). Keyed by vat_number so re-runs don't duplicate.
INSERT INTO client (name, company_name, phone, address, vat_number, country)
SELECT 'ACME Trading Co', 'ACME Trading Co', '0500000001',
       'Demo HQ, Riyadh', '300000000000099', 'SA'
WHERE NOT EXISTS (
  SELECT 1 FROM client WHERE vat_number = '300000000000099'
);
SET @client_id = (SELECT id FROM client WHERE vat_number = '300000000000099' LIMIT 1);

-- 1 supplier under Demo Co. Keyed by (company_id, name).
INSERT INTO supplier (company_id, name, phone_number, address, vat_number)
SELECT @company_id, 'Demo Supplier', '0500000002', 'Demo Warehouse, Riyadh',
       '300000000000088'
WHERE NOT EXISTS (
  SELECT 1 FROM supplier
   WHERE company_id = @company_id AND name = 'Demo Supplier'
);
SET @supplier_id = (
  SELECT id FROM supplier
   WHERE company_id = @company_id AND name = 'Demo Supplier'
   LIMIT 1
);

-- 1 company-mode draft invoice (state=0, client_id set).
-- Keyed by note='DEMO_SEED_INVOICE' so re-runs don't duplicate.
INSERT INTO bill (state, client_id, store_id, merchant_id, discount,
                  maintenance_cost, note, payment_method, userName)
SELECT 0, @client_id, @store_id, @admin_id, 0, 0,
       'DEMO_SEED_INVOICE', 10, 'Demo Admin'
WHERE NOT EXISTS (
  SELECT 1 FROM bill WHERE note = 'DEMO_SEED_INVOICE'
);
SET @bill_id = (SELECT id FROM bill WHERE note = 'DEMO_SEED_INVOICE' LIMIT 1);

-- 1 line item on the demo invoice (only when the line doesn't exist yet).
INSERT INTO bill_product (product_id, bill_id, vat, price, quantity, name)
SELECT p.id, @bill_id, 15.00, 45.00, 1.000, 'OEM Filter A'
FROM product p
WHERE p.article_id = 9001 AND p.store_id = @store_id
  AND NOT EXISTS (
    SELECT 1 FROM bill_product WHERE bill_id = @bill_id
  );

-- 1 draft purchase bill so /dashboard/purchase-bills/edit/{id} is reachable.
-- Keyed via the unique (supplier_id, supplier_sequence_number) constraint.
INSERT INTO purchase_bill (state, supplier_id, store_id, merchant_id,
                           discount, payment_method, supplier_sequence_number)
SELECT 0, @supplier_id, @store_id, @admin_id, 0, 10, 9001
WHERE @supplier_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM purchase_bill
     WHERE supplier_id = @supplier_id AND supplier_sequence_number = 9001
  );
SET @pb_id = (
  SELECT id FROM purchase_bill
   WHERE supplier_id = @supplier_id AND supplier_sequence_number = 9001
   LIMIT 1
);

INSERT INTO purchase_bill_product (product_id, bill_id, vat, price, quantity, name)
SELECT p.id, @pb_id, 15.00, 20.00, 5.000, 'OEM Filter A'
FROM product p
WHERE @pb_id IS NOT NULL
  AND p.article_id = 9001 AND p.store_id = @store_id
  AND NOT EXISTS (
    SELECT 1 FROM purchase_bill_product WHERE bill_id = @pb_id
  );

-- 1 cash voucher with a stable, searchable recipient_name.
-- voucher_number is per-merchant; 9001 is far above the auto-allocated
-- range, so it will not collide with normal app activity.
INSERT INTO cash_voucher (voucher_number, voucher_type, amount, payment_method,
                          state, recipient_type, recipient_name, description,
                          store_id, merchant_id, created_by)
SELECT 9001, 'disbursement', 100.00, 'cash', 0, 'other',
       'DEMO RECIPIENT QA', 'Seed voucher for qa-20 search needle',
       @store_id, @admin_id, @admin_id
WHERE NOT EXISTS (
  SELECT 1 FROM cash_voucher
   WHERE merchant_id = @admin_id AND recipient_name = 'DEMO RECIPIENT QA'
);
SQL

echo "[seed] demo users ensured: admin / manager / employee"
echo "[seed] demo dataset ensured: 3 products, 1 client, 1 supplier, 1 invoice, 1 purchase bill, 1 cash voucher"
