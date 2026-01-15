package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/turbopuffer/turbopuffer-go"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}
	header := m.viewHeader()
	footer := m.viewFooter()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 3 {
		return header + "\n" + footer
	}

	leftWidth := m.width / 3
	if leftWidth < 24 {
		leftWidth = 24
	}
	if leftWidth > 40 {
		leftWidth = 40
	}
	rightWidth := m.width - leftWidth

	left := m.viewNamespaces(leftWidth, bodyHeight)
	right := m.viewRightPanel(rightWidth, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Top, header, body, footer)
}

func (m model) viewHeader() string {
	ns := m.namespace
	if ns == "" {
		ns = "-"
	}
	profile := m.profileName
	if profile == "" {
		profile = "-"
	}
	title := headerStyle.Render("tpuf")
	meta := subtleStyle.Render(fmt.Sprintf("profile:%s  %s  ns:%s", profile, m.profile.DisplayRegion(), ns))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", meta)
}

func (m model) viewFooter() string {
	status := m.status
	if status == "" {
		status = "Ready"
	}
	if m.statusErr {
		status = errorStyle.Render(status)
	}
	help := subtleStyle.Render("q quit  tab focus  / filter/query  enter select  r refresh  t/v mode  g get id  p profile")
	return lipgloss.JoinHorizontal(lipgloss.Top, status, "  ", help)
}

func (m model) viewNamespaces(width, height int) string {
	title := titleStyle.Render("Namespaces")
	filter := m.nsFilter.View()
	if m.inputMode != inputNamespaceFilter {
		if strings.TrimSpace(m.nsFilter.Value()) == "" {
			filter = subtleStyle.Render("(/) filter")
		} else {
			filter = subtleStyle.Render(fmt.Sprintf("filter: %s", m.nsFilter.Value()))
		}
	}

	lines := make([]string, 0, height)
	lines = append(lines, title)
	lines = append(lines, filter)

	listHeight := height - 3
	if listHeight < 0 {
		listHeight = 0
	}

	items := make([]string, 0, len(m.nsMatches))
	for _, idx := range m.nsMatches {
		items = append(items, m.namespaces[idx].ID)
	}
	listWidth := width - 2
	list, start, end := renderList(items, m.nsCursor, listHeight, listWidth, m.focus == focusNamespaces)
	lines = append(lines, list...)
	lines = append(lines, listStatus(start, end, len(items)))

	panel := strings.Join(lines, "\n")
	style := blurredBorder
	if m.focus == focusNamespaces {
		style = focusedBorder
	}
	return style.Width(width).Height(height).Render(panel)
}

func (m model) viewRightPanel(width, height int) string {
	var panel string
	switch m.activePane {
	case paneSchema:
		panel = m.viewSchema(width, height)
	case paneMeta:
		panel = m.viewMeta(width, height)
	default:
		panel = m.viewDocs(width, height)
	}
	style := blurredBorder
	if m.focus == focusDocs {
		style = focusedBorder
	}
	return style.Width(width).Height(height).Render(panel)
}

func (m model) viewDocs(width, height int) string {
	tabs := m.viewTabs()
	queryLabel := "query"
	if m.queryMode == queryVector {
		queryLabel = "vector"
	}
	query := m.queryInput.View()
	if m.inputMode != inputQuery {
		if strings.TrimSpace(m.queryInput.Value()) == "" {
			query = subtleStyle.Render(fmt.Sprintf("(/) %s search", queryLabel))
		} else {
			query = subtleStyle.Render(fmt.Sprintf("%s: %s", queryLabel, m.queryInput.Value()))
		}
	}
	ns := m.namespace
	if ns == "" {
		ns = "-"
	}
	contextLine := subtleStyle.Render(fmt.Sprintf("ns: %s  mode: %s  top_k: %d", ns, queryLabel, m.profile.TopK))
	header := lipgloss.JoinVertical(lipgloss.Top, tabs, contextLine, query)

	listHeight := height / 2
	if listHeight < 4 {
		listHeight = 4
	}
	maxListHeight := height - 8
	if maxListHeight < 4 {
		maxListHeight = 4
	}
	if listHeight > maxListHeight {
		listHeight = maxListHeight
	}
	detailHeight := height - listHeight - 3
	if detailHeight < 0 {
		detailHeight = 0
	}

	listWidth := width - 2
	rows := make([]string, 0, len(m.docs))
	for _, row := range m.docs {
		rows = append(rows, rowSummary(row, m.profile.TextAttr, m.profile.VectorAttr, listWidth))
	}
	list, start, end := renderList(rows, m.docsCursor, listHeight, listWidth, m.focus == focusDocs)
	listFooter := listStatus(start, end, len(rows))

	detail := ""
	if m.inputMode == inputDocID {
		detail = m.docIDInput.View()
	} else {
		detail = m.docDetail(width-2, detailHeight)
	}

	parts := []string{header, strings.Join(list, "\n"), listFooter, detail}
	return strings.Join(parts, "\n")
}

func (m model) docDetail(width, height int) string {
	if len(m.docs) == 0 || m.docsCursor >= len(m.docs) {
		return subtleStyle.Render("No document selected")
	}
	row := m.docs[m.docsCursor]
	content := prettyJSON(sanitizeRow(row, m.profile.VectorAttr))
	if content == "" {
		return subtleStyle.Render("Empty document")
	}
	if width < 10 {
		return content
	}
	style := lipgloss.NewStyle().Width(width).Height(height)
	return style.Render(content)
}

func (m model) viewSchema(width, height int) string {
	tabs := m.viewTabs()
	body := m.schemaRendered
	if body == "" {
		body = subtleStyle.Render("No schema loaded")
	}
	content := renderScrollable(body, width, height-1, m.schemaScroll)
	return strings.Join([]string{tabs, content}, "\n")
}

func (m model) viewMeta(width, height int) string {
	tabs := m.viewTabs()
	body := m.metaRendered
	if body == "" {
		body = subtleStyle.Render("No metadata loaded")
	}
	content := renderScrollable(body, width, height-1, m.metaScroll)
	return strings.Join([]string{tabs, content}, "\n")
}

func (m model) viewTabs() string {
	titles := []string{"Docs", "Schema", "Meta"}
	var out []string
	for i, title := range titles {
		if pane(i) == m.activePane {
			out = append(out, selectedStyle.Render(title))
		} else {
			out = append(out, subtleStyle.Render(title))
		}
	}
	return strings.Join(out, " ")
}

func renderList(items []string, cursor int, height int, width int, focused bool) ([]string, int, int) {
	if height <= 0 {
		return nil, 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(items) {
		cursor = len(items) - 1
	}
	start := 0
	if cursor >= height {
		start = cursor - height + 1
	}
	end := start + height
	if end > len(items) {
		end = len(items)
	}

	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		label := items[i]
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		line := prefix + label
		if width > 0 {
			line = truncate(line, width)
		}
		if i == cursor {
			if focused {
				line = selectedStyle.Render(line)
			} else {
				line = lipgloss.NewStyle().Bold(true).Render(line)
			}
		}
		lines = append(lines, line)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines, start, end
}

func listStatus(start int, end int, total int) string {
	if total == 0 {
		return subtleStyle.Render("0 items")
	}
	return subtleStyle.Render(fmt.Sprintf("%d-%d of %d", start+1, end, total))
}

func renderScrollable(content string, width int, height int, offset int) string {
	if height <= 0 {
		return ""
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(content)
	lines := strings.Split(wrapped, "\n")
	if offset < 0 {
		offset = 0
	}
	maxOffset := 0
	if len(lines) > height {
		maxOffset = len(lines) - height
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	out := strings.Join(lines[offset:end], "\n")
	style := lipgloss.NewStyle().Width(width).Height(height)
	return style.Render(out)
}

func rowSummary(row turbopuffer.Row, textAttr string, vectorAttr string, width int) string {
	id := rowID(row)
	dist := ""
	if value, ok := row["$dist"]; ok {
		dist = fmt.Sprintf("dist:%v", value)
	}
	preview := rowPreview(row, textAttr, vectorAttr)
	parts := make([]string, 0, 3)
	parts = append(parts, id)
	if dist != "" {
		parts = append(parts, dist)
	}
	if preview != "" {
		parts = append(parts, preview)
	}
	line := strings.Join(parts, " | ")
	return truncate(line, width)
}

func rowID(row turbopuffer.Row) string {
	if value, ok := row["id"]; ok {
		return fmt.Sprint(value)
	}
	if value, ok := row["$id"]; ok {
		return fmt.Sprint(value)
	}
	if value, ok := row["_id"]; ok {
		return fmt.Sprint(value)
	}
	return "<no id>"
}

func rowPreview(row turbopuffer.Row, textAttr string, vectorAttr string) string {
	if textAttr != "" {
		if value, ok := row[textAttr]; ok {
			if str, ok := value.(string); ok {
				return collapseWhitespace(str)
			}
		}
	}
	for key, value := range row {
		if key == "id" || key == "$id" || key == "_id" {
			continue
		}
		if key == vectorAttr || key == "vector" || key == "$vector" {
			continue
		}
		switch v := value.(type) {
		case string:
			return collapseWhitespace(v)
		case fmt.Stringer:
			return v.String()
		case float64, float32, int, int64, bool:
			return fmt.Sprint(v)
		}
	}
	return ""
}

func sanitizeRow(row turbopuffer.Row, vectorAttr string) map[string]any {
	clean := make(map[string]any, len(row))
	for key, value := range row {
		if key == vectorAttr || key == "vector" || key == "$vector" {
			clean[key] = summarizeVector(value)
			continue
		}
		if isVectorLike(value) {
			clean[key] = summarizeVector(value)
			continue
		}
		clean[key] = value
	}
	return clean
}

func summarizeVector(value any) string {
	switch v := value.(type) {
	case []float32:
		return fmt.Sprintf("<vector elided len=%d>", len(v))
	case []float64:
		return fmt.Sprintf("<vector elided len=%d>", len(v))
	case []any:
		return fmt.Sprintf("<vector elided len=%d>", len(v))
	default:
		return "<vector elided>"
	}
}

func isVectorLike(value any) bool {
	switch v := value.(type) {
	case []float32:
		return len(v) >= 16
	case []float64:
		return len(v) >= 16
	case []any:
		if len(v) < 16 {
			return false
		}
		for _, item := range v {
			switch item.(type) {
			case float64, float32, int, int64:
				continue
			default:
				return false
			}
		}
		return true
	default:
		return false
	}
}

func collapseWhitespace(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

func truncate(value string, max int) string {
	if max <= 0 {
		return value
	}
	if len(value) <= max {
		return value
	}
	if max <= 1 {
		return value[:max]
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}
