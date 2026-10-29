# T026 Implementation Plan

1. Establish the baseline and record the T026 requirements/research/design
   artifacts.
2. Add the focused handler regression test and run it against commit `9bdb0d6`
   to capture the red behavior before production edits.
3. Add pure OTP/config validators and a narrow publisher interface seam.
4. Gate `OnboardBranchZatca` on normalized OTP and authoritative persisted
   config, preserving existing route and error conventions.
5. Update the backend README with the boundary contract.
6. Run gofmt, targeted tests, `go test ./...`, and `go vet ./...`; review the
   diff and converge the artifacts.
