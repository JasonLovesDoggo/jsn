package skit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	Time     time.Time
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
	command, err := script.ResolveCommand(action)
	if err != nil {
		return RunResult{Script: script, Action: action, Duration: time.Since(start), Err: err}
	}
	var execCmd *exec.Cmd
	if command.Inline {
		execCmd = inlineCommand(ctx, command.Value)
	} else {
		execCmd = exec.CommandContext(ctx, command.Value)
	}
	execCmd.Dir = script.Dir
	execCmd.Env = append(os.Environ(), flattenEnv(script.Env)...)
	var buf bytes.Buffer
	execCmd.Stdout = &buf
	execCmd.Stderr = &buf
	err = execCmd.Run()
	result := RunResult{
		Script:   script,
		Action:   action,
		Output:   buf.String(),
		Duration: time.Since(start),
		Err:      err,
		Time:     start,
	}
	if r.states != nil {
		record := RunRecord{
			Time:    result.Time,
			Action:  string(action),
			Success: err == nil,
			Output:  result.Output,
		}
		if err != nil {
			record.Err = err.Error()
		}
		if recErr := r.states.Record(script.Slug, action, record); recErr != nil {
			fmt.Fprintf(os.Stderr, "skit: failed to record history for %s: %v\n", script.Slug, recErr)
		}
	}
	return result
}

func inlineCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
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
