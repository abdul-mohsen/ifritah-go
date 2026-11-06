package outbox

import (
	"context"
	"log"
	"time"
)

// Run drains the outbox until ctx is done. It drains once at start and then once per interval, so
// interval must be positive. A failed drain is logged and tried again at the next tick, so one
// database error does not stop the worker.
func (d *Drainer) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := d.DrainOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("outbox: drain failed, trying again in %s: %v", interval, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
