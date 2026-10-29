# T031 implementation plan

1. Add red lifecycle regressions against the clean `98ae215` baseline.
2. Add the import-lane migration/schema and sqlc query source.
3. Implement T030-compatible models, immutable evidence persistence, and
   tenant/version/status guarded authenticated APIs.
4. Regenerate sqlc, format, run focused and full validation, update README and
   STATUS, then converge/self-review.
5. Create one isolated local commit without push or PR.
