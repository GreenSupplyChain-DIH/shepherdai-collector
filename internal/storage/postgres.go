package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/config"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/models"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/normalize"
)

type Repository struct {
	pool           *pgxpool.Pool
	telemetryTable string
	alertsTable    string
}

func NewRepository(ctx context.Context, cfg config.Config) (*Repository, error) {
	if err := normalize.ValidateTableNames(cfg.SchemaName, cfg.TelemetryTable); err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &Repository{
		pool:           pool,
		telemetryTable: qualifiedName(cfg.SchemaName, cfg.TelemetryTable),
		alertsTable:    qualifiedName(cfg.SchemaName, cfg.AlertsTable),
	}, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) UpsertTelemetry(ctx context.Context, records []models.TelemetryRecord) error {
	if len(records) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	query := fmt.Sprintf(`
INSERT INTO %s (
    time,
    cow_id,
	camera_id,
    body_temperature,
	body_temperature_observed_at,
    milk_yield,
	milk_yield_observed_at,
    milk_reduction_ratio,
	milk_reduction_ratio_observed_at,
    elevated_temp_alert,
	elevated_temp_alert_observed_at,
    milk_alert,
	milk_alert_observed_at,
    health_alert,
	health_alert_observed_at,
	received_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
ON CONFLICT (time, cow_id) DO UPDATE SET
	camera_id = EXCLUDED.camera_id,
    body_temperature = COALESCE(EXCLUDED.body_temperature, %s.body_temperature),
	body_temperature_observed_at = COALESCE(EXCLUDED.body_temperature_observed_at, %s.body_temperature_observed_at),
    milk_yield = COALESCE(EXCLUDED.milk_yield, %s.milk_yield),
	milk_yield_observed_at = COALESCE(EXCLUDED.milk_yield_observed_at, %s.milk_yield_observed_at),
    milk_reduction_ratio = COALESCE(EXCLUDED.milk_reduction_ratio, %s.milk_reduction_ratio),
	milk_reduction_ratio_observed_at = COALESCE(EXCLUDED.milk_reduction_ratio_observed_at, %s.milk_reduction_ratio_observed_at),
    elevated_temp_alert = EXCLUDED.elevated_temp_alert,
	elevated_temp_alert_observed_at = COALESCE(EXCLUDED.elevated_temp_alert_observed_at, %s.elevated_temp_alert_observed_at),
    milk_alert = EXCLUDED.milk_alert,
	milk_alert_observed_at = COALESCE(EXCLUDED.milk_alert_observed_at, %s.milk_alert_observed_at),
    health_alert = EXCLUDED.health_alert,
	health_alert_observed_at = COALESCE(EXCLUDED.health_alert_observed_at, %s.health_alert_observed_at),
	received_at = EXCLUDED.received_at
`, r.telemetryTable, r.telemetryTable, r.telemetryTable, r.telemetryTable, r.telemetryTable, r.telemetryTable, r.telemetryTable, r.telemetryTable, r.telemetryTable, r.telemetryTable)

	for _, record := range records {
		batch.Queue(query,
			record.ObservedAt,
			record.CowID,
			record.CameraID,
			record.BodyTemperature,
			record.BodyTemperatureObservedAt,
			record.MilkYield,
			record.MilkYieldObservedAt,
			record.MilkReductionRatio,
			record.MilkReductionRatioObservedAt,
			record.ElevatedTempAlert,
			record.ElevatedTempAlertObservedAt,
			record.MilkAlert,
			record.MilkAlertObservedAt,
			record.HealthAlert,
			record.HealthAlertObservedAt,
			record.ReceivedAt,
		)
	}
	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range records {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpsertAlerts(ctx context.Context, events []models.AlertEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	query := fmt.Sprintf(`
INSERT INTO %s (
    cow_id,
	camera_id,
    alert_type,
    start_time,
    end_time,
    peak_temperature,
	milk_yield,
    milk_reduction_ratio,
    resolved
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (cow_id, alert_type, start_time) DO UPDATE SET
	camera_id = EXCLUDED.camera_id,
    end_time = EXCLUDED.end_time,
    peak_temperature = COALESCE(EXCLUDED.peak_temperature, %s.peak_temperature),
	milk_yield = COALESCE(EXCLUDED.milk_yield, %s.milk_yield),
    milk_reduction_ratio = COALESCE(EXCLUDED.milk_reduction_ratio, %s.milk_reduction_ratio),
    resolved = EXCLUDED.resolved
`, r.alertsTable, r.alertsTable, r.alertsTable, r.alertsTable)

	for _, event := range events {
		batch.Queue(query,
			event.CowID,
			event.CameraID,
			event.AlertType,
			event.StartTime,
			event.EndTime,
			event.PeakTemperature,
			event.MilkYield,
			event.MilkReductionRatio,
			event.Resolved,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range events {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func qualifiedName(schema string, table string) string {
	return strings.Join([]string{schema, table}, ".")
}
