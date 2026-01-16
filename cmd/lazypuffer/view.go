package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/turbopuffer/turbopuffer-go"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}
	header := wrapBlock(m.viewHeader(), m.width)
	footer := wrapBlock(m.viewFooter(), m.width)
	headerHeight := lipgloss.Height(header)
	if headerHeight >= m.height {
		return clipToHeight(header, m.height)
	}
	footerHeight := lipgloss.Height(footer)
	bodyHeight := m.height - headerHeight - footerHeight
	if bodyHeight < 1 {
		footer = clipToHeight(footer, m.height-headerHeight)
		return header + "\n" + footer
	}

	leftWidth, rightWidth := panelWidths(m.width)

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
	title := headerStyle.Render("lazypuffer")
	meta := subtleStyle.Render(fmt.Sprintf("profile:%s  %s  ns:%s", profile, m.profile.DisplayRegion(), ns))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", meta)
}

func (m model) viewFooter() string {
	if m.inputMode == inputProfileName {
		prompt := m.profileInput.View()
		help := subtleStyle.Render("enter to create, esc to cancel")
		return lipgloss.JoinHorizontal(lipgloss.Top, prompt, "  ", help)
	}
	if m.inputMode == inputFilter {
		prompt := m.filterInput.View()
		help := subtleStyle.Render("enter to apply, esc to cancel")
		return lipgloss.JoinHorizontal(lipgloss.Top, prompt, "  ", help)
	}
	status := m.status
	if status == "" {
		status = "Ready"
	}
	if m.statusErr {
		status = errorStyle.Render(status)
	}
	help := subtleStyle.Render("q quit  tab focus  / query  f filter  enter select  r refresh  t/v mode  g get id  y copy  p profile  c config")
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
	if height >= 3 {
		lines = append(lines, title)
		lines = append(lines, filter)
	} else if height == 2 {
		lines = append(lines, title)
	} else {
		title = ""
		filter = ""
	}

	available := height - len(lines)
	if available < 0 {
		available = 0
	}
	listHeight := available
	showStatus := false
	if available >= 2 {
		showStatus = true
		listHeight = available - 1
	}

	items := make([]string, 0, len(m.nsMatches))
	for _, idx := range m.nsMatches {
		items = append(items, m.namespaces[idx].ID)
	}
	listWidth := width - 2
	list, start, end := renderList(items, m.nsCursor, listHeight, listWidth, m.focus == focusNamespaces)
	lines = append(lines, list...)
	if showStatus {
		status := listStatus(start, end, len(items))
		if listWidth > 0 {
			status = truncate(status, listWidth)
		}
		lines = append(lines, status)
	}

	panel := strings.Join(lines, "\n")
	panel = clipToHeight(panel, height)
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
		panel = m.viewDocsSplit(width, height)
	}
	panel = clipToHeight(panel, height)
	if m.activePane == paneDocs {
		return panel
	}
	style := blurredBorder
	if m.focus != focusNamespaces {
		style = focusedBorder
	}
	return style.Width(width).Height(height).Render(panel)
}

func (m model) viewDocsSplit(width, height int) string {
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
	contextLine := subtleStyle.Render(fmt.Sprintf("ns: %s  mode: %s  top_k: %d  focus: %s", ns, queryLabel, m.profile.TopK, m.focusLabel()))
	rankLine := subtleStyle.Render(m.rankSummary())
	filterLine := subtleStyle.Render(m.filterSummary())
	header := lipgloss.JoinVertical(lipgloss.Top, tabs, contextLine, rankLine, filterLine, query)
	header = wrapBlock(header, width)

	headerHeight := lipgloss.Height(header)
	topPaneHeight, bottomPaneHeight := splitPaneHeights(height)
	contentHeight := maxInt(0, topPaneHeight-2)
	listRows := maxInt(1, contentHeight-headerHeight-1)

	listWidth := width - 2
	rows := make([]string, 0, len(m.docs))
	for _, row := range m.docs {
		rows = append(rows, rowSummary(row, m.profile.TextAttr, m.profile.VectorAttr, listWidth))
	}
	list, start, end := renderList(rows, m.docsCursor, listRows, listWidth, m.focus == focusDocsList)
	listFooter := listStatus(start, end, len(rows))
	if listWidth > 0 {
		listFooter = truncate(listFooter, listWidth)
	}
	listContent := strings.Join([]string{header, strings.Join(list, "\n"), listFooter}, "\n")
	listContent = clipToHeight(listContent, maxInt(0, topPaneHeight-2))
	listStyle := blurredBorder
	if m.focus == focusDocsList {
		listStyle = focusedBorder
	}
	listStyle = listStyle.BorderBottom(true)
	listPane := listStyle.Width(width).Height(topPaneHeight).Render(listContent)

	detailContentHeight := maxInt(0, bottomPaneHeight-2)
	detailHeader := ""
	detailBody := ""
	bodyHeight := detailContentHeight
	if detailContentHeight > 0 {
		bodyHeight = maxInt(0, detailContentHeight-1)
		if m.inputMode == inputDocID {
			detailHeader = m.docIDInput.View()
		} else {
			label := "Detail"
			if width > 2 {
				label = truncate(label, width-2)
			}
			if m.focus == focusDocsDetail {
				detailHeader = selectedStyle.Render(label)
			} else {
				detailHeader = subtleStyle.Render(label)
			}
		}
	}
	if m.inputMode != inputDocID {
		detailBody = m.docDetail(width-2, bodyHeight, m.detailScroll)
	}
	detailContent := strings.TrimRight(strings.Join([]string{detailHeader, detailBody}, "\n"), "\n")
	detailContent = clipToHeight(detailContent, detailContentHeight)
	detailStyle := blurredBorder
	if m.focus == focusDocsDetail {
		detailStyle = focusedBorder
	}
	detailStyle = detailStyle.BorderTop(false)
	detailPane := detailStyle.Width(width).Height(bottomPaneHeight).Render(detailContent)

	return lipgloss.JoinVertical(lipgloss.Top, listPane, detailPane)
}

func (m model) docDetail(width, height int, offset int) string {
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
	return renderScrollable(content, width, height, offset)
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
	body := "No metadata loaded"
	if m.meta != nil {
		body = renderMetaSummary(m.meta)
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

// layout helpers live in layout.go

func (m model) rankSummary() string {
	if m.queryMode == queryVector {
		attr := m.profile.VectorAttr
		if attr == "" {
			attr = "vector"
		}
		return fmt.Sprintf("rank: vector(%s)", attr)
	}
	if strings.TrimSpace(m.queryInput.Value()) == "" {
		return "rank: id asc"
	}
	attr := m.profile.TextAttr
	if attr == "" {
		attr = "text"
	}
	return fmt.Sprintf("rank: bm25(%s)", attr)
}

func (m model) filterSummary() string {
	raw := strings.TrimSpace(m.filterRaw)
	if raw == "" {
		return "filters: none"
	}
	return "filters: " + raw
}

func renderMetaSummary(meta *turbopuffer.NamespaceMetadata) string {
	if meta == nil {
		return ""
	}
	lines := []string{
		headerStyle.Render("Namespace metadata"),
		"",
		fmt.Sprintf("rows: %s", humanize.Comma(meta.ApproxRowCount)),
		fmt.Sprintf("logical bytes: %s", humanize.Bytes(uint64(meta.ApproxLogicalBytes))),
		fmt.Sprintf("created: %s", meta.CreatedAt.Format(time.RFC3339)),
		fmt.Sprintf("updated: %s", meta.UpdatedAt.Format(time.RFC3339)),
	}
	enc := "sse"
	if meta.Encryption.Cmek.KeyName != "" {
		enc = "cmek"
	}
	lines = append(lines, fmt.Sprintf("encryption: %s", enc))
	if meta.Encryption.Cmek.KeyName != "" {
		lines = append(lines, fmt.Sprintf("cmek key: %s", meta.Encryption.Cmek.KeyName))
	}
	if meta.Index.Status != "" {
		lines = append(lines, fmt.Sprintf("index status: %s", meta.Index.Status))
	}
	if meta.Index.UnindexedBytes > 0 {
		lines = append(lines, fmt.Sprintf("unindexed bytes: %s", humanize.Bytes(uint64(meta.Index.UnindexedBytes))))
	}
	if len(meta.Schema) > 0 {
		vecCount := 0
		for _, cfg := range meta.Schema {
			if isVectorType(cfg.Type) {
				vecCount++
			}
		}
		lines = append(lines, fmt.Sprintf("attributes: %d (vector: %d)", len(meta.Schema), vecCount))
	}
	return strings.Join(lines, "\n")
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
