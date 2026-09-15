package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestParsePollIntervalAcceptsSupportedModes(t *testing.T) {
	for _, test := range []struct {
		value string
		want  time.Duration
	}{
		{value: "30s", want: 30 * time.Second},
		{value: "5m", want: 5 * time.Minute},
	} {
		t.Run(test.value, func(t *testing.T) {
			got, err := ParsePollInterval(test.value)
			if err != nil {
				t.Fatalf("ParsePollInterval(%q) error = %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("ParsePollInterval(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestParsePollIntervalRejectsInvalidMode(t *testing.T) {
	if _, err := ParsePollInterval("10s"); err == nil {
		t.Fatal(`ParsePollInterval("10s") error = nil, want error`)
	}
}

func TestRunPollingRunsImmediatelyThenWaitsBeforeSecondCycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const interval = 30 * time.Second
	var cycles atomic.Int32
	waitStarted := make(chan struct{})
	releaseWait := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- runPolling(ctx, interval, func(context.Context) error {
			if cycles.Add(1) == 2 {
				cancel()
			}
			return nil
		}, func(ctx context.Context, got time.Duration) error {
			if got != interval {
				t.Errorf("wait duration = %v, want %v", got, interval)
			}
			close(waitStarted)
			select {
			case <-releaseWait:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()

	select {
	case <-waitStarted:
	case <-time.After(time.Second):
		t.Fatal("polling did not wait after the immediate cycle")
	}
	if got := cycles.Load(); got != 1 {
		t.Fatalf("cycles before releasing wait = %d, want 1", got)
	}
	close(releaseWait)

	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("runPolling() error = %v", err)
	}
	if got := cycles.Load(); got != 2 {
		t.Fatalf("cycles after releasing wait = %d, want 2", got)
	}
}

func TestRunPollingStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cycles := 0
	waitStarted := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- runPolling(ctx, time.Minute, func(context.Context) error {
			cycles++
			return nil
		}, func(ctx context.Context, _ time.Duration) error {
			close(waitStarted)
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	select {
	case <-waitStarted:
	case <-time.After(time.Second):
		t.Fatal("polling did not enter wait")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runPolling() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runPolling() did not stop after cancellation")
	}
	if cycles != 1 {
		t.Fatalf("cycles = %d, want 1", cycles)
	}
}

func TestRunPollingDoesNotOverlapCycles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})
	done := make(chan error, 1)
	var cycles atomic.Int32

	go func() {
		done <- runPolling(ctx, time.Second, func(context.Context) error {
			switch cycles.Add(1) {
			case 1:
				close(firstStarted)
				<-releaseFirst
			case 2:
				close(secondStarted)
				cancel()
			}
			return nil
		}, func(context.Context, time.Duration) error { return nil })
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first cycle did not start")
	}
	select {
	case <-secondStarted:
		t.Fatal("second cycle started before first cycle completed")
	default:
	}
	close(releaseFirst)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("polling did not finish")
	}
	if got := cycles.Load(); got != 2 {
		t.Fatalf("cycles = %d, want 2", got)
	}
}

func TestRunPollingCancellationWhileWaitingReturnsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	waitStarted := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runPolling(ctx, 5*time.Minute, func(context.Context) error {
			return nil
		}, func(ctx context.Context, _ time.Duration) error {
			close(waitStarted)
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	select {
	case <-waitStarted:
	case <-time.After(time.Second):
		t.Fatal("polling did not enter wait")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runPolling() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runPolling() did not return promptly")
	}
}
