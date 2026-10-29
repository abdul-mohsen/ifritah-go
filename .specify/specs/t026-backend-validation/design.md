# T026 Design

## Boundary flow

```text
authenticated admin request
        |
        v
parse branch id
        |
        v
trim + validate OTP (no DB/publisher side effects)
        |
        v
GetZatcaBranchConfig(branch id)
        |
        +-- no row --> 400 detail: config is required
        +-- DB error --> 500 existing failure convention
        |
        v
validate 12 persisted config fields and established formats
        |
        +-- invalid --> 400 detail with deterministic field:reason entries
        |
        v
publish onboarding event with trimmed OTP
```

## Components

- `pkg/handlers/zatca_validation.go`
  - Pure OTP and config validation helpers.
  - Ordered field checks for deterministic, actionable errors.
  - No database, publisher, or HTTP side effects.
- `pkg/handlers/handler.go`
  - Replace the concrete internal publisher field with a narrow interface while
    keeping `New` source-compatible for production callers.
- `pkg/handlers/branch.go`
  - Validate OTP first.
  - Load the existing generated config query.
  - Return 400/500 as appropriate before calling the publisher.
  - Pass the normalized OTP to the existing compatibility publisher method.
- `pkg/handlers/zatca_validation_test.go`
  - sqlmock-backed handler regression tests with a recording publisher.

## Validation contract

### OTP

- Trim with `strings.TrimSpace`.
- Require six bytes.
- Require every byte to be between ASCII `0` and `9`.
- Return field `otp` and reason `must be exactly 6 ASCII digits`.

### Config

Validate, in request/JSON field order:

1. `csr_org_identifier` — nonblank.
2. `csr_org_unit` — nonblank.
3. `csr_org_name` — nonblank.
4. `csr_country` — nonblank.
5. `csr_location` — four uppercase ASCII letters followed by four ASCII digits.
6. `business_category` — nonblank.
7. `seller_vat` — exactly 15 ASCII digits, starting and ending with `3`.
8. `seller_crn` — exactly 10 ASCII digits, starting with `7`.
9. `street` — nonblank.
10. `building` — nonblank.
11. `district` — nonblank.
12. `postal_code` — nonblank.

All values are trimmed for validation. The normalized values are used only for
validation; this lane does not rewrite persisted config.

## Error handling

- 400: `{"detail":"invalid ZATCA onboarding request: <field> <reason>; ..."}`.
- Missing config row: same 400 shape with `zatca_config is required`.
- Non-`sql.ErrNoRows` config read error: log the internal error and return the
  existing generic `failed to load ZATCA config` 500 detail.
- Publisher failure: preserve the existing empty 500 response behavior.
- No credentials, OTP, or database error text is included in client responses.

## Security and compatibility

- Route registration, JWT middleware, admin middleware, branch ID parsing, and
  tenant database selection are unchanged.
- Existing `ZatcaPublisher.OnboadBranch` remains the concrete implementation;
  only the handler's internal dependency is narrowed for testability.
- No outbox, NATS message, generated SQL, or schema changes are required.

## Test strategy

- Red baseline: add a handler test for a six-character non-ASCII-digit OTP and
  verify the current implementation reaches/panics at publication instead of
  returning 400; run it before production changes.
- Green focused tests:
  - malformed OTP returns 400 and publisher call count remains zero;
  - incomplete config returns all actionable missing fields and publisher call
    count remains zero;
  - valid config with padded OTP publishes exactly the trimmed OTP.
- Run formatting, targeted handler tests, full Go tests, and `go vet ./...`.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-1 | Pure OTP validator, first handler gate, recording publisher test |
| FR-2 | Existing generated config query and ordered config validator |
| FR-3 | No route/middleware/publisher contract changes; `detail` string errors |
| NFR side-effect free | Validation precedes config publication and uses pure helpers |
| NFR generated SQL/schema | Reuse existing sqlc query; no SQL/schema edits |
