package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/alerts"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/buffer"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/config"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/health"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/jetson"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/normalize"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/storage"
)

type App struct {
	cfg        config.Config
	logger     *slog.Logger
	client     *jetson.Client
	repo       *storage.Repository
	buffer     *buffer.JSONLBuffer
	health     *health.State
	server     *http.Server
	backoff    time.Duration
	maxBackoff time.Duration
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo, err := storage.NewRepository(ctx, cfg)
	if err != nil {
		return nil, err
	}

	h := health.NewState()
	server := &http.Server{
		Addr:              cfg.HealthAddr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		cfg:        cfg,
		logger:     logger,
		client:     jetson.NewClient(cfg),
		repo:       repo,
		buffer:     buffer.New(cfg.BufferPath),
		health:     h,
		server:     server,
		backoff:    cfg.BackoffBase,
		maxBackoff: cfg.BackoffMax,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.repo.Close()

	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("health server failed", "error", err)
		}
	}()

	if err := a.flushBuffered(ctx); err != nil {
		a.logger.Warn("initial buffer flush failed", "error", err)
	}

	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()

	for {
		err := a.pollOnce(ctx)
		if err != nil {
			a.logger.Warn("poll failed", "error", err)
			select {
			case <-ctx.Done():
				return a.shutdown(context.Background())
			case <-time.After(a.backoff):
			}
			a.backoff *= 2
			if a.backoff > a.maxBackoff {
				a.backoff = a.maxBackoff
			}
		} else {
			a.backoff = a.cfg.BackoffBase
		}

		select {
		case <-ctx.Done():
			return a.shutdown(context.Background())
		case <-ticker.C:
		}
	}
}

func (a *App) Backfill(ctx context.Context, from time.Time, to time.Time) error {
	defer a.repo.Close()

	var failures []error
	for date := from.UTC(); !date.After(to.UTC()); date = date.AddDate(0, 0, 1) {
		if err := ctx.Err(); err != nil {
			return err
		}

		started := time.Now()
		payloads, err := a.client.FetchForDate(ctx, date)
		if err == nil {
			accepted, rejected, persistErr := a.persistPayloads(ctx, payloads, time.Now().UTC(), true)
			if persistErr != nil {
				err = persistErr
			} else {
				a.logger.Info("backfill date complete", "date", date.Format(time.DateOnly), "fetched", len(payloads), "accepted", accepted, "rejected", rejected, "duration", time.Since(started).String())
			}
		}
		if err != nil {
			failure := fmt.Errorf("backfill %s: %w", date.Format(time.DateOnly), err)
			failures = append(failures, failure)
			a.logger.Error("backfill date failed", "date", date.Format(time.DateOnly), "error", err, "duration", time.Since(started).String())
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("backfill completed with %d failed date(s): %w", len(failures), errors.Join(failures...))
	}
	return nil
}

func (a *App) shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return a.server.Shutdown(shutdownCtx)
}

func (a *App) pollOnce(ctx context.Context) error {
	started := time.Now()
	payloads, err := a.client.Fetch(ctx)
	if err != nil {
		return err
	}
	accepted, rejected, err := a.persistPayloads(ctx, payloads, time.Now().UTC(), false)
	if err != nil {
		records, _ := normalize.Records(a.cfg, payloads, time.Now().UTC())
		_ = a.buffer.Store(ctx, records)
		a.health.RecordBuffer(len(records), err)
		return err
	}
	if err := a.flushBuffered(ctx); err != nil {
		a.logger.Warn("buffer flush failed", "error", err)
	}

	a.health.RecordPoll(accepted, rejected)
	a.logger.Info("poll complete", "fetched", len(payloads), "accepted", accepted, "rejected", rejected, "duration", time.Since(started).String())
	return nil
}

func (a *App) persistPayloads(ctx context.Context, payloads []map[string]any, receivedAt time.Time, returnAlertError bool) (int, int, error) {
	records, rejected := normalize.Records(a.cfg, payloads, receivedAt)
	if err := a.repo.UpsertTelemetry(ctx, records); err != nil {
		return len(records), rejected, err
	}
	if err := a.repo.UpsertAlerts(ctx, alerts.ActiveEvents(records)); err != nil {
		if returnAlertError {
			return len(records), rejected, err
		}
		a.logger.Warn("alert upsert failed", "error", err)
	}
	return len(records), rejected, nil
}

func (a *App) flushBuffered(ctx context.Context) error {
	records, err := a.buffer.Load(ctx)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if err := a.repo.UpsertTelemetry(ctx, records); err != nil {
		return fmt.Errorf("flush buffered records: %w", err)
	}
	a.health.RecordFlush()
	return a.buffer.Reset(ctx)
}
