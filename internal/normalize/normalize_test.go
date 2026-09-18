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

func TestRecordsNormalizesNGSILDPropertiesAndTimestamps(t *testing.T) {
	cfg := config.Config{AllowAllCowIDs: true}
	receivedAt := time.Date(2026, 9, 15, 2, 0, 0, 0, time.UTC)
	payloads := []map[string]any{{
		"s4agri:animalId": map[string]any{"type": "Property", "value": "93_cam0_dummy_id"},
		"shepherd:BodyTemperature": map[string]any{
			"type": "Property", "value": 38.2, "observedAt": "2026-09-15T01:56:14Z",
		},
		"s4agri:MilkYield": map[string]any{"type": "Property", "value": nil, "observedAt": nil},
		"shepherd:elevatedBodyTemperatureAlert": map[string]any{
			"type": "Property", "value": false, "observedAt": "2026-09-15T01:56:14Z",
		},
		"shepherd:healthAlert": map[string]any{
			"type": "Property", "value": "NONE", "observedAt": "2026-09-15T00:00:00Z",
		},
	}}

	records, rejected := Records(cfg, payloads, receivedAt)
	if rejected != 0 || len(records) != 1 {
		t.Fatalf("expected one accepted record, got %d accepted and %d rejected", len(records), rejected)
	}
	record := records[0]
	if record.CowID != 93 || record.CameraID != "cam0" {
		t.Fatalf("unexpected cow/camera: %d/%q", record.CowID, record.CameraID)
	}
	if record.BodyTemperature == nil || *record.BodyTemperature != 38.2 {
		t.Fatalf("unexpected body temperature: %v", record.BodyTemperature)
	}
	if record.BodyTemperatureObservedAt == nil || record.BodyTemperatureObservedAt.Format(time.RFC3339) != "2026-09-15T01:56:14Z" {
		t.Fatalf("unexpected body temperature timestamp: %v", record.BodyTemperatureObservedAt)
	}
	if record.HealthAlertObservedAt == nil || record.HealthAlertObservedAt.Format(time.RFC3339) != "2026-09-15T00:00:00Z" {
		t.Fatalf("unexpected health alert timestamp: %v", record.HealthAlertObservedAt)
	}
	if record.MilkYield != nil || record.MilkYieldObservedAt != nil {
		t.Fatalf("expected missing milk yield and timestamp, got %v/%v", record.MilkYield, record.MilkYieldObservedAt)
	}
	if record.ObservedAt.Format(time.RFC3339) != "2026-09-15T01:56:14Z" {
		t.Fatalf("unexpected record timestamp %s", record.ObservedAt.Format(time.RFC3339))
	}
}

func TestRecordsPreservesOffsetTimestamps(t *testing.T) {
	cfg := config.Config{AllowAllCowIDs: true}
	receivedAt := time.Date(2026, 9, 18, 15, 0, 0, 0, time.FixedZone("EEST", 3*60*60))
	payloads := []map[string]any{{
		"cow_id": "101_cam2_dummy",
		"shepherd:BodyTemperature": map[string]any{
			"type": "Property", "value": 39.0, "observedAt": "2026-09-18T13:45:00+03:00",
		},
		"shepherd:healthAlert": map[string]any{
			"type": "Property", "value": "NONE", "observedAt": "2026-09-18T13:40:00+03:00",
		},
	}}

	records, rejected := Records(cfg, payloads, receivedAt)
	if rejected != 0 || len(records) != 1 {
		t.Fatalf("expected one accepted record, got %d accepted and %d rejected", len(records), rejected)
	}
	record := records[0]
	if record.ObservedAt.Format(time.RFC3339) != "2026-09-18T13:45:00+03:00" {
		t.Fatalf("unexpected record timestamp %s", record.ObservedAt.Format(time.RFC3339))
	}
	if record.BodyTemperatureObservedAt == nil || record.BodyTemperatureObservedAt.Format(time.RFC3339) != "2026-09-18T13:45:00+03:00" {
		t.Fatalf("unexpected body temperature observedAt: %v", record.BodyTemperatureObservedAt)
	}
	if record.HealthAlertObservedAt == nil || record.HealthAlertObservedAt.Format(time.RFC3339) != "2026-09-18T13:40:00+03:00" {
		t.Fatalf("unexpected health alert observedAt: %v", record.HealthAlertObservedAt)
	}
	if record.ReceivedAt.Format(time.RFC3339) != "2026-09-18T15:00:00+03:00" {
		t.Fatalf("unexpected receivedAt %s", record.ReceivedAt.Format(time.RFC3339))
	}
}

func TestRecordsFallsBackToReceivedAtWithoutTimezoneRewrite(t *testing.T) {
	cfg := config.Config{AllowAllCowIDs: true}
	receivedAt := time.Date(2026, 9, 18, 16, 5, 0, 0, time.FixedZone("EEST", 3*60*60))
	payloads := []map[string]any{{
		"cow_id":          222,
		"bodyTemperature": 38.7,
	}}

	records, rejected := Records(cfg, payloads, receivedAt)
	if rejected != 0 || len(records) != 1 {
		t.Fatalf("expected one accepted record, got %d accepted and %d rejected", len(records), rejected)
	}
	record := records[0]
	if !record.ObservedAt.Equal(receivedAt) {
		t.Fatalf("expected observedAt to equal receivedAt, got %s vs %s", record.ObservedAt.Format(time.RFC3339), receivedAt.Format(time.RFC3339))
	}
	if record.ObservedAt.Format(time.RFC3339) != "2026-09-18T16:05:00+03:00" {
		t.Fatalf("unexpected observedAt %s", record.ObservedAt.Format(time.RFC3339))
	}
	if record.ReceivedAt.Format(time.RFC3339) != "2026-09-18T16:05:00+03:00" {
		t.Fatalf("unexpected receivedAt %s", record.ReceivedAt.Format(time.RFC3339))
	}
}
