package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoragePrepareCreatesEntityDirectories(t *testing.T) {
	storage := NewStorage(filepath.Join(t.TempDir(), "output"))

	if err := storage.Prepare(); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	for _, directory := range []string{"users", "projects"} {
		path := filepath.Join(storage.RootDir, directory)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", path)
		}
	}
}

func TestStorageSavesUsersAndProjectsWithRawJSON(t *testing.T) {
	storage := NewStorage(filepath.Join(t.TempDir(), "output"))
	user := json.RawMessage(`{"gid":"user-1","name":"Ada","unknown":{"value":true}}`)
	project := json.RawMessage(`{"gid":"project-1","name":"Alpha","future_field":[1,2,3]}`)

	if err := storage.SaveUser("user-1", user); err != nil {
		t.Fatalf("SaveUser() error = %v", err)
	}
	if err := storage.SaveProject("project-1", project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	assertStoredJSONEquals(t, filepath.Join(storage.RootDir, "users", "user-1.json"), user)
	assertStoredJSONEquals(t, filepath.Join(storage.RootDir, "projects", "project-1.json"), project)
}

func TestStorageOverwritesExistingEntityFile(t *testing.T) {
	storage := NewStorage(filepath.Join(t.TempDir(), "output"))
	first := json.RawMessage(`{"gid":"user-1","name":"Before","unknown":false}`)
	second := json.RawMessage(`{"gid":"user-1","name":"After","unknown":true}`)

	if err := storage.SaveUser("user-1", first); err != nil {
		t.Fatalf("first SaveUser() error = %v", err)
	}
	if err := storage.SaveUser("user-1", second); err != nil {
		t.Fatalf("second SaveUser() error = %v", err)
	}

	assertStoredJSONEquals(t, filepath.Join(storage.RootDir, "users", "user-1.json"), second)
}

func TestStorageReturnsFilesystemErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "output")
	if err := os.WriteFile(root, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	storage := NewStorage(root)
	if err := storage.Prepare(); err == nil {
		t.Fatal("Prepare() error = nil, want filesystem error")
	}
}

func assertStoredJSONEquals(t *testing.T, path string, want json.RawMessage) {
	t.Helper()

	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var got any
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatalf("stored JSON in %q is invalid: %v", path, err)
	}
	var expected any
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Fatalf("test JSON is invalid: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("stored JSON in %q = %s, want semantic JSON %s", path, gotBytes, want)
	}
}
