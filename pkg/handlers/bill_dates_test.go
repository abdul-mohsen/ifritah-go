package handlers

import (
	"strings"
	"testing"
	"time"
)

// daemonDateCases are the inputs of ParseFlexible, each with the value it gives in RFC 3339 with nanoseconds, or "" when it refuses the
// input. The daemon has no test for these at commit becfdd0 (zatca_integration, processor/types.go), so the cases were written from that
// function. The backend and the daemon must read these inputs the same way.
var daemonDateCases = []struct{ in, want string }{
	{"2026-11-03T10:00:00+03:00", "2026-11-03T10:00:00+03:00"},
	{"2026-11-03T10:00:00Z", "2026-11-03T10:00:00Z"},
	{"2026-11-03T10:00:00.25Z", "2026-11-03T10:00:00.25Z"},
	{"2026-11-03 10:00:00", "2026-11-03T10:00:00Z"},
	{"2026-11-03", "2026-11-03T00:00:00Z"},
	{"2026-11-03T10:00:00", ""},
	{"2026-11-03 10:00", ""},
	{"2026-11-03 10:00:00+03:00", ""},
	{"2026-02-30", ""},
	{"2026-13-01", ""},
	{"03/11/2026", ""},
	{"", ""},
}

func TestParseDaemonDateReadsTheDaemonsInputs(t *testing.T) {
	for _, tc := range daemonDateCases {
		got, err := parseDaemonDate(tc.in)
		switch {
		case tc.want == "" && err == nil:
			t.Errorf("parseDaemonDate(%q) = %s; want a refusal", tc.in, got.Format(time.RFC3339Nano))
		case tc.want != "" && err != nil:
			t.Errorf("parseDaemonDate(%q) = %v; want %s", tc.in, err, tc.want)
		case tc.want != "" && got.Format(time.RFC3339Nano) != tc.want:
			t.Errorf("parseDaemonDate(%q) = %s; want %s", tc.in, got.Format(time.RFC3339Nano), tc.want)
		}
	}
}

// The saved text is the UTC wall clock to the second. A fraction of half a second or more rounds up, as the database rounds it.
func TestSavedEffectiveDateIsTheUTCWallClockToTheSecond(t *testing.T) {
	cases := map[string]string{
		"2026-11-03T10:00:00+03:00":   "2026-11-03 07:00:00",
		"2026-11-03T10:00:00.6+03:00": "2026-11-03 07:00:01",
		"2026-11-03T23:59:59.5Z":      "2026-11-04 00:00:00",
	}
	for in, want := range cases {
		parsed, err := time.Parse(time.RFC3339Nano, in)
		if err != nil {
			t.Fatal(err)
		}
		if got := savedEffectiveDate(parsed); got != want {
			t.Errorf("savedEffectiveDate(%s) = %q; want %q", in, got, want)
		}
	}
}

// A sale dated today is not in the future, at any time of the day.
func TestSaleDatedTodayIsNotInTheFuture(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 3, 23, 59, 59, 0, time.UTC),
	} {
		if issues := dateIssues(savedEffectiveDate(now), now); len(issues) != 0 {
			t.Errorf("a sale saved at %s = %+v; want no date issue", now, issues)
		}
	}
}

// date_in_future is an issue date later than today. The rule is in UTC: at 02:30 on 4 November in Riyadh, a sale dated 4 November is
// still in the future here, because the saved text is UTC.
func TestDateInFutureIsAnIssueDateLaterThanTodayInUTC(t *testing.T) {
	now := time.Date(2026, 11, 3, 23, 30, 0, 0, time.UTC)
	cases := map[string]bool{
		"2026-11-03 23:59:59": false,
		"2026-11-02 12:00:00": false,
		"2026-11-04 00:00:00": true,
		"2027-01-01 00:00:00": true,
	}
	for saved, want := range cases {
		if got := hasImportCode(dateIssues(saved, now), importDateInFuture); got != want {
			t.Errorf("dateIssues(%q) has date_in_future = %v; want %v", saved, got, want)
		}
	}
}

// date_invalid is a saved text that the daemon cannot read. The sale path saves only RFC 3339 times, which the daemon reads, so this
// tests the rule on the text itself.
func TestDateInvalidIsASavedTextTheDaemonCannotRead(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	for _, saved := range []string{"03/11/2026", "2026-11-03T10:00:00", "2026-13-01 00:00:00", ""} {
		if !hasImportCode(dateIssues(saved, now), importDateInvalid) {
			t.Errorf("dateIssues(%q) = %+v; want date_invalid", saved, dateIssues(saved, now))
		}
	}
	for _, saved := range []string{"2026-11-03 10:00:00", "2026-11-03"} {
		if hasImportCode(dateIssues(saved, now), importDateInvalid) {
			t.Errorf("dateIssues(%q) has date_invalid; want the daemon's layouts to read it", saved)
		}
	}
}

// withEffectiveDate is a sale body with an effective date added.
func withEffectiveDate(body, date string) string {
	return strings.Replace(body, `"store_id":1,`, `"store_id":1,"effective_date":"`+date+`",`, 1)
}

// dayAtNoonUTC is the day of t at 12:00 UTC, as RFC 3339. Noon is far from midnight, so the day that the check uses does not change
// during a test.
func dayAtNoonUTC(t time.Time) string {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
}
