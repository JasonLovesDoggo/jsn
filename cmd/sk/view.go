package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"pkg.jsn.cam/jsn/internal/skit"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#0EA5E9"))
	runBadge      = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	toggleBadge   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	statusOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	statusErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	footerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#475569")).Padding(0, 1)
)

const maxVisibleRows = 10

func (m model) View() string {
	if m.mode == modeHistory {
		return renderHistoryView(m)
	}
	return renderBrowseView(m)
}

func windowStart(cursor, total, window int) int {
	if total <= window {
		return 0
	}
	start := cursor - window/2
	if start < 0 {
		start = 0
	}
	if start+window > total {
		start = total - window
	}
	return start
}

func renderBrowseView(m model) string {
	if len(m.scripts) == 0 {
		return fmt.Sprintf("scriptkit\n\nNo scripts found. Add one under %s/<slug>/skit.toml\n", m.scriptsDir)
	}
	var b strings.Builder
	header := titleStyle.Render("scriptkit")
	meta := mutedStyle.Render(fmt.Sprintf(" · %d scripts", len(m.scripts)))
	if m.running {
		meta += mutedStyle.Render(" · running…")
	}
	b.WriteString(header + meta + "\n")
	if m.mode != modeNew {
		b.WriteString(m.search.View())
		b.WriteString("\n\n")
	} else {
		b.WriteString("New script\n")
		b.WriteString(m.prompt.View())
		b.WriteString("\n\n")
	}
	if len(m.matches) == 0 {
		b.WriteString(mutedStyle.Render("No matches") + "\n")
	} else {
		start := windowStart(m.cursor, len(m.matches), maxVisibleRows)
		end := start + maxVisibleRows
		if end > len(m.matches) {
			end = len(m.matches)
		}
		for pos := start; pos < end; pos++ {
			idx := m.matches[pos]
			s := m.scripts[idx]
			selected := pos == m.cursor
			b.WriteString(renderScriptLine(s, selected, m.toggleCache[s.Slug]) + "\n")
			if s.Description != "" {
				desc := "  " + mutedStyle.Render(s.Description)
				if selected {
					desc = selectedStyle.Render(desc)
				}
				b.WriteString(desc + "\n")
			}
		}
		if end < len(m.matches) {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("…%d more\n", len(m.matches)-end)))
		} else if start > 0 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("…%d above\n", start)))
		}
	}
	b.WriteString("\n")
	b.WriteString(renderStatus(m.status, m.statusErr))
	b.WriteString("\n")
	b.WriteString(renderFooter(m))
	b.WriteString("\n")
	if m.showDetails && m.lastResult != nil {
		b.WriteString(borderStyle.Render(renderDetails(m.lastResult)) + "\n")
	}
	return b.String()
}

func renderHistoryView(m model) string {
	script := m.currentScript()
	if script == nil {
		return "History\n\nNo script selected."
	}
	var b strings.Builder
	title := titleStyle.Render(fmt.Sprintf("History · %s", script.Name))
	b.WriteString(title + "\n")
	history := m.history[script.Slug]
	if len(history) == 0 {
		b.WriteString("No runs recorded for this script yet.\n")
		b.WriteString("\n" + renderFooter(m) + "\n")
		return b.String()
	}
	maxRows := 15
	start := len(history) - maxRows
	if start < 0 {
		start = 0
	}
	for i := start; i < len(history); i++ {
		run := history[i]
		status := statusOK.Render("OK")
		if !run.Success {
			status = statusErr.Render("ERR")
		}
		b.WriteString(fmt.Sprintf("%s %s (%s)\n", run.Time.Format("2006-01-02 15:04:05"), status, strings.ToUpper(run.Action)))
		if run.Output != "" {
			lines := strings.Split(strings.TrimSpace(run.Output), "\n")
			for _, line := range lines {
				b.WriteString("  " + line + "\n")
			}
		}
		if run.Err != "" {
			b.WriteString("  err: " + run.Err + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(renderFooter(m) + "\n")
	return b.String()
}

func renderScriptLine(s *skit.Script, selected bool, lastAction skit.ToggleAction) string {
	cursor := " "
	if selected {
		cursor = selectedStyle.Render("›")
	}
	badge := renderBadge(s)
	line := fmt.Sprintf("%s %s %s", cursor, badge, s.Name)
	if s.Type == skit.ScriptTypeToggle {
		next := skit.NextToggleAction(lastAction)
		line += mutedStyle.Render(fmt.Sprintf(" · next: %s", strings.ToUpper(string(next))))
	}
	if !s.SupportsCurrentPlatform() {
		line += statusErr.Render(" · not for this platform")
	}
	if selected {
		return selectedStyle.Render(line)
	}
	return line
}

func renderBadge(s *skit.Script) string {
	if s.Type == skit.ScriptTypeToggle {
		return toggleBadge.Render("[toggle]")
	}
	return runBadge.Render("[run]")
}

func renderFooter(m model) string {
	switch m.mode {
	case modeAction:
		return footerStyle.Render("Action: e edit manifest  o edit script  n new  d delete  ← exit")
	case modeNew:
		return footerStyle.Render("Enter slug, press Enter to create or Esc to cancel")
	case modeConfirmDelete:
		return footerStyle.Render("Delete? y/n (Esc cancels)")
	case modeChooseCommand:
		return footerStyle.Render("Toggle edit: e enable  d disable  Esc cancel")
	case modeHistory:
		return footerStyle.Render("History: h/Esc close")
	default:
		return footerStyle.Render("↑/↓ move  enter run  ctrl+r rerun  h history  → actions  ? output  ctrl+c quit")
	}
}

func renderStatus(status string, isErr bool) string {
	if status == "" {
		return ""
	}
	if isErr {
		return statusErr.Render(status)
	}
	return statusOK.Render(status)
}

func renderDetails(res *skit.RunResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	title := fmt.Sprintf("%s · %s", res.Script.Name, res.Duration.Round(10*time.Millisecond))
	if res.Err != nil {
		title += fmt.Sprintf(" · %v", res.Err)
	}
	b.WriteString(title + "\n")
	out := strings.TrimSpace(res.Output)
	if out == "" {
		b.WriteString("(no output)")
		return b.String()
	}
	for _, line := range strings.Split(out, "\n") {
		b.WriteString("  " + line + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
