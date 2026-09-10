package normalize

import (
	"testing"
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/config"
)

func TestRecordsFiltersCowIDsAndNormalizes(t *testing.T) {
	cfg := config.Config{
		TargetCowIDs: map[int]struct{}{3: {}},
	}
	receivedAt := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	payloads := []map[string]any{
		{
			"cow_id":          3,
			"observedAt":      "2026-09-04T09:59:58Z",
			"bodyTemperature": 39.4,
			"milkAlert":       true,
		},
		{
			"cow_id":          9,
			"bodyTemperature": 39.1,
		},
	}

	records, rejected := Records(cfg, payloads, receivedAt)
	if rejected != 1 {
		t.Fatalf("expected 1 rejected record, got %d", rejected)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 accepted record, got %d", len(records))
	}
	if records[0].CowID != 3 {
		t.Fatalf("expected cow id 3, got %d", records[0].CowID)
	}
	if !records[0].MilkAlert {
		t.Fatal("expected milk alert to be true")
	}
	if records[0].ObservedAt.Format(time.RFC3339) != "2026-09-04T09:59:58Z" {
		t.Fatalf("unexpected observed time %s", records[0].ObservedAt.Format(time.RFC3339))
	}
}

func TestRecordsExtractsCameraIDFromCompositeCowID(t *testing.T) {
	cfg := config.Config{AllowAllCowIDs: true}
	payloads := []map[string]any{{
		"cow_id":          "333_cam1_dummy",
		"bodyTemperature": 39.4,
	}}

	records, rejected := Records(cfg, payloads, time.Now().UTC())
	if rejected != 0 || len(records) != 1 {
		t.Fatalf("expected one accepted record, got %d accepted and %d rejected", len(records), rejected)
	}
	if records[0].CowID != 333 {
		t.Fatalf("expected cow id 333, got %d", records[0].CowID)
	}
	if records[0].CameraID != "cam1" {
		t.Fatalf("expected camera id cam1, got %q", records[0].CameraID)
	}
}

func TestRecordsLeavesCameraIDEmptyWhenAbsent(t *testing.T) {
	cfg := config.Config{AllowAllCowIDs: true}
	payloads := []map[string]any{{"cow_id": 333}}

	records, rejected := Records(cfg, payloads, time.Now().UTC())
	if rejected != 0 || len(records) != 1 {
		t.Fatalf("expected one accepted record, got %d accepted and %d rejected", len(records), rejected)
	}
	if records[0].CameraID != "" {
		t.Fatalf("expected empty camera id, got %q", records[0].CameraID)
	}
}
