# T026 Backend M1 Validation Specification

The backend must reject malformed ZATCA onboarding requests at the
authenticated admin HTTP boundary. OTP input is trimmed and must be exactly
six ASCII digits. Before the existing onboarding publisher is called, the
backend must load the branch's persisted ZATCA configuration and require the
existing 12-field contract, including valid Saudi VAT/CRN and CSR location
formats. Rejections are HTTP 400 responses with actionable field names and
reasons in the existing `detail` envelope. Successful requests publish only
the trimmed OTP. Authentication, tenant, role, generated SQL, message, outbox,
and schema behavior remain unchanged.

See `requirements.md`, `research.md`, `design.md`, and `plan.md` for the
traceability and validation plan.
