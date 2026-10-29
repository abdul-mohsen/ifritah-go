# T014 Validation Quickstart

Run from the T014 worktree:

```powershell
rtk sqlc generate
rtk gofmt -w pkg/handlers/pub.go pkg/handlers/zatca_outbox.go pkg/handlers/zatca_envelope_test.go
rtk go test ./pkg/handlers -run 'TestZatcaPublisher|TestZatcaOutboxEnvelope' -count=1
rtk go build ./...
rtk go vet ./...
rtk go test ./... -count=1
```

The focused publisher tests inspect captured JSON and NATS publish options.
They must show the versioned fields, legacy aliases, stable message identity,
outbox row ID, and attempt metadata. No live NATS, MySQL, or daemon is needed.
