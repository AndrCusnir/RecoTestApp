package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractUsersRetriesAfter429AndDeliversEntity(t *testing.T) {
	requestCount := 0
	var sleepCalls int
	var sleptFor time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"gid":"user-1","name":"Ada"}],"next_page":null}`)
	}))
	defer server.Close()

	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL
	client.MaxRetries = 1
	client.Sleep = func(ctx context.Context, delay time.Duration) error {
		sleepCalls++
		sleptFor = delay
		return nil
	}

	var gotGID string
	err := client.ExtractUsers(context.Background(), testWorkspaceGID, func(gid string, entity json.RawMessage) error {
		gotGID = gid
		return nil
	})
	if err != nil {
		t.Fatalf("ExtractUsers() error = %v", err)
	}
	if sleepCalls != 1 {
		t.Fatalf("Sleep calls = %d, want 1", sleepCalls)
	}
	if sleptFor != 2*time.Second {
		t.Fatalf("Sleep duration = %v, want 2s", sleptFor)
	}
	if requestCount != 2 {
		t.Fatalf("HTTP attempts = %d, want 2", requestCount)
	}
	if gotGID != "user-1" {
		t.Fatalf("consumer GID = %q, want user-1", gotGID)
	}
}

func TestExtractUsersStopsAfterMaxRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL
	client.MaxRetries = 2
	client.Sleep = func(context.Context, time.Duration) error { return nil }

	err := client.ExtractUsers(context.Background(), testWorkspaceGID, func(string, json.RawMessage) error {
		return nil
	})
	if err == nil {
		t.Fatal("ExtractUsers() error = nil, want exhausted retry error")
	}
	if attempts != 3 {
		t.Fatalf("HTTP attempts = %d, want 3", attempts)
	}
}

func TestExtractUsersDoesNotRetryNon429Errors(t *testing.T) {
	attempts := 0
	sleepCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL
	client.MaxRetries = 3
	client.Sleep = func(context.Context, time.Duration) error {
		sleepCalls++
		return nil
	}

	err := client.ExtractUsers(context.Background(), testWorkspaceGID, func(string, json.RawMessage) error {
		return nil
	})
	if err == nil {
		t.Fatal("ExtractUsers() error = nil, want 401 error")
	}
	if attempts != 1 {
		t.Fatalf("HTTP attempts = %d, want 1", attempts)
	}
	if sleepCalls != 0 {
		t.Fatalf("Sleep calls = %d, want 0", sleepCalls)
	}
}

func TestExtractUsersPropagatesSleepCancellation(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL
	client.MaxRetries = 1
	client.Sleep = func(context.Context, time.Duration) error {
		return context.Canceled
	}

	err := client.ExtractUsers(context.Background(), testWorkspaceGID, func(string, json.RawMessage) error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExtractUsers() error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("HTTP attempts = %d, want 1", attempts)
	}
}

func TestExtractUsersRetryPreservesPaginationQuery(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			_, _ = io.WriteString(w, `{"data":[],"next_page":{"offset":"page2"}}`)
			return
		}
		if requestCount == 2 {
			if got := r.URL.Query().Get("workspace"); got != testWorkspaceGID {
				t.Errorf("429 request workspace = %q, want %q", got, testWorkspaceGID)
			}
			if got := r.URL.Query().Get("limit"); got != "100" {
				t.Errorf("429 request limit = %q, want 100", got)
			}
			if got := r.URL.Query().Get("offset"); got != "page2" {
				t.Errorf("429 request offset = %q, want page2", got)
			}
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if got := r.URL.Query().Get("offset"); got != "page2" {
			t.Errorf("retry request offset = %q, want page2", got)
		}
		_, _ = io.WriteString(w, `{"data":[{"gid":"user-2"}],"next_page":null}`)
	}))
	defer server.Close()

	client := NewClient("test-pat", server.Client())
	client.BaseURL = server.URL
	client.MaxRetries = 1
	client.Sleep = func(context.Context, time.Duration) error { return nil }

	var gotGID string
	err := client.ExtractUsers(context.Background(), testWorkspaceGID, func(gid string, entity json.RawMessage) error {
		gotGID = gid
		return nil
	})
	if err != nil {
		t.Fatalf("ExtractUsers() error = %v", err)
	}
	if requestCount != 3 {
		t.Fatalf("HTTP attempts = %d, want 3", requestCount)
	}
	if gotGID != "user-2" {
		t.Fatalf("consumer GID = %q, want user-2", gotGID)
	}
}
