package main

import "testing"

func TestLoadConfigRequiresWorkspaceGID(t *testing.T) {
	t.Setenv("ASANA_PAT", "test-pat")
	t.Setenv("ASANA_WORKSPACE_GID", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want missing workspace GID error")
	}
	if err.Error() != "ASANA_WORKSPACE_GID is required" {
		t.Fatalf("LoadConfig() error = %q, want workspace GID error", err)
	}
}

func TestLoadConfigReadsWorkspaceGID(t *testing.T) {
	const workspace = "workspace-123"
	t.Setenv("ASANA_PAT", "test-pat")
	t.Setenv("ASANA_WORKSPACE_GID", workspace)
	t.Setenv("ASANA_POLL_INTERVAL", "30s")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.PAT != "test-pat" {
		t.Fatalf("Config.PAT = %q, want test-pat", config.PAT)
	}
	if config.WorkspaceGID != workspace {
		t.Fatalf("Config.WorkspaceGID = %q, want %q", config.WorkspaceGID, workspace)
	}
}
