// Package state manages lean persistent state for the sfq CLI tool.
// State is stored in the user's home directory at ~/.sfq/state.json.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State holds the lean persistent state for sfq.
type State struct {
	LastFile        string    `json:"last_file"`
	LastOutput      string    `json:"last_output"`
	LastGeneratedAt time.Time `json:"last_generated_at"`
}

func stateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".sfq"), nil
}

func statePath() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

// Load reads the current state from disk. Returns an empty State if none exists yet.
func Load() (*State, error) {
	path, err := statePath()
	if err != nil {
		return &State{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("cannot read state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("corrupt state file: %w", err)
	}
	return &s, nil
}

// Save persists the state to disk, creating the directory if needed.
func Save(s *State) error {
	dir, err := stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create state directory: %w", err)
	}

	path := filepath.Join(dir, "state.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cannot write state file: %w", err)
	}
	return nil
}

// RecordGeneration updates the state to reflect a newly generated file.
func RecordGeneration(sourceFile, outputFile string) error {
	s := &State{
		LastFile:        sourceFile,
		LastOutput:      outputFile,
		LastGeneratedAt: time.Now().UTC(),
	}
	return Save(s)
}
