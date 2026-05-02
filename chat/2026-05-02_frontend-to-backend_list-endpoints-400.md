# [FRONTEND → BACKEND] List endpoints return 400 for seeded demo users (admin/manager/employee) — blocks ~22 e2e tests

**Date:** 2026-05-02
**From:** afrita-go (frontend) · branch `fix/token-refresh-on-idle` · HEAD `50b7206`
**To:**   ifritah-go (backend) · branch `dev` · HEAD `2c3bfe0`
**Severity:** P1 — blocks dashboard `/invoices`, `/purchase-bills`, multi-branch flows for demo accounts in CI and any local dev not using legacy `ssda` user.
**Frontend ticket:** GitHub Action run [25231090142](https://github.com/abdul-mohsen/go_afrita/actions/runs/25231090142) — 66 of 248 e2e specs failing; ~22 trace back to this defect.

---

## TL;DR

After backend PR #21 aligned `seed-demo-users.sh` passwords with the frontend e2e (`admin/admin`, `manager/manager`, `employee/employee`), the seeded users **have no `company_id` and no `store` mapping**. As a result, every list endpoint that requires `store_ids` (`/api/v2/bill/all`, `/api/v2/purchase_bill/all`, very likely supplier-report exports too) returns **HTTP 400** for those users, and the frontend converts that into a 500-toast page.

We need **either** the seed script to assign demo users to a company with at least one store, **or** the API to default `store_ids` to the user's accessible stores when the field is omitted (or both — they're complementary).

---

## Reproduction

Direct backend probe (token from `/api/v2/login` with `admin/admin`):

```bash
TOKEN=$(curl -s -X POST https://dev.ifritah.com/api/v2/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | jq -r .access_token)

# 1. The user has zero accessible stores:
curl -s -H "Authorization: Bearer $TOKEN" https://dev.ifritah.com/api/v2/stores/all
# → null

# 2. So bill/all rejects the request:
curl -s -o /dev/null -w 'HTTP %{http_code}\n' \
  -X POST https://dev.ifritah.com/api/v2/bill/all \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"page_number":0,"page_size":10}'
# → HTTP 400

# 3. Same for purchase_bill/all:
curl -s -o /dev/null -w 'HTTP %{http_code}\n' \
  -X POST https://dev.ifritah.com/api/v2/purchase_bill/all \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"page_number":0,"page_size":10}'
# → HTTP 400

# 4. The legacy user works (38 stores):
TOKEN=$(curl -s -X POST .../login -d '{"username":"ssda","password":"Qwerty123"}' | jq -r .access_token)
curl -s -H "Authorization: Bearer $TOKEN" .../stores/all | jq 'length'
# → 38
```

So the difference is purely seed/data: `admin/manager/employee` have no company/stores; `ssda` does.

---

## Root cause walk-through

### `pkg/handlers/store.go:17`
```go
func (h *handler) getStores(user userSession) []Store {
    rows, err := h.DB.Query(`select store.id, addressId, store.name
        from store
        join company on store.company_id = company.id
        join user    on user.id = ? and company.id = user.company_id`, user.id)
    ...
}
```
With `user.company_id IS NULL`, the join produces zero rows ⇒ `getStores` returns `nil`.

### `pkg/handlers/bill.go:55`
```go
func (h *handler) GetBills(c *gin.Context) {
    userSession := GetSessionInfo(c)

    var storeIds []int
    for _, v := range h.getStores(userSession) {
        storeIds = append(storeIds, v.Id)
    }
    request := model.BillRequestFilter{
        StoreIds: storeIds, Page: 0, PageSize: 10,
    }
    if err := c.BindJSON(&request); err != nil { c.Status(400); return }

    if request.Page < 0 || request.PageSize <= 0 ||
       request.StoreIds == nil || len(request.StoreIds) == 0 {
        c.Status(400); return     // ← we land here for demo users
    }
    ...
}
```

### `scripts/seed-demo-users.sh:60`
```sql
INSERT INTO user (username, full_name, password, role, is_active) VALUES
  ('admin',    'Demo Admin',    '${ADMIN_HASH}',    'admin',    1),
  ('manager',  'Demo Manager',  '${MANAGER_HASH}',  'manager',  1),
  ('employee', 'Demo Employee', '${EMPLOYEE_HASH}', 'employee', 1)
```
No `company_id`, no `store_user` rows.

### Side note — `BindJSON` semantics
`c.BindJSON(&request)` does **not** preserve the pre-populated `StoreIds: storeIds` if the incoming JSON omits the key. `encoding/json` decodes a fresh value into the field that exists in the body; absent keys keep their current value, but in practice for a slice the decoded zero of an empty body is the original — however when the body is a non-empty `{}` without the key, the slice is in fact preserved. The 400 we see is from the explicit `len == 0` guard, not from JSON decoding. Worth a comment in the handler to remove the ambiguity.

---

## Recommended fixes (any one closes the bug; both is best)

### Fix 1 — Patch the seed script (smallest, ship today)

```diff
 INSERT INTO user (username, full_name, password, role, is_active) VALUES
   ('admin',    'Demo Admin',    '${ADMIN_HASH}',    'admin',    1),
   ('manager',  'Demo Manager',  '${MANAGER_HASH}',  'manager',  1),
   ('employee', 'Demo Employee', '${EMPLOYEE_HASH}', 'employee', 1)
 ON DUPLICATE KEY UPDATE
   full_name = VALUES(full_name),
   password  = VALUES(password),
   role      = VALUES(role),
   is_active = 1;

+-- ---- demo company + stores -------------------------------------------
+INSERT INTO company (name) VALUES ('Demo Co')
+  ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id);
+SET @company_id = LAST_INSERT_ID();
+
+UPDATE user SET company_id = @company_id
+  WHERE username IN ('admin','manager','employee');
+
+-- two stores so multi-branch tests have something to pick (qa-12).
+INSERT INTO store (company_id, name) VALUES
+  (@company_id, 'Demo Store 1'),
+  (@company_id, 'Demo Store 2')
+ON DUPLICATE KEY UPDATE name = VALUES(name);
```

### Fix 2 — Default `store_ids` on the server when omitted (recommended long-term)

Apply this pattern in `GetBills`, `GetAllPurchaseBill`, and any other list endpoint that today rejects empty `StoreIds`:

```diff
 var storeIds []int
 for _, v := range h.getStores(userSession) {
     storeIds = append(storeIds, v.Id)
 }
-request := model.BillRequestFilter{ StoreIds: storeIds, Page: 0, PageSize: 10 }
+request := model.BillRequestFilter{ Page: 0, PageSize: 10 }
 if err := c.BindJSON(&request); err != nil {
     log.Printf("GetBills: %v", err)
     c.Status(http.StatusBadRequest); return
 }
-if request.Page < 0 || request.PageSize <= 0 ||
-   request.StoreIds == nil || len(request.StoreIds) == 0 {
-    c.Status(http.StatusBadRequest); return
-}
+if len(request.StoreIds) == 0 {
+    // Caller did not narrow — default to every store the user can access.
+    request.StoreIds = storeIds
+}
+if request.Page < 0 || request.PageSize <= 0 {
+    c.Status(http.StatusBadRequest); return
+}
+if len(request.StoreIds) == 0 {
+    // Truly no accessible stores → empty result, not an error.
+    c.JSON(http.StatusOK, []any{})
+    return
+}
```
Rationale:
- `store_ids` is a **filter**, not a primary key. Optional filters that are absent should default to "no narrowing" (RFC 7231 §6.5.4 spirit).
- Returning `200 []` for "no data the user can see" is friendlier than `400` and lets the frontend render an empty list instead of a 500 toast.
- This also fixes any future user that legitimately has zero stores assigned (read-only auditor accounts, etc.).

### Fix 3 — supplier-account-report exports return 500

`/dashboard/suppliers/{id}/report/export-csv` and `/export-excel` return HTTP 500 for `admin/admin` (qa-29). Almost certainly the same store-scope guard in the export handler. Please:
1. Check `pkg/handlers/supplier_report.go` (or equivalent) for an analogous `store_ids` empty-list 400/500 path.
2. Apply the same default-to-user's-stores logic.
3. Confirm the response body emits valid CSV/Excel rather than panicking when the dataset is empty.

---

## Frontend mitigation already deployed

To avoid noisy 500 pages while this is being fixed, the frontend now treats a 400 from `/api/v2/bill/all` and `/api/v2/purchase_bill/all` as "empty list" (logs a warning) instead of an error toast. See `helpers/api_helpers.go` `FetchInvoicesAll` and `FetchPurchaseBillsAll`. This means the invoices/purchase-bills pages render an empty table for demo users until the backend fix lands.

This mitigation is intentionally narrow (only those two endpoints, only HTTP 400). It does **not** mask other backend errors.

---

## Tests blocked by this defect (e2e — Playwright)

| Spec | Failing tests |
|---|---|
| `qa-02-pages-render` | renders /dashboard/invoices, /dashboard/purchase-bills |
| `qa-05-list-pages` | invoices renders/search/state/pagination, purchase-bills renders/search/pagination |
| `qa-13-dates` | invoices list date range filter |
| `qa-18-rbac` | role=admin matrix, role=manager matrix |
| `qa-11-bill-form` | branch selects show 0 options |
| `qa-12-multibranch` | 3 store-list tests |
| `qa-30-purchase-bill-stock-price-labels` | cost/selling price labels |
| `qa-20-list-filters` | invoice dropdown selection (30 s timeout) |
| `qa-10-cash-voucher-flow` | create draft via form (30 s timeout) |
| `qa-16-voucher-state-transitions` | full round-trip, skipping approve |
| `qa-29-supplier-report-export` | CSV table, Excel/PDF sections |

All are expected to go green after Fix 1 + Fix 2 + Fix 3.

---

## Suggested PR order (backend side)

1. **PR A** — Patch `scripts/seed-demo-users.sh` (Fix 1). Re-run the seeder against `dev`. **Closes ~20 of the 22 failures** within minutes.
2. **PR B** — Make `store_ids` optional in `GetBills` and `GetAllPurchaseBill` (Fix 2).
3. **PR C** — Investigate and fix supplier report export 500 (Fix 3).
4. **PR D** — Audit other endpoints with the same `store_ids` guard (`/cashvoucher/all`, `/dashboardData`, etc.) and apply the same pattern.

Happy to pair on any of these or open the seed-script PR from this side if it helps. Ping me on this thread.

— Frontend team
