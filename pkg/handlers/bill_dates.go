package handlers

import (
	"fmt"
	"time"
)

// The ZATCA daemon takes a bill's issue date and time from its saved effective date. ParseDates in zatca_integration (processor/types.go,
// commit becfdd0) reads that text with ParseFlexible, which tries these layouts in order and keeps the first that reads the value. The
// daemon has no test for ParseDates or ParseFlexible, so the cases in bill_dates_test.go were written from this code.
var daemonDateLayouts = []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}

// dateLayout is the layout of a day, used to compare two days.
const dateLayout = "2006-01-02"

// parseDaemonDate is ParseFlexible of the ZATCA daemon (zatca_integration, processor/types.go, commit becfdd0). A value with no zone is
// read as UTC, which is the zone that the daemon reads a saved date in.
func parseDaemonDate(s string) (time.Time, error) {
	for _, layout := range daemonDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date format: %s", s)
}

// savedEffectiveDate is the text that the database holds for an effective date. The connection writes times in UTC: main calls the
// Connect in pkg/db/gen/config.go, which sets no location, so the driver's default, UTC, applies. A DATETIME keeps whole seconds and
// rounds the fraction. The daemon reads this text back, so the rules below read the same text.
func savedEffectiveDate(t time.Time) string {
	return t.UTC().Round(time.Second).Format("2006-01-02 15:04:05")
}

// dateIssues checks the saved effective date of a sale with the two date rules. date_invalid: the daemon cannot read the saved text, so
// it refuses the bill. date_in_future: the issue date is later than today, both in UTC. That a future issue date is not valid is our
// reading of the ZATCA guidelines, and ZATCA's own validator is not run here.
func dateIssues(saved string, now time.Time) []importIssue {
	issue, err := parseDaemonDate(saved)
	if err != nil {
		return []importIssue{{Code: importDateInvalid, Message: "the effective date cannot be read as a date and time"}}
	}
	if issue.UTC().Format(dateLayout) > now.UTC().Format(dateLayout) {
		return []importIssue{{Code: importDateInFuture, Message: "the effective date is later than today"}}
	}
	return nil
}
