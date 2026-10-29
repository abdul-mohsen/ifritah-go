# ifritah-go

## Database contracts

The backend owns the ZATCA submission persistence contract. The durable
submission, provider outcome, attempt, and artifact schema is defined in
`pkg/db/schema/schema.sql` and the replay-safe migration
`pkg/db/migrations/0008_zatca_submission_state.sql`. Monitor SQL lives in
`pkg/db/queries/zatca_monitor.sql`.

Regenerate the ignored sqlc output before building or testing:

```powershell
rtk sqlc generate
```

Generated files under `pkg/db/gen` are pipeline output and must not be edited
by hand.

## ZATCA monitor

The admin-only monitor endpoints read tenant-scoped durable submission,
outcome, and attempt rows:

- `GET /api/v2/zatca/monitor/stats`
- `GET /api/v2/zatca/monitor/branches`
- `GET /api/v2/zatca/monitor/submissions`

Aggregate counts, pending age, retry state, and provider details are sourced
from those rows. Missing optional outcome or attempt evidence remains `null`;
the monitor does not create branch or submission values for rows that are not
durably present.