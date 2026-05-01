# [FRONTEND → BACKEND] Seed canonical e2e test users on dev

**From:** afrita-go (frontend)
**To:** ifritah-go (backend)
**Date:** 2026-05-01
**Status:** OPEN — blocking E2E green
**Repo / PR:** https://github.com/abdul-mohsen/go_ifritah/pull/16

## Ask

The frontend Playwright suite (running against `https://dev.ifritah.com` from
the `E2E (against dev backend)` GitHub Action) needs the three canonical demo
accounts seeded on the dev DB. Today only `ssda / Qwerty123` exists, which
means:

- Default `login()` helper in `e2e/helpers/qa.js` cannot use `admin/admin*`.
- `qa-18-rbac.spec.js` cannot exercise the manager/employee negative paths.
- Any spec that assumes a non-admin role currently has to be skipped or
  forced to admin, which masks RBAC regressions.

`scripts/seed-demo-users.sh` already exists and is the right place — but
its current `bcrypt_hash` calls use the **username as the password**
(`admin/admin`, `manager/manager`, `employee/employee`). Please update it to
the canonical e2e fixture passwords below, then run it on dev.

### Required users

| Role     | Username   | Password (plaintext) | role/permission                          |
|----------|------------|----------------------|------------------------------------------|
| admin    | `admin`    | `admin123`           | full admin (same as `ssda`)              |
| manager  | `manager`  | `manager123`         | everything **except** `settings:*`        |
| employee | `employee` | `employee123`        | no resource permissions seeded (default) |

Concrete diff request for `scripts/seed-demo-users.sh`:

```diff
-ADMIN_HASH="$(bcrypt_hash admin)"
-MANAGER_HASH="$(bcrypt_hash manager)"
-EMPLOYEE_HASH="$(bcrypt_hash employee)"
+ADMIN_HASH="$(bcrypt_hash admin123)"
+MANAGER_HASH="$(bcrypt_hash manager123)"
+EMPLOYEE_HASH="$(bcrypt_hash employee123)"
```

These match the RBAC matrix already encoded in
`afrita-go/handlers/rbac.go` and the assertions in
`afrita-go/e2e/tests/qa-18-rbac.spec.js`:

```
/dashboard/settings           → admin only
/dashboard/branches           → admin + manager (NOT employee)
/dashboard/stores             → admin + manager (NOT employee)
/dashboard/cash-vouchers/*/approve|reject → admin + manager (NOT employee)
all other /dashboard/*        → admin + manager + employee
```

## Why these specific values

These are non-secret fixture credentials baked into the public test suite.
They are intended for **dev only** — please do not enable on staging or
prod. The frontend treats them as the default for both local runs and the
GitHub Actions e2e workflow (`.github/workflows/e2e.yml`).

## Acceptance

- `POST https://dev.ifritah.com/api/v2/login` with each of the three
  username/password pairs returns HTTP 200 + access/refresh tokens.
- Decoded access JWT has the expected role claim
  (`admin` / `manager` / `employee`).
- The CI run on PR #16 turns the qa-18 RBAC spec from "all admin" back to
  the real role matrix.

## Frontend side already in place

- `e2e/helpers/qa.js` `login()` defaults to `admin / admin123`.
- `e2e/helpers/auth.js` (used by zatca specs) uses `admin / admin123`.
- `e2e/tests/qa-18-rbac.spec.js` iterates the three accounts above.
- `.github/workflows/e2e.yml` runs against `https://dev.ifritah.com` with
  no extra env (no per-user secrets needed since these are dev-only).

Please reply with the seed commit/migration SHA so we can re-run the e2e
workflow and close out PR #16.
