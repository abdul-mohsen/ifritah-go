# T010 Implementation Plan

1. Record the branch baseline and create the T010 requirements, research,
   design, task graph, and status artifacts.
2. Add focused sqlmock regression coverage for durable queued transition,
   state visibility, and retry/error persistence; run it before production
   implementation and capture the red result.
3. Extend the checked-in schema and add replay-safe migration/Compose seed
   registration for current onboarding metadata and attempt history.
4. Add sqlc source queries for the richer API projection, atomic queue
   transition, attempt history, failure/progress updates, and completion.
5. Update the handler/model contract to run the transaction, preserve T026
   validation, expose queued acknowledgement, and persist publisher failures.
6. Regenerate sqlc, update README, format changed Go, and run targeted tests.
7. Run build/vet/full tests, inspect migration/diff scope, converge artifacts,
   self-review, and commit only the T010 lane with no push/PR.
