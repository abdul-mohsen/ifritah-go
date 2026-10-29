# ZATCA Monitor Contract

Existing routes remain unchanged:

- `GET /api/v2/zatca/monitor/stats`
- `GET /api/v2/zatca/monitor/branches`
- `GET /api/v2/zatca/monitor/submissions`

The monitor response retains existing fields and adds durable persistence
fields where available:

- explicit `state` (`pending`, `accepted`, `accepted_with_warning`,
  `rejected`, `ambiguous`, `terminal_failure`);
- stable `document_id`;
- provider code/reference and warning code/message;
- retry count, last/next retry timestamps;
- request/cleared XML references;
- provider outcome timestamp.

Rows are always scoped to the authenticated tenant. Missing provider values
remain null; legacy rows without an explicit outcome are not assigned a
provider code or warning.
