package skit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ToggleState tracks the last action performed for a toggle script.
type ToggleState struct {
	LastAction ToggleAction `json:"last_action"`
	UpdatedAt  time.Time    `json:"updated_at"`
	Runs       []RunRecord  `json:"runs,omitempty"`
}

// RunRecord captures metadata for a script execution.
type RunRecord struct {
	Time    time.Time `json:"time"`
	Action  string    `json:"action"`
	Success bool      `json:"success"`
	Output  string    `json:"output,omitempty"`
	Err     string    `json:"err,omitempty"`
}

// StateStore persists ToggleState records on disk.
type StateStore struct {
	root string
	mu   sync.Mutex
}

// NewStateStore initializes the directory if needed.
func NewStateStore(root string) (*StateStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &StateStore{root: root}, nil
}

// Load retrieves a script's state, returning zero values when no record exists.
func (s *StateStore) Load(slug string) (ToggleState, error) {
	path := s.pathFor(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ToggleState{}, nil
		}
		return ToggleState{}, err
	}
	var st ToggleState
	if err := json.Unmarshal(data, &st); err != nil {
		return ToggleState{}, fmt.Errorf("parse state for %s: %w", slug, err)
	}
	return st, nil
}

// Record updates the stored state after a run, recording both successful and failed runs.
func (s *StateStore) Record(slug string, action ToggleAction, record RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := ToggleState{LastAction: action, UpdatedAt: time.Now()}
	if data, err := os.ReadFile(s.pathFor(slug)); err == nil {
		_ = json.Unmarshal(data, &state)
		state.LastAction = action
		state.UpdatedAt = time.Now()
	}
	state.Runs = append(state.Runs, record)
	if len(state.Runs) > 20 {
		state.Runs = state.Runs[len(state.Runs)-20:]
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.pathFor(slug), data, 0o644)
}

// NextAction determines the next toggle action using stored history.
func (s *StateStore) NextAction(slug string) ToggleAction {
	st, err := s.Load(slug)
	if err != nil {
		return ToggleActionEnable
	}
	return NextToggleAction(st.LastAction)
}

func (s *StateStore) pathFor(slug string) string {
	return filepath.Join(s.root, slug+".json")
}

// Delete removes any persisted state for a script.
func (s *StateStore) Delete(slug string) error {
	if s == nil {
		return nil
	}
	path := s.pathFor(slug)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
