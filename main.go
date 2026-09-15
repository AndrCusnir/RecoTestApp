package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client, storage, outputDir, err := newExtractionRuntime(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = runPolling(ctx, config.Interval, func(ctx context.Context) error {
		return runExtractionCycle(ctx, config, client, storage, outputDir)
	}, nil)
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runOnce(ctx context.Context) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	return runOnceWithConfig(ctx, config)
}

func runOnceWithConfig(ctx context.Context, config Config) error {
	client, storage, outputDir, err := newExtractionRuntime(config)
	if err != nil {
		return err
	}
	return runExtractionCycle(ctx, config, client, storage, outputDir)
}

func newExtractionRuntime(config Config) (*Client, *Storage, string, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := NewClient(config.PAT, httpClient)
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = "output"
	}
	storage := NewStorage(outputDir)
	if err := storage.Prepare(); err != nil {
		return nil, nil, "", fmt.Errorf("prepare storage: %w", err)
	}
	return client, storage, outputDir, nil
}

func runExtractionCycle(ctx context.Context, config Config, client *Client, storage *Storage, outputDir string) error {
	userCount := 0
	if err := client.ExtractUsers(ctx, config.WorkspaceGID, func(gid string, entity json.RawMessage) error {
		if err := storage.SaveUser(gid, entity); err != nil {
			return fmt.Errorf("save user %q: %w", gid, err)
		}
		userCount++
		return nil
	}); err != nil {
		return fmt.Errorf("extract users: %w", err)
	}

	projectCount := 0
	if err := client.ExtractProjects(ctx, config.WorkspaceGID, func(gid string, entity json.RawMessage) error {
		if err := storage.SaveProject(gid, entity); err != nil {
			return fmt.Errorf("save project %q: %w", gid, err)
		}
		projectCount++
		return nil
	}); err != nil {
		return fmt.Errorf("extract projects: %w", err)
	}

	fmt.Printf("extracted users: %d\n", userCount)
	fmt.Printf("extracted projects: %d\n", projectCount)
	fmt.Printf("output directory: %s\n", outputDir)
	return nil
}
