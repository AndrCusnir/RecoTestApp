package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExtractionCycleSavesUsersAndProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users":
			_, _ = io.WriteString(w, `{"data":[{"gid":"user-1","name":"Ada","unknown_user_field":true}],"next_page":null}`)
		case "/projects":
			_, _ = io.WriteString(w, `{"data":[{"gid":"project-1","name":"Alpha","unknown_project_field":[1,2]}],"next_page":null}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	outputDir := t.TempDir()
	storage := NewStorage(outputDir)
	if err := storage.Prepare(); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL
	config := Config{PAT: "test-pat", WorkspaceGID: workspaceGID}

	if err := runExtractionCycle(context.Background(), config, client, storage, outputDir); err != nil {
		t.Fatalf("runExtractionCycle() error = %v", err)
	}

	assertStoredJSONEquals(t, filepath.Join(outputDir, "users", "user-1.json"), json.RawMessage(`{"gid":"user-1","name":"Ada","unknown_user_field":true}`))
	assertStoredJSONEquals(t, filepath.Join(outputDir, "projects", "project-1.json"), json.RawMessage(`{"gid":"project-1","name":"Alpha","unknown_project_field":[1,2]}`))
}

func TestRunExtractionCyclePropagatesStorageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"gid":"user-1"}],"next_page":null}`)
	}))
	defer server.Close()

	root := filepath.Join(t.TempDir(), "output")
	if err := os.WriteFile(root, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	storage := NewStorage(root)
	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL
	config := Config{PAT: "test-pat", WorkspaceGID: workspaceGID}

	err := runExtractionCycle(context.Background(), config, client, storage, root)
	if err == nil {
		t.Fatal("runExtractionCycle() error = nil, want storage error")
	}
	if !strings.Contains(err.Error(), "save user") {
		t.Fatalf("runExtractionCycle() error = %q, want save user context", err)
	}
}
