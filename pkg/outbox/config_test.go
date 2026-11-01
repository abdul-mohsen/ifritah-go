package outbox

import (
	"testing"
	"time"
)

func TestDrainerFromEnvUsesTheDefaults(t *testing.T) {
	for _, key := range []string{"OUTBOX_INTERVAL", "OUTBOX_BATCH_SIZE", "OUTBOX_LEASE", "OUTBOX_MAX_ATTEMPTS"} {
		t.Setenv(key, "")
	}
	d, interval, err := DrainerFromEnv(nil, nil, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if interval != 500*time.Millisecond || d.BatchSize != 50 || d.Lease != time.Minute || d.MaxAttempts != 10 || d.Owner != "worker-1" {
		t.Fatalf("defaults: every %s, batch %d, lease %s, attempts %d, owner %q; want 500ms, 50, 1m, 10, worker-1",
			interval, d.BatchSize, d.Lease, d.MaxAttempts, d.Owner)
	}
}

func TestDrainerFromEnvReadsTheOverrides(t *testing.T) {
	t.Setenv("OUTBOX_INTERVAL", "2s")
	t.Setenv("OUTBOX_BATCH_SIZE", "25")
	t.Setenv("OUTBOX_LEASE", "90s")
	t.Setenv("OUTBOX_MAX_ATTEMPTS", "4")
	d, interval, err := DrainerFromEnv(nil, nil, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if interval != 2*time.Second || d.BatchSize != 25 || d.Lease != 90*time.Second || d.MaxAttempts != 4 {
		t.Fatalf("overrides: every %s, batch %d, lease %s, attempts %d", interval, d.BatchSize, d.Lease, d.MaxAttempts)
	}
}

func TestDrainerFromEnvRefusesBadSettings(t *testing.T) {
	cases := map[string]struct {
		key, value, owner string
	}{
		"empty owner":         {owner: ""},
		"owner too long":      {owner: string(make([]byte, 129))},
		"zero interval":       {key: "OUTBOX_INTERVAL", value: "0s", owner: "w"},
		"negative lease":      {key: "OUTBOX_LEASE", value: "-1s", owner: "w"},
		"batch of zero":       {key: "OUTBOX_BATCH_SIZE", value: "0", owner: "w"},
		"batch too large":     {key: "OUTBOX_BATCH_SIZE", value: "1001", owner: "w"},
		"attempts too large":  {key: "OUTBOX_MAX_ATTEMPTS", value: "1001", owner: "w"},
		"unparsable interval": {key: "OUTBOX_INTERVAL", value: "soon", owner: "w"},
		"unparsable batch":    {key: "OUTBOX_BATCH_SIZE", value: "many", owner: "w"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.key != "" {
				t.Setenv(tc.key, tc.value)
			}
			if _, _, err := DrainerFromEnv(nil, nil, tc.owner); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}
