package alerts

import (
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/models"
)

func ActiveEvents(records []models.TelemetryRecord) []models.AlertEvent {
	events := make([]models.AlertEvent, 0)
	for _, record := range records {
		if record.MilkAlert {
			events = append(events, newEvent(record, "MILK"))
		}
		if record.HealthAlert != "NONE" {
			events = append(events, newEvent(record, "HEALTH"))
		}
		if record.ElevatedTempAlert {
			events = append(events, newEvent(record, "ELEVATED_BODY"))
		}
	}
	return events
}

func newEvent(record models.TelemetryRecord, alertType string) models.AlertEvent {
	event := models.AlertEvent{
		CowID:     record.CowID,
		CameraID:  record.CameraID,
		AlertType: alertType,
		StartTime: alertObservedAt(record, alertType),
		Resolved:  false,
	}
	switch alertType {
	case "MILK":
		event.MilkYield = record.MilkYield
		event.MilkReductionRatio = record.MilkReductionRatio
	case "ELEVATED_BODY":
		event.PeakTemperature = record.BodyTemperature
	}
	return event
}

func alertObservedAt(record models.TelemetryRecord, alertType string) time.Time {
	var observedAt *time.Time
	switch alertType {
	case "MILK":
		observedAt = record.MilkAlertObservedAt
	case "HEALTH":
		observedAt = record.HealthAlertObservedAt
	case "ELEVATED_BODY":
		observedAt = record.ElevatedTempAlertObservedAt
	}
	if observedAt == nil {
		return record.ObservedAt
	}
	return *observedAt
}
