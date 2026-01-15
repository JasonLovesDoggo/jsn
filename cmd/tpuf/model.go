package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"github.com/turbopuffer/turbopuffer-go"
)

type pane int

const (
	paneDocs pane = iota
	paneSchema
	paneMeta
)

type focusArea int

const (
	focusNamespaces focusArea = iota
	focusDocs
)

type inputMode int

const (
	inputNone inputMode = iota
	inputNamespaceFilter
	inputQuery
	inputDocID
)

type queryMode int

const (
	queryText queryMode = iota
	queryVector
)

type model struct {
	width          int
	height         int
	config         *Config
	profileName    string
	profile        Profile
	profileNames   []string
	profileIndex   int
	client         *turbopuffer.Client
	activePane     pane
	focus          focusArea
	inputMode      inputMode
	status         string
	statusErr      bool
	loading        bool
	namespaces     []turbopuffer.NamespaceSummary
	nsMatches      []int
	nsCursor       int
	namespace      string
	nsFilter       textinput.Model
	queryMode      queryMode
	queryInput     textinput.Model
	docIDInput     textinput.Model
	docs           []turbopuffer.Row
	docsCursor     int
	meta           *turbopuffer.NamespaceMetadata
	metaRendered   string
	schema         *turbopuffer.NamespaceSchemaResponse
	schemaRendered string
	metaScroll     int
	schemaScroll   int
}

type namespacesMsg struct {
	items []turbopuffer.NamespaceSummary
	err   error
}

type metadataMsg struct {
	meta *turbopuffer.NamespaceMetadata
	err  error
}

type schemaMsg struct {
	schema *turbopuffer.NamespaceSchemaResponse
	err    error
}

type docsMsg struct {
	rows []turbopuffer.Row
	err  error
}

type switchProfileMsg struct {
	name    string
	index   int
	profile Profile
}

type statusMsg struct {
	text string
	err  bool
}

func NewModel(cfg *Config, profileName string, profile Profile) model {
	nsFilter := textinput.New()
	nsFilter.Placeholder = "Filter namespaces"
	nsFilter.CharLimit = 256
	nsFilter.Prompt = "/ "
	nsFilter.Blur()

	queryInput := textinput.New()
	queryInput.Placeholder = "Search text"
	queryInput.CharLimit = 2048
	queryInput.Prompt = "/ "
	queryInput.Blur()

	docIDInput := textinput.New()
	docIDInput.Placeholder = "Document ID"
	docIDInput.CharLimit = 512
	docIDInput.Prompt = "id> "
	docIDInput.Blur()

	names := cfg.ProfileNames()
	index := 0
	for i, name := range names {
		if name == profileName {
			index = i
			break
		}
	}

	m := model{
		config:       cfg,
		profileName:  profileName,
		profile:      profile,
		profileNames: names,
		profileIndex: index,
		client:       NewClient(profile),
		activePane:   paneDocs,
		focus:        focusNamespaces,
		inputMode:    inputNone,
		nsFilter:     nsFilter,
		queryInput:   queryInput,
		docIDInput:   docIDInput,
		queryMode:    queryText,
		namespace:    profile.Namespace,
	}
	m.setStatus("Loading namespaces...", false)
	return m
}

func (m model) Init() tea.Cmd {
	return m.fetchNamespacesCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case namespacesMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("namespaces: %v", msg.err), true)
			return m, nil
		}
		m.namespaces = msg.items
		m.rebuildNamespaceMatches()
		if m.namespace == "" {
			m.namespace = m.firstNamespace()
		}
		if m.namespace != "" {
			m.nsCursor = m.namespaceIndex(m.namespace)
		}
		m.setStatus(fmt.Sprintf("Loaded %d namespaces", len(m.namespaces)), false)
		return m, tea.Batch(
			m.fetchMetadataCmd(),
			m.fetchSchemaCmd(),
			m.fetchDocsCmd(),
		)
	case metadataMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("metadata: %v", msg.err), true)
			return m, nil
		}
		m.meta = msg.meta
		m.metaRendered = prettyJSON(msg.meta)
		m.metaScroll = 0
		return m, nil
	case schemaMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("schema: %v", msg.err), true)
			return m, nil
		}
		m.schema = msg.schema
		m.schemaRendered = prettyJSON(msg.schema)
		m.schemaScroll = 0
		return m, nil
	case docsMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("query: %v", msg.err), true)
			return m, nil
		}
		m.docs = msg.rows
		m.docsCursor = 0
		m.setStatus(fmt.Sprintf("Docs: %d result(s)", len(m.docs)), false)
		return m, nil
	case switchProfileMsg:
		m.profileName = msg.name
		m.profileIndex = msg.index
		m.profile = msg.profile
		m.client = NewClient(msg.profile)
		m.namespace = msg.profile.Namespace
		m.nsFilter.SetValue("")
		m.rebuildNamespaceMatches()
		m.setStatus(fmt.Sprintf("Profile: %s", msg.name), false)
		return m, m.fetchNamespacesCmd()
	case statusMsg:
		m.setStatus(msg.text, msg.err)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) setStatus(text string, isErr bool) {
	m.status = text
	m.statusErr = isErr
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.inputMode != inputNone {
		return m.handleInputKey(msg)
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "tab":
		m.toggleFocus()
		return m, nil
	case "right":
		m.activePane = (m.activePane + 1) % 3
		return m, nil
	case "left":
		m.activePane = (m.activePane + 2) % 3
		return m, nil
	case "d":
		m.activePane = paneDocs
		return m, nil
	case "s":
		m.activePane = paneSchema
		m.schemaScroll = 0
		return m, nil
	case "m":
		m.activePane = paneMeta
		m.metaScroll = 0
		return m, nil
	case "j", "down":
		if m.focus == focusDocs && m.activePane != paneDocs {
			m.scrollActivePane(1)
		} else {
			m.moveCursor(1)
		}
		return m, nil
	case "k", "up":
		if m.focus == focusDocs && m.activePane != paneDocs {
			m.scrollActivePane(-1)
		} else {
			m.moveCursor(-1)
		}
		return m, nil
	case "enter":
		if m.focus == focusNamespaces {
			ns := m.currentNamespace()
			if ns != "" && ns != m.namespace {
				m.namespace = ns
				m.setStatus(fmt.Sprintf("Namespace: %s", ns), false)
			}
			return m, tea.Batch(
				m.fetchMetadataCmd(),
				m.fetchSchemaCmd(),
				m.fetchDocsCmd(),
			)
		}
		return m, nil
	case "/":
		if m.focus == focusNamespaces {
			m.inputMode = inputNamespaceFilter
			m.nsFilter.Focus()
			return m, nil
		}
		if m.activePane == paneDocs {
			m.inputMode = inputQuery
			m.queryInput.Focus()
			return m, nil
		}
	case "g":
		if m.activePane == paneDocs {
			m.inputMode = inputDocID
			m.docIDInput.Focus()
			return m, nil
		}
	case "r":
		return m.handleRefresh()
	case "t":
		m.queryMode = queryText
		m.queryInput.Placeholder = "Search text"
		m.setStatus("Query mode: text", false)
		return m, nil
	case "v":
		m.queryMode = queryVector
		m.queryInput.Placeholder = "Vector (comma or JSON)"
		m.setStatus("Query mode: vector", false)
		return m, nil
	case "p":
		return m, m.cycleProfileCmd(1)
	}
	return m, nil
}

func (m model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.inputMode {
	case inputNamespaceFilter:
		if msg.String() == "esc" {
			m.nsFilter.Blur()
			m.inputMode = inputNone
			return m, nil
		}
		if msg.String() == "enter" {
			m.nsFilter.Blur()
			m.inputMode = inputNone
			m.rebuildNamespaceMatches()
			return m, nil
		}
		var cmd tea.Cmd
		m.nsFilter, cmd = m.nsFilter.Update(msg)
		return m, cmd
	case inputQuery:
		if msg.String() == "esc" {
			m.queryInput.Blur()
			m.inputMode = inputNone
			return m, nil
		}
		if msg.String() == "enter" {
			m.queryInput.Blur()
			m.inputMode = inputNone
			return m, m.fetchDocsCmd()
		}
		var cmd tea.Cmd
		m.queryInput, cmd = m.queryInput.Update(msg)
		return m, cmd
	case inputDocID:
		if msg.String() == "esc" {
			m.docIDInput.Blur()
			m.inputMode = inputNone
			return m, nil
		}
		if msg.String() == "enter" {
			id := strings.TrimSpace(m.docIDInput.Value())
			m.docIDInput.Blur()
			m.inputMode = inputNone
			if id == "" {
				return m, nil
			}
			return m, m.fetchDocByIDCmd(id)
		}
		var cmd tea.Cmd
		m.docIDInput, cmd = m.docIDInput.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m *model) toggleFocus() {
	if m.focus == focusNamespaces {
		m.focus = focusDocs
	} else {
		m.focus = focusNamespaces
	}
}

func (m *model) moveCursor(delta int) {
	if m.focus == focusNamespaces {
		if len(m.nsMatches) == 0 {
			m.nsCursor = 0
			return
		}
		m.nsCursor += delta
		if m.nsCursor < 0 {
			m.nsCursor = 0
		}
		if m.nsCursor >= len(m.nsMatches) {
			m.nsCursor = len(m.nsMatches) - 1
		}
		return
	}
	if m.activePane == paneDocs {
		if len(m.docs) == 0 {
			m.docsCursor = 0
			return
		}
		m.docsCursor += delta
		if m.docsCursor < 0 {
			m.docsCursor = 0
		}
		if m.docsCursor >= len(m.docs) {
			m.docsCursor = len(m.docs) - 1
		}
	}
}

func (m *model) scrollActivePane(delta int) {
	height := m.contentHeight()
	if height <= 1 {
		return
	}
	switch m.activePane {
	case paneSchema:
		maxOffset := maxScrollOffset(m.schemaRendered, height-1)
		m.schemaScroll = clampScroll(m.schemaScroll+delta, maxOffset)
	case paneMeta:
		maxOffset := maxScrollOffset(m.metaRendered, height-1)
		m.metaScroll = clampScroll(m.metaScroll+delta, maxOffset)
	}
}

func (m model) contentHeight() int {
	if m.height <= 2 {
		return 0
	}
	return m.height - 2
}

func (m model) handleRefresh() (tea.Model, tea.Cmd) {
	if m.focus == focusNamespaces {
		return m, m.fetchNamespacesCmd()
	}
	switch m.activePane {
	case paneDocs:
		return m, m.fetchDocsCmd()
	case paneSchema:
		return m, m.fetchSchemaCmd()
	case paneMeta:
		return m, m.fetchMetadataCmd()
	default:
		return m, nil
	}
}

func (m model) fetchNamespacesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var items []turbopuffer.NamespaceSummary
		pager := client.NamespacesAutoPaging(ctx, turbopuffer.NamespacesParams{})
		for pager.Next() {
			items = append(items, pager.Current())
		}
		if err := pager.Err(); err != nil {
			return namespacesMsg{err: err}
		}
		return namespacesMsg{items: items}
	}
}

func (m model) fetchMetadataCmd() tea.Cmd {
	if m.namespace == "" {
		return func() tea.Msg { return statusMsg{text: "No namespace selected", err: true} }
	}
	client := m.client
	namespace := m.namespace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ns := client.Namespace(namespace)
		meta, err := ns.Metadata(ctx, turbopuffer.NamespaceMetadataParams{})
		return metadataMsg{meta: meta, err: err}
	}
}

func (m model) fetchSchemaCmd() tea.Cmd {
	if m.namespace == "" {
		return func() tea.Msg { return statusMsg{text: "No namespace selected", err: true} }
	}
	client := m.client
	namespace := m.namespace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ns := client.Namespace(namespace)
		schema, err := ns.Schema(ctx, turbopuffer.NamespaceSchemaParams{})
		return schemaMsg{schema: schema, err: err}
	}
}

func (m model) fetchDocsCmd() tea.Cmd {
	if m.namespace == "" {
		return func() tea.Msg { return statusMsg{text: "No namespace selected", err: true} }
	}
	query := strings.TrimSpace(m.queryInput.Value())
	mode := m.queryMode
	textAttr := m.profile.TextAttr
	vectorAttr := m.profile.VectorAttr
	topK := m.profile.TopK
	client := m.client
	namespace := m.namespace

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		params := turbopuffer.NamespaceQueryParams{
			TopK:              turbopuffer.Int(int64(topK)),
			ExcludeAttributes: buildExcludeAttributes(m.profile, m.schema),
		}
		switch mode {
		case queryVector:
			vector, err := parseVector(query)
			if err != nil {
				return docsMsg{err: err}
			}
			params.RankBy = turbopuffer.NewRankByVector(vectorAttr, vector)
		default:
			if query == "" {
				params.RankBy = turbopuffer.NewRankByAttribute("id", turbopuffer.RankByAttributeOrderAsc)
			} else {
				params.RankBy = turbopuffer.NewRankByTextBM25(textAttr, query)
			}
		}

		ns := client.Namespace(namespace)
		resp, err := ns.Query(ctx, params)
		if err != nil {
			return docsMsg{err: err}
		}
		return docsMsg{rows: resp.Rows}
	}
}

func (m model) fetchDocByIDCmd(id string) tea.Cmd {
	if m.namespace == "" {
		return func() tea.Msg { return statusMsg{text: "No namespace selected", err: true} }
	}
	client := m.client
	namespace := m.namespace
	parsedID := parseID(id)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		params := turbopuffer.NamespaceQueryParams{
			TopK:              turbopuffer.Int(1),
			ExcludeAttributes: buildExcludeAttributes(m.profile, m.schema),
			Filters:           turbopuffer.NewFilterEq("id", parsedID),
			RankBy:            turbopuffer.NewRankByAttribute("id", turbopuffer.RankByAttributeOrderAsc),
		}
		ns := client.Namespace(namespace)
		resp, err := ns.Query(ctx, params)
		if err != nil {
			return docsMsg{err: err}
		}
		return docsMsg{rows: resp.Rows}
	}
}

func (m *model) rebuildNamespaceMatches() {
	m.nsMatches = m.nsMatches[:0]
	if len(m.namespaces) == 0 {
		m.nsCursor = 0
		return
	}
	filter := strings.TrimSpace(m.nsFilter.Value())
	if filter == "" {
		for i := range m.namespaces {
			m.nsMatches = append(m.nsMatches, i)
		}
		m.clampNamespaceCursor()
		return
	}
	names := make([]string, len(m.namespaces))
	for i, ns := range m.namespaces {
		names[i] = ns.ID
	}
	matches := fuzzy.Find(filter, names)
	for _, match := range matches {
		m.nsMatches = append(m.nsMatches, match.Index)
	}
	m.clampNamespaceCursor()
}

func (m *model) clampNamespaceCursor() {
	if len(m.nsMatches) == 0 {
		m.nsCursor = 0
		return
	}
	if m.nsCursor < 0 {
		m.nsCursor = 0
	}
	if m.nsCursor >= len(m.nsMatches) {
		m.nsCursor = len(m.nsMatches) - 1
	}
}

func (m model) currentNamespace() string {
	if len(m.nsMatches) == 0 {
		return ""
	}
	idx := m.nsMatches[m.nsCursor]
	if idx < 0 || idx >= len(m.namespaces) {
		return ""
	}
	return m.namespaces[idx].ID
}

func (m model) firstNamespace() string {
	if len(m.namespaces) == 0 {
		return ""
	}
	return m.namespaces[0].ID
}

func (m model) namespaceIndex(name string) int {
	for i, idx := range m.nsMatches {
		if m.namespaces[idx].ID == name {
			return i
		}
	}
	return 0
}

func (m model) cycleProfileCmd(delta int) tea.Cmd {
	if len(m.profileNames) == 0 {
		return func() tea.Msg {
			return statusMsg{text: "No profiles in config", err: true}
		}
	}
	next := m.profileIndex + delta
	if next >= len(m.profileNames) {
		next = 0
	}
	if next < 0 {
		next = len(m.profileNames) - 1
	}
	profileName := m.profileNames[next]
	profile := m.config.Profiles[profileName]
	return func() tea.Msg {
		return switchProfileMsg{
			name:    profileName,
			index:   next,
			profile: profile,
		}
	}
}

func parseVector(input string) ([]float32, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("vector query empty")
	}
	if strings.HasPrefix(trimmed, "[") {
		var vals []float32
		if err := json.Unmarshal([]byte(trimmed), &vals); err == nil {
			return vals, nil
		}
		var vals64 []float64
		if err := json.Unmarshal([]byte(trimmed), &vals64); err != nil {
			return nil, fmt.Errorf("vector parse: %w", err)
		}
		out := make([]float32, len(vals64))
		for i, v := range vals64 {
			out[i] = float32(v)
		}
		return out, nil
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	out := make([]float32, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		val, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return nil, fmt.Errorf("vector parse: %w", err)
		}
		out = append(out, float32(val))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("vector parse: no values")
	}
	return out, nil
}

func parseID(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if num, err := strconv.ParseInt(value, 10, 64); err == nil {
		return num
	}
	return value
}

func prettyJSON(v any) string {
	if v == nil {
		return ""
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(out)
}

func buildExcludeAttributes(profile Profile, schema *turbopuffer.NamespaceSchemaResponse) []string {
	names := make(map[string]struct{})
	vectorAttr := strings.TrimSpace(profile.VectorAttr)
	if schema != nil {
		for name, cfg := range *schema {
			if isVectorType(cfg.Type) {
				names[name] = struct{}{}
			}
			if vectorAttr != "" && name == vectorAttr {
				names[name] = struct{}{}
			}
		}
	} else if vectorAttr != "" {
		names[vectorAttr] = struct{}{}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	return out
}

func isVectorType(attrType string) bool {
	attrType = strings.TrimSpace(attrType)
	if attrType == "" {
		return false
	}
	if strings.HasPrefix(attrType, "[") && (strings.HasSuffix(attrType, "f16") || strings.HasSuffix(attrType, "f32")) {
		return true
	}
	if strings.Contains(attrType, "]f16") || strings.Contains(attrType, "]f32") {
		return true
	}
	return false
}

func maxScrollOffset(content string, height int) int {
	if height <= 0 {
		return 0
	}
	if content == "" {
		return 0
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return 0
	}
	return len(lines) - height
}

func clampScroll(offset int, max int) int {
	if offset < 0 {
		return 0
	}
	if offset > max {
		return max
	}
	return offset
}
