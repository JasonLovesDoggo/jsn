package skit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Runner executes scripts and records toggle state transitions.
type Runner struct {
	states *StateStore
}

// NewRunner wires a state store into the runner.
func NewRunner(store *StateStore) *Runner {
	return &Runner{states: store}
}

// RunResult stores execution metadata for the TUI to display.
type RunResult struct {
	Script   *Script
	Action   ToggleAction
	Output   string
	Duration time.Duration
	Err      error
}

// Execute resolves the correct command for the current platform and runs it.
func (r *Runner) Execute(ctx context.Context, script *Script) RunResult {
	start := time.Now()
	action := ToggleAction("")
	if script.Type == ScriptTypeToggle {
		if r.states != nil {
			action = r.states.NextAction(script.Slug)
		} else {
			action = ToggleActionEnable
		}
	}
	cmdPath, err := script.ResolveCommand(action)
	if err != nil {
		return RunResult{Script: script, Action: action, Duration: time.Since(start), Err: err}
	}
	command := exec.CommandContext(ctx, cmdPath)
	command.Dir = script.Dir
	command.Env = append(os.Environ(), flattenEnv(script.Env)...)
	var buf bytes.Buffer
	command.Stdout = &buf
	command.Stderr = &buf
	err = command.Run()
	result := RunResult{
		Script:   script,
		Action:   action,
		Output:   buf.String(),
		Duration: time.Since(start),
		Err:      err,
	}
	if err == nil && script.Type == ScriptTypeToggle && r.states != nil {
		if recErr := r.states.Record(script.Slug, action); recErr != nil {
			result.Err = fmt.Errorf("record toggle state: %w", recErr)
		}
	}
	return result
}

func flattenEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
