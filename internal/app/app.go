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
	receivedAt := time.Now().UTC()
	records, rejected := normalize.Records(a.cfg, payloads, receivedAt)
	events := alerts.ActiveEvents(records)

	if err := a.repo.UpsertTelemetry(ctx, records); err != nil {
		_ = a.buffer.Store(ctx, records)
		a.health.RecordBuffer(len(records), err)
		return err
	}
	if err := a.repo.UpsertAlerts(ctx, events); err != nil {
		a.logger.Warn("alert upsert failed", "error", err)
	}
	if err := a.flushBuffered(ctx); err != nil {
		a.logger.Warn("buffer flush failed", "error", err)
	}

	a.health.RecordPoll(len(records), rejected)
	a.logger.Info("poll complete", "fetched", len(payloads), "accepted", len(records), "rejected", rejected, "duration", time.Since(started).String())
	return nil
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
