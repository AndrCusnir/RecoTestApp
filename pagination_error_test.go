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

func TestExtractUsersRejectsNextPageWithoutOffset(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = io.WriteString(w, `{"data":[],"next_page":{}}`)
	}))
	defer server.Close()

	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL

	err := client.ExtractUsers(context.Background(), testWorkspaceGID, func(string, json.RawMessage) error {
		return nil
	})
	if err == nil {
		t.Fatal("ExtractUsers() error = nil, want pagination error")
	}
	if !strings.Contains(err.Error(), "next_page without offset") {
		t.Fatalf("ExtractUsers() error = %q, want empty-offset context", err)
	}
	if requests != 1 {
		t.Fatalf("HTTP requests = %d, want 1", requests)
	}
}
