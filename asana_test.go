package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadConfigRequiresAsanaPAT(t *testing.T) {
	t.Setenv("ASANA_PAT", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want missing PAT error")
	}
	if !strings.Contains(err.Error(), "ASANA_PAT") {
		t.Fatalf("LoadConfig() error = %q, want it to mention ASANA_PAT", err)
	}
}

func TestExtractUsersSinglePageAuthenticatesAndPreservesRawEntity(t *testing.T) {
	const token = "test-pat"
	const unknownFieldValue = "preserved value"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/users" {
			t.Errorf("request path = %q, want /users", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer "+token)
		}
		if got := r.URL.Query().Get("workspace"); got != workspaceGID {
			t.Errorf("workspace query = %q, want %q", got, workspaceGID)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit query = %q, want 100", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"gid":"user-123","name":"Ada","future_field":"`+unknownFieldValue+`"}],"next_page":null}`)
	}))
	defer server.Close()

	client := NewClient(token, server.Client())
	client.BaseURL = server.URL

	var gotGID string
	var gotEntity json.RawMessage
	err := client.ExtractUsers(context.Background(), workspaceGID, func(gid string, entity json.RawMessage) error {
		gotGID = gid
		gotEntity = entity
		return nil
	})
	if err != nil {
		t.Fatalf("ExtractUsers() error = %v", err)
	}
	if gotGID != "user-123" {
		t.Fatalf("consumer GID = %q, want user-123", gotGID)
	}
	if !strings.Contains(string(gotEntity), `"future_field":"`+unknownFieldValue+`"`) {
		t.Fatalf("consumer entity = %s, want unknown field preserved", gotEntity)
	}

	var decoded map[string]any
	if err := json.Unmarshal(gotEntity, &decoded); err != nil {
		t.Fatalf("consumer entity is not valid JSON: %v", err)
	}
	if decoded["future_field"] != unknownFieldValue {
		t.Fatalf("future_field = %v, want %q", decoded["future_field"], unknownFieldValue)
	}
}

func TestExtractUsersPaginatesAndPreservesEveryRawEntity(t *testing.T) {
	const token = "test-pat"

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/users" {
			t.Errorf("request path = %q, want /users", r.URL.Path)
		}
		if got := r.URL.Query().Get("workspace"); got != workspaceGID {
			t.Errorf("workspace query = %q, want %q", got, workspaceGID)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit query = %q, want 100", got)
		}

		w.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			if got := r.URL.Query().Get("offset"); got != "" {
				t.Errorf("first request offset = %q, want empty", got)
			}
			_, _ = io.WriteString(w, `{"data":[{"gid":"user-1","name":"Ada","page_field":"one"},{"gid":"user-2","name":"Grace","page_field":"one"}],"next_page":{"offset":"page2"}}`)
		case 2:
			if got := r.URL.Query().Get("offset"); got != "page2" {
				t.Errorf("second request offset = %q, want page2", got)
			}
			_, _ = io.WriteString(w, `{"data":[{"gid":"user-3","name":"Lin","page_field":"two"},{"gid":"user-4","name":"Katherine","page_field":"two"}],"next_page":null}`)
		default:
			http.Error(w, "unexpected extra request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(token, server.Client())
	client.BaseURL = server.URL

	entities := make(map[string]json.RawMessage)
	err := client.ExtractUsers(context.Background(), workspaceGID, func(gid string, entity json.RawMessage) error {
		if _, exists := entities[gid]; exists {
			t.Errorf("consumer received duplicate GID %q", gid)
		}
		entities[gid] = entity
		return nil
	})
	if err != nil {
		t.Fatalf("ExtractUsers() error = %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("HTTP request count = %d, want 2", requestCount)
	}
	if len(entities) != 4 {
		t.Fatalf("consumer received %d users, want 4", len(entities))
	}
	for gid, expectedPage := range map[string]string{
		"user-1": "one",
		"user-2": "one",
		"user-3": "two",
		"user-4": "two",
	} {
		entity, ok := entities[gid]
		if !ok {
			t.Errorf("consumer did not receive %q", gid)
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(entity, &decoded); err != nil {
			t.Errorf("entity %q is not valid JSON: %v", gid, err)
			continue
		}
		if decoded["page_field"] != expectedPage {
			t.Errorf("entity %q page_field = %v, want %q", gid, decoded["page_field"], expectedPage)
		}
	}
}

func TestExtractProjectsPaginatesAndPreservesEveryRawEntity(t *testing.T) {
	const token = "test-pat"

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/projects" {
			t.Errorf("request path = %q, want /projects", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer "+token)
		}
		if got := r.URL.Query().Get("workspace"); got != workspaceGID {
			t.Errorf("workspace query = %q, want %q", got, workspaceGID)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit query = %q, want 100", got)
		}

		w.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			if got := r.URL.Query().Get("offset"); got != "" {
				t.Errorf("first request offset = %q, want empty", got)
			}
			_, _ = io.WriteString(w, `{"data":[{"gid":"project-1","name":"Alpha","project_field":"one"},{"gid":"project-2","name":"Beta","project_field":"one"}],"next_page":{"offset":"projects-page-2"}}`)
		case 2:
			if got := r.URL.Query().Get("offset"); got != "projects-page-2" {
				t.Errorf("second request offset = %q, want projects-page-2", got)
			}
			_, _ = io.WriteString(w, `{"data":[{"gid":"project-3","name":"Gamma","project_field":"two"},{"gid":"project-4","name":"Delta","project_field":"two"}],"next_page":null}`)
		default:
			http.Error(w, "unexpected extra request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(token, server.Client())
	client.BaseURL = server.URL

	entities := make(map[string]json.RawMessage)
	err := client.ExtractProjects(context.Background(), workspaceGID, func(gid string, entity json.RawMessage) error {
		if _, exists := entities[gid]; exists {
			t.Errorf("consumer received duplicate GID %q", gid)
		}
		entities[gid] = entity
		return nil
	})
	if err != nil {
		t.Fatalf("ExtractProjects() error = %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("HTTP request count = %d, want 2", requestCount)
	}
	if len(entities) != 4 {
		t.Fatalf("consumer received %d projects, want 4", len(entities))
	}
	for gid, expectedPage := range map[string]string{
		"project-1": "one",
		"project-2": "one",
		"project-3": "two",
		"project-4": "two",
	} {
		entity, ok := entities[gid]
		if !ok {
			t.Errorf("consumer did not receive %q", gid)
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(entity, &decoded); err != nil {
			t.Errorf("entity %q is not valid JSON: %v", gid, err)
			continue
		}
		if decoded["project_field"] != expectedPage {
			t.Errorf("entity %q project_field = %v, want %q", gid, decoded["project_field"], expectedPage)
		}
	}
}
