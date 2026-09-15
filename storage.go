package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Storage struct {
	RootDir string
}

func NewStorage(rootDir string) *Storage {
	return &Storage{RootDir: rootDir}
}

func (s *Storage) Prepare() error {
	for _, directory := range []string{"users", "projects"} {
		path := filepath.Join(s.RootDir, directory)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("create storage directory %q: %w", path, err)
		}
	}
	return nil
}

func (s *Storage) SaveUser(gid string, entity json.RawMessage) error {
	return s.save("users", gid, entity)
}

func (s *Storage) SaveProject(gid string, entity json.RawMessage) error {
	return s.save("projects", gid, entity)
}

func (s *Storage) save(kind string, gid string, entity json.RawMessage) error {
	if err := s.Prepare(); err != nil {
		return err
	}

	path := filepath.Join(s.RootDir, kind, gid+".json")
	if err := os.WriteFile(path, entity, 0644); err != nil {
		return fmt.Errorf("write %s %q: %w", kind, path, err)
	}
	return nil
}
