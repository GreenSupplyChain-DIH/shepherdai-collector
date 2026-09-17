# Cow Collector

External Go microservice for polling the Jetson livestock API and persisting normalized telemetry into the CEI-InOE PostgreSQL database.

## Scope

- Poll the Jetson API on a fixed interval.
- Normalize incoming telemetry into a stable database shape.
- Filter records to a configured set of cow IDs.
- Upsert telemetry rows idempotently on `(time, cow_id)`.
- Persist active alert events.
- Extract `camera_id` from identifiers such as `333_cam1_dummy`.
- Buffer telemetry locally when PostgreSQL is unavailable.
- Expose a lightweight health endpoint.

## Project layout

- `cmd/cow-collector`: binary entrypoint.
- `internal/config`: environment-backed configuration.
- `internal/jetson`: HTTP client for the Jetson API.
- `internal/normalize`: payload flattening and cow filtering.
- `internal/storage`: PostgreSQL repository logic.
- `internal/buffer`: local JSONL retry buffer.
- `internal/alerts`: alert projection from telemetry.
- `internal/health`: health state and `/health` handler.
- `internal/app`: orchestration and poll loop.

## Local development

1. Copy `.env.example` to `.env` and fill in the Jetson and database values.
2. Ensure the CEI-InOE PostgreSQL service is reachable and the telemetry tables already exist.
3. Run `make test`.
4. Run `make run`.

## Historical backfill

Import a bounded, inclusive UTC date range from the Jetson history endpoint:

```sh
go run ./cmd/cow-collector backfill --from 2026-09-01 --to 2026-09-15
```

The command requests `GET {JETSON_BASE_URL}/health-status/{YYYY-MM-DD}` once for each date, then applies the same normalization, telemetry persistence, and active-alert projection as live polling. It does not start the health server, polling loop, or local retry buffer.

Backfill continues when an individual date fails and exits non-zero after the range completes if any dates failed. It is safe to rerun the same range because telemetry and alert writes are upserts.

## Compose integration with CEI-InOE

The CEI-InOE repository contains `docker-compose-integration-services.yml` for same-machine deployment. Start both stacks from the CEI-InOE repository root:

```sh
docker compose -f docker-compose.dev.yml -f docker-compose-integration-services.yml up --build
```

## Database expectations

The collector assumes these tables already exist in PostgreSQL:

- `jetson_telemetry.cow_telemetry_history`
- `jetson_telemetry.cow_alert_events`

The collector does not run CEI-InOE migrations. CEI-InOE remains the schema owner.

## Cow telemetry and alerts

The CEI-InOE migration creates the `jetson_telemetry` schema and these tables:

- `cow_telemetry_history`: timestamp, cow ID, camera ID, temperature, milk yield, milk reduction ratio, alert flags, health level, and receive time.
- `cow_alert_events`: one row per active alert type (`MILK`, `HEALTH`, or `ELEVATED_BODY`), with timestamp, cow ID, camera ID, and the metric relevant to that alert.

For NGSI-LD properties, each value's `observedAt` is stored in its matching `*_observed_at` column. The telemetry `time` column uses the latest available observation timestamp and falls back to the receipt time only when the payload contains none.

For a source identifier such as `333_cam1_dummy`, the collector stores `cow_id = 333` and `camera_id = cam1`. If no `camN` token is present, `camera_id` is an empty string.
