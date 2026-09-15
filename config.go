package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const workspaceGID = "1218492064871302"

const pollIntervalEnv = "ASANA_POLL_INTERVAL"

type Config struct {
	PAT          string
	WorkspaceGID string
	OutputDir    string
	Interval     time.Duration
}

func LoadConfig() (Config, error) {
	pat := strings.TrimSpace(os.Getenv("ASANA_PAT"))
	if pat == "" {
		return Config{}, fmt.Errorf("ASANA_PAT is required")
	}

	intervalValue := strings.TrimSpace(os.Getenv(pollIntervalEnv))
	if intervalValue == "" {
		intervalValue = "30s"
	}
	interval, err := ParsePollInterval(intervalValue)
	if err != nil {
		return Config{}, err
	}

	return Config{
		PAT:          pat,
		WorkspaceGID: workspaceGID,
		Interval:     interval,
	}, nil
}
