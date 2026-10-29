# T024: Durable Debit-Note Backend Contract

## Summary

Create the backend contract required for a debit event to have a durable,
tenant-safe source row, stable identity, parent invoice sequence and EGS
context, a transactional outbox event, and a pending/provider outcome visible
through the existing ZATCA monitor.

## Scope

The feature includes the debit-note table/migrations, sqlc queries, authenticated
`POST /api/v2/bill/debit` endpoint, transactional publisher integration,
generic submission source identity, additive event/monitor fields, focused
regressions, and contract documentation.

It explicitly excludes daemon debit XML/processing, import persistence, stock,
accounting, payments, frontend work, and live provider integration.

## Contract

An authenticated request for a tenant-owned parent bill atomically creates:

1. a debit source row with tenant, parent bill, branch, sequence, EGS, reason,
   and stable source identity;
2. a pending generic ZATCA submission keyed by document kind/source ID;
3. a versioned debit outbox event with stable key and sequence/EGS context.

The request publishes only after commit through the existing outbox drainer.
Monitor routes expose the debit document/source/context and explicit outcome
without weakening tenant predicates.

## Compatibility

Existing bill, credit, onboarding, outbox, envelope, monitor, JWT, and RBAC
contracts remain unchanged except for additive fields. Generated sqlc output is
pipeline output and is never hand-edited.

## Validation

The implementation must pass focused red/green regressions, sqlc generation,
gofmt, build, vet, targeted tests, and `go test ./...`. Live MySQL migration,
lock, and daemon/provider outcome validation remains an explicit operational
limitation.
