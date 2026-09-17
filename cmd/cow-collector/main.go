package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/app"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/config"
)

func main() {
	var err error
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "backfill":
			err = runBackfill(os.Args[2:])
		default:
			err = fmt.Errorf("unknown command %q", os.Args[1])
		}
	} else {
		err = runCollector()
	}
	if err != nil {
		slog.Error("cow collector failed", "error", err)
		os.Exit(1)
	}
}

func runCollector() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	collector, err := app.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := collector.Run(ctx); err != nil {
		return fmt.Errorf("run collector: %w", err)
	}
	logger.Info("collector stopped")
	return nil
}

func runBackfill(args []string) error {
	from, to, err := parseBackfillDates(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	collector, err := app.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := collector.Backfill(ctx, from, to); err != nil {
		return fmt.Errorf("backfill: %w", err)
	}
	logger.Info("backfill complete", "from", from.Format(time.DateOnly), "to", to.Format(time.DateOnly))
	return nil
}

func parseBackfillDates(args []string) (time.Time, time.Time, error) {
	flags := flag.NewFlagSet("backfill", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	fromRaw := flags.String("from", "", "first UTC date to import (YYYY-MM-DD)")
	toRaw := flags.String("to", "", "last UTC date to import (YYYY-MM-DD)")
	if err := flags.Parse(args); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse backfill flags: %w", err)
	}
	if flags.NArg() != 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("backfill accepts only --from and --to flags")
	}
	if *fromRaw == "" || *toRaw == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("backfill requires --from and --to in YYYY-MM-DD format")
	}
	from, err := time.Parse(time.DateOnly, *fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --from date %q: %w", *fromRaw, err)
	}
	to, err := time.Parse(time.DateOnly, *toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --to date %q: %w", *toRaw, err)
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("--from date must not be after --to date")
	}
	return from.UTC(), to.UTC(), nil
}
