package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	focusDocsList
	focusDocsDetail
)

type inputMode int

const (
	inputNone inputMode = iota
	inputNamespaceFilter
	inputQuery
	inputDocID
	inputProfileName
	inputFilter
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
	configPath     string
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
	profileInput   textinput.Model
	filterInput    textinput.Model
	filterRaw      string
	filter         turbopuffer.Filter
	pendingProfile string
	docs           []turbopuffer.Row
	docsCursor     int
	meta           *turbopuffer.NamespaceMetadata
	metaRendered   string
	schema         *turbopuffer.NamespaceSchemaResponse
	schemaRendered string
	metaScroll     int
	schemaScroll   int
	detailScroll   int
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
	warn string
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

type configEditedMsg struct {
	profile string
	err     error
}

func NewModel(cfg *Config, configPath string, profileName string, profile Profile) model {
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

	profileInput := textinput.New()
	profileInput.Placeholder = "new profile name"
	profileInput.CharLimit = 64
	profileInput.Prompt = "profile> "
	profileInput.Blur()

	filterInput := textinput.New()
	filterInput.Placeholder = "status=published, likes>=10"
	filterInput.CharLimit = 512
	filterInput.Prompt = "filter> "
	filterInput.Blur()

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
		configPath:   configPath,
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
		profileInput: profileInput,
		filterInput:  filterInput,
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
		hadMeta := m.meta != nil
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("metadata: %v", msg.err), true)
			return m, nil
		}
		m.meta = msg.meta
		m.metaRendered = prettyJSON(msg.meta)
		m.metaScroll = 0
		if !hadMeta {
			return m, m.fetchDocsCmd()
		}
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
		m.detailScroll = 0
		if msg.warn != "" {
			m.setStatus(msg.warn, true)
		} else {
			m.setStatus(fmt.Sprintf("Docs: %d result(s)", len(m.docs)), false)
		}
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
	case configEditedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Config edit: %v", msg.err), true)
			return m, nil
		}
		cfg, err := LoadConfig(m.configPath)
		if err != nil {
			m.setStatus(fmt.Sprintf("Config reload: %v", err), true)
			return m, nil
		}
		m.config = cfg
		m.profileNames = cfg.ProfileNames()
		if msg.profile != "" {
			if prof, ok := cfg.Profiles[msg.profile]; ok {
				if needsOnboarding(prof) {
					m.setStatus(fmt.Sprintf("Profile %q missing API key or region", msg.profile), true)
				} else {
					m.profileName = msg.profile
					m.profile = prof
					m.client = NewClient(prof)
					m.namespace = prof.Namespace
					m.setStatus(fmt.Sprintf("Profile: %s", msg.profile), false)
					return m, m.fetchNamespacesCmd()
				}
			} else {
				m.setStatus(fmt.Sprintf("Profile %q not found after edit", msg.profile), true)
			}
		}
		return m, nil
	case statusMsg:
		m.setStatus(msg.text, msg.err)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
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
		if m.activePane == paneDocs {
			switch m.focus {
			case focusNamespaces:
				m.focus = focusDocsList
			case focusDocsList:
				m.focus = focusDocsDetail
			default:
				m.focus = focusNamespaces
			}
			return m, nil
		}
		if m.focus == focusNamespaces {
			m.focus = focusDocsList
		} else {
			m.focus = focusNamespaces
		}
		return m, nil
	case "right":
		m.activePane = (m.activePane + 1) % 3
		return m, nil
	case "left":
		m.activePane = (m.activePane + 2) % 3
		return m, nil
	case "d":
		m.activePane = paneDocs
		if m.focus != focusNamespaces {
			m.focus = focusDocsList
		}
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
		if m.activePane == paneDocs && m.focus == focusDocsDetail {
			m.detailScroll = clampScroll(m.detailScroll+1, m.detailMaxOffset())
			return m, nil
		}
		if m.activePane != paneDocs && m.focus != focusNamespaces {
			m.scrollActivePane(1)
			return m, nil
		}
		m.moveCursor(1)
		return m, nil
	case "k", "up":
		if m.activePane == paneDocs && m.focus == focusDocsDetail {
			m.detailScroll = clampScroll(m.detailScroll-1, m.detailMaxOffset())
			return m, nil
		}
		if m.activePane != paneDocs && m.focus != focusNamespaces {
			m.scrollActivePane(-1)
			return m, nil
		}
		m.moveCursor(-1)
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
	case "y":
		if m.activePane == paneDocs {
			if err := m.copyDetailToClipboard(); err != nil {
				m.setStatus(fmt.Sprintf("copy: %v", err), true)
				return m, nil
			}
			m.setStatus("Copied detail to clipboard", false)
			return m, nil
		}
	case "f":
		if m.activePane == paneDocs {
			m.inputMode = inputFilter
			m.filterInput.SetValue(m.filterRaw)
			m.filterInput.Focus()
			m.setStatus("Edit filters", false)
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
		if len(m.profileNames) <= 1 {
			m.inputMode = inputProfileName
			m.profileInput.SetValue("")
			m.profileInput.Focus()
			m.setStatus("Create new profile (name)", false)
			return m, nil
		}
		return m, m.cycleProfileCmd(1)
	case "c":
		return m, m.openConfigCmd("")
	}
	return m, nil
}

func (m model) copyDetailToClipboard() error {
	if len(m.docs) == 0 || m.docsCursor >= len(m.docs) {
		return fmt.Errorf("no document selected")
	}
	row := m.docs[m.docsCursor]
	content := prettyJSON(sanitizeRow(row, m.profile.VectorAttr))
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("empty document")
	}
	return clipboard.WriteAll(content)
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.inputMode != inputNone {
		return m, nil
	}
	if msg.Button != tea.MouseButtonWheelUp && msg.Button != tea.MouseButtonWheelDown {
		return m, nil
	}
	if m.width == 0 || m.height == 0 {
		return m, nil
	}

	header := wrapBlock(m.viewHeader(), m.width)
	footer := wrapBlock(m.viewFooter(), m.width)
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	bodyHeight := m.height - headerHeight - footerHeight
	if bodyHeight <= 0 {
		return m, nil
	}
	if msg.Y < headerHeight || msg.Y >= headerHeight+bodyHeight {
		return m, nil
	}

	delta := 1
	if msg.Button == tea.MouseButtonWheelUp {
		delta = -1
	}

	leftWidth, _ := panelWidths(m.width)
	if msg.X < leftWidth {
		m.focus = focusNamespaces
		m.moveCursor(delta)
		return m, nil
	}

	if m.activePane == paneDocs {
		topHeight, _ := splitPaneHeights(bodyHeight)
		if msg.Y < headerHeight+topHeight {
			m.focus = focusDocsList
			m.moveCursor(delta)
			return m, nil
		}
		m.focus = focusDocsDetail
		m.detailScroll = clampScroll(m.detailScroll+delta, m.detailMaxOffset())
		return m, nil
	}

	if m.focus == focusNamespaces {
		m.focus = focusDocsList
	}
	m.scrollActivePane(delta)
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
	case inputProfileName:
		if msg.String() == "esc" {
			m.profileInput.Blur()
			m.inputMode = inputNone
			return m, nil
		}
		if msg.String() == "enter" {
			name := strings.TrimSpace(m.profileInput.Value())
			m.profileInput.Blur()
			m.inputMode = inputNone
			if name == "" {
				return m, nil
			}
			if _, ok := m.config.Profiles[name]; ok {
				m.setStatus(fmt.Sprintf("Profile %q already exists", name), true)
				return m, nil
			}
			newProfile := Profile{
				Namespace:  m.namespace,
				VectorAttr: m.profile.VectorAttr,
				TextAttr:   m.profile.TextAttr,
				TopK:       m.profile.TopK,
			}
			m.config.Profiles[name] = newProfile
			if err := WriteConfig(m.configPath, m.config); err != nil {
				m.setStatus(fmt.Sprintf("Save profile: %v", err), true)
				return m, nil
			}
			m.pendingProfile = name
			m.setStatus(fmt.Sprintf("Profile %q created, open config to edit", name), false)
			return m, m.openConfigCmd(name)
		}
		var cmd tea.Cmd
		m.profileInput, cmd = m.profileInput.Update(msg)
		return m, cmd
	case inputFilter:
		if msg.String() == "esc" {
			m.filterInput.Blur()
			m.inputMode = inputNone
			return m, nil
		}
		if msg.String() == "enter" {
			raw := strings.TrimSpace(m.filterInput.Value())
			m.filterInput.Blur()
			m.inputMode = inputNone
			if raw == "" {
				m.filterRaw = ""
				m.filter = nil
				m.setStatus("Filters cleared", false)
				return m, m.fetchDocsCmd()
			}
			filter, canonical, err := parseFilters(raw)
			if err != nil {
				m.setStatus(fmt.Sprintf("Filter error: %v", err), true)
				return m, nil
			}
			m.filterRaw = canonical
			m.filter = filter
			m.setStatus("Filters updated", false)
			return m, m.fetchDocsCmd()
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m *model) toggleFocus() {
	if m.focus == focusNamespaces {
		m.focus = focusDocsList
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
		m.detailScroll = 0
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

func (m model) detailMaxOffset() int {
	if len(m.docs) == 0 || m.docsCursor >= len(m.docs) {
		return 0
	}
	row := m.docs[m.docsCursor]
	content := prettyJSON(sanitizeRow(row, m.profile.VectorAttr))
	width := m.detailViewportWidth()
	if width > 0 {
		content = lipgloss.NewStyle().Width(width).Render(content)
	}
	lines := strings.Split(content, "\n")
	height := m.detailViewportHeight()
	if height <= 0 || len(lines) <= height {
		return 0
	}
	return len(lines) - height
}

func (m model) detailViewportWidth() int {
	_, rightWidth := panelWidths(m.width)
	content := rightWidth - 2
	if content < 0 {
		return 0
	}
	return content
}

func (m model) detailViewportHeight() int {
	body := m.rightPanelHeight()
	_, bottom := splitPaneHeights(body)
	if bottom <= 0 {
		return 0
	}
	content := bottom - 2
	if content < 0 {
		return 0
	}
	if content > 0 {
		content--
	}
	return content
}

func (m model) docsHeaderHeight() int {
	queryLabel := "query"
	if m.queryMode == queryVector {
		queryLabel = "vector"
	}
	ns := m.namespace
	if ns == "" {
		ns = "-"
	}
	contextLine := subtleStyle.Render(fmt.Sprintf("ns: %s  mode: %s  top_k: %d  focus: %s", ns, queryLabel, m.profile.TopK, m.focusLabel()))
	rankLine := subtleStyle.Render(m.rankSummary())
	filterLine := subtleStyle.Render(m.filterSummary())
	query := m.queryInput.View()
	if m.inputMode != inputQuery {
		if strings.TrimSpace(m.queryInput.Value()) == "" {
			query = subtleStyle.Render(fmt.Sprintf("(/) %s search", queryLabel))
		} else {
			query = subtleStyle.Render(fmt.Sprintf("%s: %s", queryLabel, m.queryInput.Value()))
		}
	}
	header := lipgloss.JoinVertical(lipgloss.Top, m.viewTabs(), contextLine, rankLine, filterLine, query)
	return lipgloss.Height(header)
}

func (m model) rightPanelHeight() int {
	header := wrapBlock(m.viewHeader(), m.width)
	footer := wrapBlock(m.viewFooter(), m.width)
	body := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if body < 0 {
		return 0
	}
	return body
}

func (m model) focusLabel() string {
	switch m.focus {
	case focusDocsDetail:
		return "detail"
	case focusDocsList:
		return "list"
	default:
		return "namespaces"
	}
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

func (m *model) openConfigCmd(profile string) tea.Cmd {
	editor := resolveEditor()
	cmd := exec.Command(editor, m.configPath)
	if profile != "" {
		m.pendingProfile = profile
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return configEditedMsg{profile: m.pendingProfile, err: err}
	})
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
			IncludeAttributes: buildIncludeAttributes(m.profile, m.meta, m.schema),
			Filters:           m.filter,
		}
		switch mode {
		case queryVector:
			if strings.TrimSpace(query) == "" {
				return docsMsg{err: fmt.Errorf("vector query empty")}
			}
			vector, err := parseVector(query)
			if err != nil {
				if isLikelyVectorInput(query) {
					return docsMsg{err: err}
				}
				vector, err = embedQuery(ctx, m.profile, query)
				if err != nil {
					return docsMsg{err: err}
				}
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
	parsedID, fallbackID := parseIDWithSchema(id, m.meta, m.schema)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		params := turbopuffer.NamespaceQueryParams{
			TopK:              turbopuffer.Int(1),
			IncludeAttributes: buildIncludeAttributes(m.profile, m.meta, m.schema),
			RankBy:            turbopuffer.NewRankByAttribute("id", turbopuffer.RankByAttributeOrderAsc),
		}
		ns := client.Namespace(namespace)
		filterable := idFilterable(m.meta, m.schema)
		resp, err := queryByID(ctx, ns, parsedID, fallbackID, params, filterable)
		if err != nil {
			return docsMsg{err: err}
		}
		warn := ""
		if !filterable {
			warn = "id is not filterable in schema; lookup may return zero results"
		}
		return docsMsg{rows: resp, warn: warn}
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

func parseFilters(raw string) (turbopuffer.Filter, string, error) {
	orGroups := splitTopLevel(raw, "||")
	orFilters := make([]turbopuffer.Filter, 0, len(orGroups))
	orCanon := make([]string, 0, len(orGroups))
	for _, group := range orGroups {
		parts := splitTopLevelMulti(group, []string{",", "&", "&&"})
		filters := make([]turbopuffer.Filter, 0, len(parts))
		canon := make([]string, 0, len(parts))
		for _, part := range parts {
			filter, canonical, err := parseFilterPart(part)
			if err != nil {
				return nil, "", err
			}
			if filter != nil {
				filters = append(filters, filter)
				if canonical != "" {
					canon = append(canon, canonical)
				}
			}
		}
		if len(filters) == 0 {
			continue
		}
		if len(filters) == 1 {
			orFilters = append(orFilters, filters[0])
			orCanon = append(orCanon, strings.Join(canon, " & "))
		} else {
			orFilters = append(orFilters, turbopuffer.NewFilterAnd(filters))
			orCanon = append(orCanon, strings.Join(canon, " & "))
		}
	}
	if len(orFilters) == 0 {
		return nil, "", nil
	}
	if len(orFilters) == 1 {
		return orFilters[0], orCanon[0], nil
	}
	return turbopuffer.NewFilterOr(orFilters), strings.Join(orCanon, " || "), nil
}

func splitTopLevel(raw string, sep string) []string {
	return splitTopLevelMulti(raw, []string{sep})
}

func splitTopLevelMulti(raw string, seps []string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var parts []string
	var buf strings.Builder
	inQuotes := false
	bracketDepth := 0
	i := 0
	for i < len(raw) {
		if raw[i] == '"' {
			inQuotes = !inQuotes
			buf.WriteByte(raw[i])
			i++
			continue
		}
		if raw[i] == '[' {
			bracketDepth++
			buf.WriteByte(raw[i])
			i++
			continue
		}
		if raw[i] == ']' {
			if bracketDepth > 0 {
				bracketDepth--
			}
			buf.WriteByte(raw[i])
			i++
			continue
		}
		if !inQuotes && bracketDepth == 0 {
			if sep, ok := matchSeparator(raw[i:], seps); ok {
				part := strings.TrimSpace(buf.String())
				if part != "" {
					parts = append(parts, part)
				}
				buf.Reset()
				i += len(sep)
				continue
			}
		}
		buf.WriteByte(raw[i])
		i++
	}
	if tail := strings.TrimSpace(buf.String()); tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

func matchSeparator(input string, seps []string) (string, bool) {
	for _, sep := range seps {
		if strings.HasPrefix(input, sep) {
			return sep, true
		}
	}
	return "", false
}

func parseFilterPart(part string) (turbopuffer.Filter, string, error) {
	part = strings.TrimSpace(part)
	if part == "" {
		return nil, "", nil
	}
	lower := strings.ToLower(part)
	if idx := strings.Index(lower, " not in "); idx >= 0 {
		field := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+len(" not in "):])
		list, err := parseValueList(value)
		if err != nil {
			return nil, "", err
		}
		return turbopuffer.NewFilterNotIn[any](field, list), fmt.Sprintf("%s not in %s", field, value), nil
	}
	if idx := strings.Index(lower, " in "); idx >= 0 {
		field := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+len(" in "):])
		list, err := parseValueList(value)
		if err != nil {
			return nil, "", err
		}
		return turbopuffer.NewFilterIn[any](field, list), fmt.Sprintf("%s in %s", field, value), nil
	}
	op, idx := findOperator(part)
	if op == "" || idx < 0 {
		return nil, "", fmt.Errorf("invalid filter: %q", part)
	}
	field := strings.TrimSpace(part[:idx])
	valueRaw := strings.TrimSpace(part[idx+len(op):])
	if field == "" || valueRaw == "" {
		return nil, "", fmt.Errorf("invalid filter: %q", part)
	}
	value, err := parseScalar(valueRaw)
	if err != nil {
		return nil, "", err
	}
	switch op {
	case "=":
		return turbopuffer.NewFilterEq(field, value), fmt.Sprintf("%s=%s", field, valueRaw), nil
	case "!=":
		return turbopuffer.NewFilterNotEq(field, value), fmt.Sprintf("%s!=%s", field, valueRaw), nil
	case ">":
		return turbopuffer.NewFilterGt(field, value), fmt.Sprintf("%s>%s", field, valueRaw), nil
	case ">=":
		return turbopuffer.NewFilterGte(field, value), fmt.Sprintf("%s>=%s", field, valueRaw), nil
	case "<":
		return turbopuffer.NewFilterLt(field, value), fmt.Sprintf("%s<%s", field, valueRaw), nil
	case "<=":
		return turbopuffer.NewFilterLte(field, value), fmt.Sprintf("%s<=%s", field, valueRaw), nil
	default:
		return nil, "", fmt.Errorf("unsupported operator: %q", op)
	}
}

func findOperator(part string) (string, int) {
	ops := []string{"!=", ">=", "<=", "=", ">", "<"}
	for _, op := range ops {
		if idx := strings.Index(part, op); idx >= 0 {
			return op, idx
		}
	}
	return "", -1
}

func parseValueList(raw string) ([]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty list")
	}
	if strings.HasPrefix(raw, "[") {
		var out []any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf("invalid list: %w", err)
		}
		return out, nil
	}
	parts := strings.Split(raw, "|")
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		val, err := parseScalar(part)
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty list")
	}
	return out, nil
}

func parseScalar(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "\"") {
		var s string
		if err := json.Unmarshal([]byte(raw), &s); err == nil {
			return s, nil
		}
	}
	lower := strings.ToLower(raw)
	if lower == "true" {
		return true, nil
	}
	if lower == "false" {
		return false, nil
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f, nil
	}
	return raw, nil
}
func resolveEditor() string {
	if editor := strings.TrimSpace(os.Getenv("VISUAL")); editor != "" {
		return editor
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return editor
	}
	return "vi"
}

func buildIncludeAttributes(profile Profile, meta *turbopuffer.NamespaceMetadata, schema *turbopuffer.NamespaceSchemaResponse) turbopuffer.IncludeAttributesParam {
	attrs := collectNonVectorAttributes(profile, meta, schema)
	if len(attrs) == 0 {
		return turbopuffer.IncludeAttributesParam{}
	}
	return turbopuffer.IncludeAttributesParam{StringArray: attrs}
}

func collectNonVectorAttributes(profile Profile, meta *turbopuffer.NamespaceMetadata, schema *turbopuffer.NamespaceSchemaResponse) []string {
	names := make(map[string]struct{})
	vectorAttr := strings.TrimSpace(profile.VectorAttr)
	add := func(name string) {
		if name == "" {
			return
		}
		if vectorAttr != "" && name == vectorAttr {
			return
		}
		names[name] = struct{}{}
	}
	if meta != nil {
		for name, cfg := range meta.Schema {
			if isVectorType(cfg.Type) {
				continue
			}
			add(name)
		}
	}
	if schema != nil {
		for name, cfg := range *schema {
			if isVectorType(cfg.Type) {
				continue
			}
			add(name)
		}
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

func idFilterable(meta *turbopuffer.NamespaceMetadata, schema *turbopuffer.NamespaceSchemaResponse) bool {
	if meta != nil {
		if cfg, ok := meta.Schema["id"]; ok {
			return cfg.Filterable
		}
	}
	if schema != nil {
		if cfg, ok := (*schema)["id"]; ok {
			return cfg.Filterable
		}
	}
	return true
}

func idSchemaType(meta *turbopuffer.NamespaceMetadata, schema *turbopuffer.NamespaceSchemaResponse) string {
	if meta != nil {
		if cfg, ok := meta.Schema["id"]; ok {
			return cfg.Type
		}
	}
	if schema != nil {
		if cfg, ok := (*schema)["id"]; ok {
			return cfg.Type
		}
	}
	return ""
}

func parseIDWithSchema(raw string, meta *turbopuffer.NamespaceMetadata, schema *turbopuffer.NamespaceSchemaResponse) (any, any) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value, nil
	}
	typ := strings.ToLower(strings.TrimSpace(idSchemaType(meta, schema)))
	switch typ {
	case "uint", "[]uint":
		if num, err := strconv.ParseUint(value, 10, 64); err == nil {
			if fmt.Sprint(num) != value {
				return num, value
			}
			return num, nil
		}
	case "int", "[]int":
		if num, err := strconv.ParseInt(value, 10, 64); err == nil {
			if fmt.Sprint(num) != value {
				return num, value
			}
			return num, nil
		}
	case "float", "[]float":
		if num, err := strconv.ParseFloat(value, 64); err == nil {
			return num, value
		}
	case "uuid", "string", "[]string":
		return value, nil
	}
	return parseID(value), value
}

func queryByID(ctx context.Context, ns turbopuffer.Namespace, parsedID any, fallbackID any, params turbopuffer.NamespaceQueryParams, filterable bool) ([]turbopuffer.Row, error) {
	params.Filters = turbopuffer.NewFilterEq("id", parsedID)
	resp, err := ns.Query(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(resp.Rows) > 0 {
		return resp.Rows, nil
	}
	if !filterable {
		return resp.Rows, nil
	}
	if fallbackID == nil || fmt.Sprint(fallbackID) == fmt.Sprint(parsedID) {
		return resp.Rows, nil
	}
	params.Filters = turbopuffer.NewFilterEq("id", fallbackID)
	resp, err = ns.Query(ctx, params)
	if err != nil {
		return nil, err
	}
	return resp.Rows, nil
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
