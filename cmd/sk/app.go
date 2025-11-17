package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"pkg.jsn.cam/jsn/internal/skit"
)

type mode int

const (
	modeBrowse mode = iota
	modeAction
	modeNew
	modeConfirmDelete
	modeChooseCommand
	modeHistory
)

type runMsg struct {
	result skit.RunResult
}

type scriptsUpdatedMsg struct {
	scripts   []*skit.Script
	err       error
	focusSlug string
}

type model struct {
	mode          mode
	scriptsDir    string
	scripts       []*skit.Script
	matches       []int
	cursor        int
	historyIdx    int
	runner        *skit.Runner
	stateStore    *skit.StateStore
	toggleCache   map[string]skit.ToggleAction
	search        textinput.Model
	prompt        textinput.Model
	status        string
	statusErr     bool
	running       bool
	lastResult    *skit.RunResult
	showDetails   bool
	pendingDelete *skit.Script
	pendingEdit   *skit.Script
}

func newModel(dir string, scripts []*skit.Script, runner *skit.Runner, store *skit.StateStore) model {
	search := textinput.New()
	search.Placeholder = "Search scripts"
	search.CharLimit = 256
	search.Focus()
	search.Prompt = "> "
	m := model{
		mode:        modeBrowse,
		scriptsDir:  dir,
		scripts:     scripts,
		runner:      runner,
		stateStore:  store,
		search:      search,
		toggleCache: make(map[string]skit.ToggleAction),
	}
	m.rebuildMatches()
	m.loadToggleCache()
	m.setStatus("Ready", false)
	return m
}

func (m *model) setStatus(msg string, err bool) {
	m.status = msg
	m.statusErr = err
}

func (m *model) rebuildMatches() {
	m.matches = m.matches[:0]
	for i := range m.scripts {
		m.matches = append(m.matches, i)
	}
	if len(m.matches) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.matches) {
		m.cursor = len(m.matches) - 1
	}
}

func (m *model) loadToggleCache() {
	if m.stateStore == nil {
		return
	}
	for _, s := range m.scripts {
		state, err := m.stateStore.Load(s.Slug)
		if err != nil {
			continue
		}
		if s.Type == skit.ScriptTypeToggle {
			m.toggleCache[s.Slug] = state.LastAction
		}
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case runMsg:
		m.running = false
		m.lastResult = &msg.result
		if msg.result.Err != nil {
			m.setStatus(fmt.Sprintf("Failed: %v", msg.result.Err), true)
		} else {
			m.setStatus(fmt.Sprintf("Finished in %s", msg.result.Duration.Round(10*time.Millisecond)), false)
		}
		if msg.result.Script != nil {
			if msg.result.Script.Type == skit.ScriptTypeToggle {
				m.toggleCache[msg.result.Script.Slug] = msg.result.Action
			}
		}
		return m, nil
	case scriptsUpdatedMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
			return m, nil
		}
		m.scripts = msg.scripts
		m.toggleCache = make(map[string]skit.ToggleAction, len(m.scripts))
		m.loadToggleCache()
		m.rebuildMatches()
		m.recomputeMatches()
		if msg.focusSlug != "" {
			m.focusSlug(msg.focusSlug)
		}
		m.mode = modeBrowse
		m.pendingDelete = nil
		m.setStatus("Scripts refreshed", false)
		return m, nil
	case commandEditMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Editor: %v", msg.err), true)
		} else {
			m.setStatus("Script updated", false)
		}
		m.mode = modeBrowse
		m.pendingEdit = nil
		return m, nil
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeBrowse:
		return m.handleBrowseKey(msg)
	case modeAction:
		return m.handleActionKey(msg)
	case modeNew:
		return m.handleNewKey(msg)
	case modeConfirmDelete:
		return m.handleDeleteKey(msg)
	case modeHistory:
		return m.handleHistoryKey(msg)
	case modeChooseCommand:
		return m.handleCommandChoiceKey(msg)
	default:
		return m, nil
	}
}

func (m *model) handleBrowseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "right", "tab":
		m.mode = modeAction
		m.setStatus("Action mode: e edit manifest, o edit script, n new, d delete, ← to exit", false)
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.matches)-1 {
			m.cursor++
		}
		return m, nil
	case "ctrl+r":
		if m.lastResult != nil && !m.running {
			return m.startRun(m.lastResult.Script)
		}
		return m, nil
	case "enter":
		script := m.currentScript()
		if script == nil || m.running {
			return m, nil
		}
		if !script.SupportsCurrentPlatform() {
			m.setStatus("Script not available on this platform", true)
			return m, nil
		}
		return m.startRun(script)
	case "?":
		m.showDetails = !m.showDetails
		return m, nil
	case "h":
		if m.currentScript() == nil {
			return m, nil
		}
		m.mode = modeHistory
		m.setStatus("History mode (h/esc to exit)", false)
		return m, nil
	}
	prev := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if prev != m.search.Value() {
		m.recomputeMatches()
	}
	return m, cmd
}

func (m *model) handleActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "esc":
		m.mode = modeBrowse
		m.setStatus("Browse mode", false)
		return m, nil
	case "e":
		script := m.currentScript()
		if script == nil {
			m.setStatus("Select a script to edit", true)
			return m, nil
		}
		m.mode = modeBrowse
		return m, m.editScript(script)
	case "n":
		m.mode = modeNew
		m.prompt = textinput.New()
		m.prompt.Placeholder = "script slug"
		m.prompt.CharLimit = 64
		m.prompt.Prompt = "slug> "
		m.prompt.Focus()
		m.prompt.SetValue(sanitizeSlug(m.search.Value()))
		m.setStatus("Enter new script slug", false)
		return m, nil
	case "d":
		script := m.currentScript()
		if script == nil {
			m.setStatus("Select a script to delete", true)
			return m, nil
		}
		m.mode = modeConfirmDelete
		m.pendingDelete = script
		m.setStatus(fmt.Sprintf("Delete %s? (y/n)", script.Name), true)
		return m, nil
	case "o":
		script := m.currentScript()
		if script == nil {
			m.setStatus("Select a script to edit", true)
			return m, nil
		}
		if script.Type == skit.ScriptTypeToggle {
			m.mode = modeChooseCommand
			m.pendingEdit = script
			m.setStatus("Toggle edit: e enable, d disable, Esc cancel", false)
			return m, nil
		}
		m.mode = modeBrowse
		return m.startEditCommandFile(script, "")
	default:
		return m, nil
	}
}

func (m *model) handleNewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeAction
		m.setStatus("Action mode: e edit manifest, o edit script, n new, d delete, ← to exit", false)
		return m, nil
	case "enter":
		slug := sanitizeSlug(m.prompt.Value())
		if slug == "" {
			m.setStatus("Slug cannot be empty", true)
			return m, nil
		}
		return m, m.createScript(slug)
	}
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m *model) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		script := m.pendingDelete
		if script == nil {
			m.mode = modeBrowse
			return m, nil
		}
		m.pendingDelete = nil
		m.setStatus(fmt.Sprintf("Removing %s", script.Name), true)
		return m, m.deleteScript(script)
	case "n", "esc":
		m.mode = modeBrowse
		m.pendingDelete = nil
		m.setStatus("Browse mode", false)
		return m, nil
	}
	return m, nil
}

func (m *model) handleCommandChoiceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	script := m.pendingEdit
	if script == nil {
		m.mode = modeBrowse
		return m, nil
	}
	switch msg.String() {
	case "e":
		m.pendingEdit = nil
		m.mode = modeBrowse
		return m.startEditCommandFile(script, skit.ToggleActionEnable)
	case "d":
		m.pendingEdit = nil
		m.mode = modeBrowse
		return m.startEditCommandFile(script, skit.ToggleActionDisable)
	case "esc":
		m.pendingEdit = nil
		m.mode = modeBrowse
		m.setStatus("Browse mode", false)
		return m, nil
	default:
		return m, nil
	}
}

func (m *model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	historyLen := len(m.historyForSlug())
	switch msg.String() {
	case "esc", "h":
		m.mode = modeBrowse
		m.historyIdx = 0
		m.setStatus("Browse mode", false)
		return m, nil
	case "up", "k":
		if m.historyIdx < historyLen {
			m.historyIdx++
		}
		return m, nil
	case "down", "j":
		if m.historyIdx > 0 {
			m.historyIdx--
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *model) recomputeMatches() {
	query := strings.TrimSpace(m.search.Value())
	if query == "" {
		m.rebuildMatches()
		return
	}
	source := scriptSource{scripts: m.scripts}
	results := fuzzy.FindFrom(query, source)
	m.matches = m.matches[:0]
	for _, r := range results {
		m.matches = append(m.matches, r.Index)
	}
	if len(m.matches) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.matches) {
		m.cursor = len(m.matches) - 1
	}
}

func (m model) currentScript() *skit.Script {
	if len(m.matches) == 0 {
		return nil
	}
	if m.cursor < 0 || m.cursor >= len(m.matches) {
		return nil
	}
	idx := m.matches[m.cursor]
	if idx < 0 || idx >= len(m.scripts) {
		return nil
	}
	return m.scripts[idx]
}

func (m *model) focusSlug(slug string) {
	for i, s := range m.scripts {
		if s.Slug == slug {
			for pos, idx := range m.matches {
				if idx == i {
					m.cursor = pos
					return
				}
			}
		}
	}
}

func (m *model) startRun(script *skit.Script) (tea.Model, tea.Cmd) {
	if script == nil {
		return m, nil
	}
	m.running = true
	action := skit.ToggleAction("")
	if script.Type == skit.ScriptTypeToggle {
		action = skit.NextToggleAction(m.toggleCache[script.Slug])
	}
	label := fmt.Sprintf("Running %s", script.Name)
	if action != "" {
		label += fmt.Sprintf(" (%s)", action)
	}
	m.setStatus(label, false)
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		return runMsg{result: m.runner.Execute(ctx, script)}
	}
}

func (m *model) editScript(script *skit.Script) tea.Cmd {
	editor := resolveEditor()
	args := append(editor[1:], script.ConfigPath)
	cmdName := editor[0]
	m.setStatus(fmt.Sprintf("Opening %s", script.ConfigPath), false)
	dir := m.scriptsDir
	slug := script.Slug
	return tea.ExecProcess(execCommand(cmdName, args...), func(err error) tea.Msg {
		if err != nil {
			return scriptsUpdatedMsg{err: err}
		}
		scripts, loadErr := skit.LoadScripts(dir)
		return scriptsUpdatedMsg{scripts: scripts, err: loadErr, focusSlug: slug}
	})
}

func (m *model) startEditCommandFile(script *skit.Script, action skit.ToggleAction) (tea.Model, tea.Cmd) {
	path, err := commandPathFor(script, action)
	if err != nil {
		m.setStatus(err.Error(), true)
		return m, nil
	}
	m.setStatus(fmt.Sprintf("Editing %s", filepath.Base(path)), false)
	return m, editCommandFile(path)
}

func execCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (m *model) createScript(slug string) tea.Cmd {
	slug = sanitizeSlug(slug)
	if slug == "" {
		return func() tea.Msg {
			return scriptsUpdatedMsg{err: errors.New("invalid slug")}
		}
	}
	dir := filepath.Join(m.scriptsDir, slug)
	runPath := filepath.Join(dir, "run.sh")
	template := fmt.Sprintf(`name = "%s"
type = "run"

[exec]
default = "./run.sh"
`, displayName(slug))
	shell := "#!/usr/bin/env bash\n\necho \"[skit] " + slug + "\"\n"
	return func() tea.Msg {
		if err := os.Mkdir(dir, 0o755); err != nil {
			if errors.Is(err, os.ErrExist) {
				return scriptsUpdatedMsg{err: fmt.Errorf("script %s already exists", slug)}
			}
			return scriptsUpdatedMsg{err: err}
		}
		if err := os.WriteFile(filepath.Join(dir, skit.ConfigFileName), []byte(template), 0o644); err != nil {
			return scriptsUpdatedMsg{err: err}
		}
		if err := os.WriteFile(runPath, []byte(shell), 0o755); err != nil {
			return scriptsUpdatedMsg{err: err}
		}
		scripts, err := skit.LoadScripts(m.scriptsDir)
		return scriptsUpdatedMsg{scripts: scripts, err: err, focusSlug: slug}
	}
}

func (m *model) deleteScript(script *skit.Script) tea.Cmd {
	dir := script.Dir
	slug := script.Slug
	state := m.stateStore
	scriptsDir := m.scriptsDir
	return func() tea.Msg {
		if err := os.RemoveAll(dir); err != nil {
			return scriptsUpdatedMsg{err: err}
		}
		if state != nil {
			if err := state.Delete(slug); err != nil {
				return scriptsUpdatedMsg{err: err}
			}
		}
		scripts, err := skit.LoadScripts(scriptsDir)
		return scriptsUpdatedMsg{scripts: scripts, err: err}
	}
}

func resolveEditor() []string {
	for _, key := range []string{"SK_EDITOR", "VISUAL", "EDITOR"} {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return strings.Fields(val)
		}
	}
	return []string{"vi"}
}

func sanitizeSlug(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ':
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	return slug
}

func displayName(slug string) string {
	if slug == "" {
		return "New Script"
	}
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

type scriptSource struct {
	scripts []*skit.Script
}

func (s scriptSource) Len() int {
	return len(s.scripts)
}

func (s scriptSource) String(i int) string {
	if i < 0 || i >= len(s.scripts) {
		return ""
	}
	entry := s.scripts[i]
	fields := []string{entry.Name, entry.Slug, strings.Join(entry.Tags, " ")}
	return strings.ToLower(strings.TrimSpace(strings.Join(fields, " ")))
}

type commandEditMsg struct {
	err error
}

func editCommandFile(path string) tea.Cmd {
	editor := resolveEditor()
	args := append(editor[1:], path)
	cmd := execCommand(editor[0], args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return commandEditMsg{err: err}
	})
}

func commandPathFor(script *skit.Script, action skit.ToggleAction) (string, error) {
	cmd, err := script.ResolveCommand(action)
	if err != nil {
		return "", err
	}
	if cmd.Inline {
		return "", fmt.Errorf("inline commands must be edited in the manifest")
	}
	if cmd.Value == "" {
		return "", fmt.Errorf("command is empty")
	}
	return cmd.Value, nil
}

func (m model) historyFor(slug string) []skit.RunRecord {
	if m.stateStore == nil || slug == "" {
		return nil
	}
	state, err := m.stateStore.Load(slug)
	if err != nil {
		return nil
	}
	return state.Runs
}

func (m model) historyForSlug() []skit.RunRecord {
	script := m.currentScript()
	if script == nil {
		return nil
	}
	h := m.historyFor(script.Slug)
	if m.historyIdx >= len(h) {
		return h
	}
	return h
}
