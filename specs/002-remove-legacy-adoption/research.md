# Research: Remove Legacy Database Adoption

## Repository Evidence

| Question | Evidence | Decision |
|---|---|---|
| Where is adoption implemented? | `pkg/db/goose/adopt.go` defines `Adopt`, checks the Goose version table, compares schema digests, and records versions 0 and 1. | Delete the file; no replacement is needed under the fresh-system-only policy. |
| Which helper code exists only for adoption? | `pkg/db/goose/schema_digest.go` embeds and parses `baseline.digests`, queries `information_schema`, and exposes `SchemaDigests`, `BaselineDigests`, `FormatDigests`, and `CompareDigests`. | Delete the helper and `pkg/db/goose/baseline.digests`. |
| Which tests validate adoption? | `pkg/db/goose/adopt_test.go` contains baseline digest parity/regeneration tests and four adoption behavior tests. | Delete the adoption-only test file; retain the general migration and tenant-safety tests. |
| How is the CLI exposed? | `cmd/migrate/main.go` documents, dispatches, implements, and prints usage for `adopt`. | Remove the command case, helper, documentation line, and usage token; remove the now-unused `database/sql` import if no other code needs it. |
| What migration numbering must remain? | `pkg/db/goose/migrations_test.go` reads embedded `*.sql` files and requires contiguous versions starting at `BaselineVersion`; the directory contains `00001` through `00010`. | Do not rename or remove any migration SQL. Keep `BaselineVersion = 1`. |
| What tenant safety must remain? | `pkg/db/goose/helpers_test.go` creates isolated databases; `.github/workflows/migrations.yml` replays the baseline as a tenant account with `ALL` only on its own database. | Preserve `helpers_test.go`, the workflow replay step, and `DEFINER=CURRENT_USER` SQL behavior. |
| What operator docs are stale? | `pkg/db/goose/README.md` describes `migrate adopt`, pre-Goose databases, and adoption CI coverage. `00001_baseline.sql` and `migrations.go` mention existing databases being adopted. | Rewrite only the stale adoption statements; retain baseline provenance and rollback safety wording. |
| Are there unrelated digest references? | `Dockerfile`, `.github/workflows/deploy.yml`, `.github/scripts/validate-build-contract.sh`, and `pkg/buildinfo` use image/build digests. | Leave unrelated digest references unchanged. |

## History and Compatibility

- Commit `38d4f00` introduced the adoption implementation, digest file, tests,
  and CLI/docs support. The current issue explicitly reverses that capability.
- The current branch is clean before implementation, so all resulting changes
  can be attributed to issue #123 and the required Spec Kit artifacts.
- Goose uses the embedded SQL files as the migration source. Removing the
  adoption helper does not alter the SQL files, the provider, or the version
  table behavior for fresh databases.

## Resolved Unknowns

1. **Should the baseline version constant be removed?** No. It is also the
   rollback boundary used by `down` and `roundtrip`, and migration numbering
   tests depend on it. It remains at `1`, with comments updated to describe the
   fresh baseline rather than adoption.
2. **Should all references to “legacy” be removed?** No. The baseline comment
   accurately documents that the schema was squashed from historical schema
   sources, and unrelated CI comments identify supported MySQL compatibility.
   Only statements that promise pre-Goose adoption are removed or reworded.
3. **Should a replacement upgrade command be added?** No. The requirement is
   fresh-system-only and explicitly excludes another pre-Goose compatibility
   mechanism.
4. **Should tests be added for absence of the command?** No new runtime test is
   needed. Removing the switch branch and usage token is directly verified by
   compilation plus repository search; existing migration tests cover the
   behavior that must remain.

## Research Conclusion

The smallest complete change is a deletion of the adoption implementation,
digest helper/data, and adoption-only tests; a surgical CLI cleanup; and
documentation/comment cleanup. No migration SQL, schema, dependency, tenant
privilege, or Goose configuration change is required.
