package main

import (
	"testing"
	"time"
)

func TestParseBackfillDates(t *testing.T) {
	from, to, err := parseBackfillDates([]string{"--from", "2026-09-01", "--to", "2026-09-03"})
	if err != nil {
		t.Fatalf("parseBackfillDates returned error: %v", err)
	}
	if from != time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected from date: %s", from)
	}
	if to != time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected to date: %s", to)
	}
}

func TestParseBackfillDatesRejectsInvalidRanges(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "missing to", args: []string{"--from", "2026-09-01"}},
		{name: "invalid from", args: []string{"--from", "2026-09-31", "--to", "2026-10-01"}},
		{name: "inverted range", args: []string{"--from", "2026-09-03", "--to", "2026-09-01"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, _, err := parseBackfillDates(testCase.args); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
