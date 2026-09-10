package alerts

import (
	"testing"
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/models"
)

func TestActiveEventsCreatesOneEventPerAlertType(t *testing.T) {
	temperature := 39.8
	yield := 18.5
	reduction := 0.25
	records := []models.TelemetryRecord{{
		ObservedAt:         time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
		CowID:              333,
		CameraID:           "cam1",
		BodyTemperature:    &temperature,
		MilkYield:          &yield,
		MilkReductionRatio: &reduction,
		ElevatedTempAlert:  true,
		MilkAlert:          true,
		HealthAlert:        "WARNING",
	}}

	events := ActiveEvents(records)
	if len(events) != 3 {
		t.Fatalf("expected three alert events, got %d", len(events))
	}

	for index, alertType := range []string{"MILK", "HEALTH", "ELEVATED_BODY"} {
		event := events[index]
		if event.AlertType != alertType {
			t.Errorf("event %d: expected type %q, got %q", index, alertType, event.AlertType)
		}
		if event.CowID != 333 || event.CameraID != "cam1" {
			t.Errorf("event %d: unexpected cow/camera: %d/%q", index, event.CowID, event.CameraID)
		}
		if event.StartTime != records[0].ObservedAt {
			t.Errorf("event %d: unexpected start time %s", index, event.StartTime)
		}
		switch alertType {
		case "MILK":
			if event.MilkYield != &yield || event.MilkReductionRatio != &reduction {
				t.Errorf("event %d: expected milk metrics", index)
			}
		case "ELEVATED_BODY":
			if event.PeakTemperature != &temperature {
				t.Errorf("event %d: expected temperature metric", index)
			}
		case "HEALTH":
			if event.PeakTemperature != nil || event.MilkYield != nil || event.MilkReductionRatio != nil {
				t.Errorf("event %d: unexpected metrics for health alert", index)
			}
		}
	}
}
