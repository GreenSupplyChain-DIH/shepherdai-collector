package models

import "time"

type TelemetryRecord struct {
	ObservedAt         time.Time
	CowID              int
	CameraID           string
	BodyTemperature    *float64
	MilkYield          *float64
	MilkReductionRatio *float64
	ElevatedTempAlert  bool
	MilkAlert          bool
	HealthAlert        string
	ReceivedAt         time.Time
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
