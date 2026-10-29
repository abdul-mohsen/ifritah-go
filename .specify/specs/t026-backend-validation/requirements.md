# T026 Backend M1 Validation Requirements

## Problem

The backend currently accepts malformed ZATCA onboarding input at the HTTP
boundary. In particular, an OTP with six non-ASCII/non-numeric characters can
reach the ZATCA publisher, and onboarding can be queued without the required
branch ZATCA configuration being present. This moves avoidable validation
failures into asynchronous processing and makes the request outcome
non-actionable for callers.

## Goals

- Reject invalid ZATCA onboarding input before any queue/publication call.
- Treat the trimmed OTP as the authoritative value and require exactly six
  ASCII digits.
- Reject onboarding when the authoritative persisted branch configuration
  lacks required VAT/address/CSR fields needed by the existing backend-to-ZATCA
  contract.
- Return actionable field-level validation details using the backend's
  established JSON `detail` error convention.
- Preserve existing authentication, tenant, branch, and admin-role boundaries.

## In scope

- `POST /api/v2/branch/:id/zatca/onboard` validation and its focused handler
  regression coverage.
- Validation of the writable `branch_zatca_config` fields that the existing
  onboarding worker consumes as authoritative input.
- Minimal test seams needed to prove that rejected requests are not published.
- Directly related feature documentation and validation evidence.

## Out of scope

- Changes to NATS/outbox semantics, message schema, ZATCA provider behavior,
  database schema, migrations, authentication middleware, or route roles.
- Changing the existing config-save endpoint's persistence contract unless
  required to make the onboarding boundary authoritative.
- Replacing generated sqlc code by hand.

## User stories

1. As an administrator onboarding a branch, I receive a 400 response naming
   each invalid or missing field instead of an opaque asynchronous failure.
2. As the ZATCA queue, I never receive an onboarding event with a malformed
   OTP or incomplete branch configuration.
3. As an existing authenticated tenant user, I retain the same route
   authentication, tenant isolation, and admin-only onboarding permissions.

## Functional requirements

### FR-1: OTP validation

- The handler MUST trim surrounding whitespace from `otp`.
- The trimmed value MUST contain exactly six bytes, each in ASCII `0` through
  `9`; Unicode numerals, letters, punctuation, and control characters MUST be
  rejected.
- The publisher MUST receive the trimmed OTP, never the untrimmed request
  value.
- Invalid OTP requests MUST return HTTP 400 with a `detail` that identifies
  `otp` and the required format.
- Invalid OTP requests MUST NOT call the publisher.

### FR-2: Authoritative branch/config validation

- Before publishing, the handler MUST load the branch's persisted
  `branch_zatca_config` values using the branch ID from the authenticated
  request context.
- Missing branch/config data MUST not be treated as valid defaults.
- Required fields MUST include the existing worker contract's authoritative
  CSR/VAT/address inputs: `csr_org_identifier`, `csr_org_unit`,
  `csr_org_name`, `csr_country`, `csr_location`, `business_category`,
  `seller_vat`, `seller_crn`, `street`, `building`, `district`, and
  `postal_code`.
- Values MUST be trimmed for validation; blank values MUST be reported by
  request field name.
- A request with any missing required field MUST return HTTP 400 with
  actionable field-level details and MUST NOT publish.

### FR-3: Boundary and error compatibility

- Existing JWT authentication, tenant/branch lookup behavior, and admin-only
  route registration MUST remain unchanged.
- Existing success and internal publisher failure behavior MUST remain
  compatible unless a validation response is required.
- Validation responses MUST use the existing `{"detail": ...}` convention and
  MUST NOT expose secrets or credentials.

## Non-functional requirements

- Validation MUST be deterministic and side-effect free before the publisher
  call.
- The change MUST not add a schema migration or alter generated SQL output
  manually.
- Tests MUST prove both rejection/no-publication and successful trimmed-OTP
  publication for a valid contract fixture.

## Acceptance criteria

- A baseline test run demonstrates the focused invalid-input case fails on
  commit `9bdb0d6` because invalid input can reach/attempt publication.
- After implementation, focused tests pass and demonstrate no publication for
  invalid OTP and incomplete config, plus trimmed OTP for valid input.
- `gofmt`, the targeted handler tests, `go test ./...`, and `go vet ./...`
  (or an explicitly documented applicable subset) pass.
- No outbox, schema, auth, tenant, role, or unrelated worktree changes are
  present.
- The final commit contains only T026 lane changes and the required
  co-author trailer.

## Assumptions, constraints, risks, and unknowns

- Assumption: the existing ZATCA worker treats the `branch_zatca_config`
  writable fields listed above as its authoritative CSR/taxpayer input; this
  will be verified against the worker contract during research.
- Assumption: `branch_zatca_config` has one row per branch and no row means
  onboarding is not configured.
- Constraint: generated sqlc sources are ignored in this repository; query
  changes, if any, must follow the repository's generation workflow.
- Constraint: no schema or outbox edits are expected.
- Risk: requiring a field that the deployed worker can safely fall back to
  could reject existing configurations; research must distinguish fallback
  fields from authoritative fields.
- Unknown: whether status preservation or a branch existence query is needed
  in the onboarding validation path; resolve from current handler and worker
  contracts before design.
