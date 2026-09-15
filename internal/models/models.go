package models

import "time"

type TelemetryRecord struct {
	ObservedAt                   time.Time
	CowID                        int
	CameraID                     string
	BodyTemperature              *float64
	BodyTemperatureObservedAt    *time.Time
	MilkYield                    *float64
	MilkYieldObservedAt          *time.Time
	MilkReductionRatio           *float64
	MilkReductionRatioObservedAt *time.Time
	ElevatedTempAlert            bool
	ElevatedTempAlertObservedAt  *time.Time
	MilkAlert                    bool
	MilkAlertObservedAt          *time.Time
	HealthAlert                  string
	HealthAlertObservedAt        *time.Time
	ReceivedAt                   time.Time
}

type AlertEvent struct {
	CowID              int
	CameraID           string
	AlertType          string
	StartTime          time.Time
	EndTime            *time.Time
	PeakTemperature    *float64
	MilkYield          *float64
	MilkReductionRatio *float64
	Resolved           bool
}
