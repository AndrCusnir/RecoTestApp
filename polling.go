package main

import (
	"context"
	"fmt"
	"time"
)

type ExtractionCycle func(context.Context) error
type PollWait func(context.Context, time.Duration) error

func ParsePollInterval(value string) (time.Duration, error) {
	switch value {
	case "30s":
		return 30 * time.Second, nil
	case "5m":
		return 5 * time.Minute, nil
	default:
		return 0, fmt.Errorf("unsupported polling interval %q; use 30s or 5m", value)
	}
}

func waitForInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runPolling(ctx context.Context, interval time.Duration, cycle ExtractionCycle, wait PollWait) error {
	if wait == nil {
		wait = waitForInterval
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := cycle(ctx); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := wait(ctx, interval); err != nil {
			return err
		}
	}
}
