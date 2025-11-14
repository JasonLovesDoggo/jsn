/*
A little tool to parse though openrouter (https://openrouter.ai/) activity export logs and allow you to break it down a little more
*/

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

/* ============================= data & compute ============================= */

type Bucket struct {
	Start time.Time
	Count int
}

type Config struct {
	CSVPath string
	From    time.Time
	Window  time.Duration // ± window around From; 0 => t >= From
	App     string
	Key     string
}

type Data struct {
	Total        int
	RangeStart   time.Time
	RangeEnd     time.Time
	OverallTPS   float64
	PeakSecond   Bucket
	SecondCounts []Bucket // per-second counts across analyzed range
}

func compute(cfg Config) (Data, error) {
	f, err := os.Open(cfg.CSVPath)
	if err != nil {
		return Data{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return Data{}, fmt.Errorf("read: %w", err)
	}
	if len(rows) == 0 {
		return Data{}, fmt.Errorf("empty csv")
	}

	// header mapping
	h := map[string]int{}
	for i, name := range rows[0] {
		h[strings.TrimSpace(name)] = i
	}
	createdIdx, ok := h["created_at"]
	if !ok {
		return Data{}, fmt.Errorf("missing 'created_at' column")
	}
	appIdx := -1
	if i, ok := h["app_name"]; ok {
		appIdx = i
	}
	keyIdx := -1
	if i, ok := h["api_key_name"]; ok {
		keyIdx = i
	}

	// time window
	var lower, upper time.Time
	useWindow := cfg.Window > 0
	if useWindow {
		lower = cfg.From.Add(-cfg.Window)
		upper = cfg.From.Add(cfg.Window)
	} else {
		lower = cfg.From
	}

	secMap := map[time.Time]int{}
	var total int
	var firstSeen, lastSeen time.Time
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if createdIdx >= len(row) {
			continue
		}
		ts := strings.TrimSpace(row[createdIdx])
		t, ok := parseCSVTimeFlexible(ts)
		if !ok {
			continue
		}

		if useWindow {
			if t.Before(lower) || t.After(upper) {
				continue
			}
		} else if t.Before(lower) {
			continue
		}

		if cfg.App != "" && appIdx >= 0 {
			if appIdx >= len(row) || strings.TrimSpace(row[appIdx]) != cfg.App {
				continue
			}
		}
		if cfg.Key != "" && keyIdx >= 0 {
			if keyIdx >= len(row) || strings.TrimSpace(row[keyIdx]) != cfg.Key {
				continue
			}
		}

		s := t.Truncate(time.Second)
		secMap[s]++
		total++
		if firstSeen.IsZero() || t.Before(firstSeen) {
			firstSeen = t
		}
		if t.After(lastSeen) {
			lastSeen = t
		}
	}

	if total == 0 {
		rs, re := intendedRange(lower, upper, useWindow, firstSeen, lastSeen)
		return Data{RangeStart: rs, RangeEnd: re}, nil
	}

	rangeStart, rangeEnd := intendedRange(lower, upper, useWindow, firstSeen, lastSeen)

	var seconds []Bucket
	for t := rangeStart.Truncate(time.Second); !t.After(rangeEnd.Truncate(time.Second)); t = t.Add(time.Second) {
		seconds = append(seconds, Bucket{Start: t, Count: secMap[t]})
	}

	dur := rangeEnd.Sub(rangeStart)
	if dur <= 0 {
		dur = time.Second
	}
	overall := float64(total) / dur.Seconds()

	peak := Bucket{}
	for _, b := range seconds {
		if b.Count > peak.Count {
			peak = b
		}
	}

	return Data{
		Total:        total,
		RangeStart:   rangeStart,
		RangeEnd:     rangeEnd,
		OverallTPS:   overall,
		PeakSecond:   peak,
		SecondCounts: seconds,
	}, nil
}

func intendedRange(lower, upper time.Time, useWindow bool, firstSeen, lastSeen time.Time) (time.Time, time.Time) {
	if useWindow {
		return lower, upper
	}
	if firstSeen.IsZero() {
		return lower, lower
	}
	if lastSeen.Before(firstSeen) {
		lastSeen = firstSeen
	}
	return firstSeen, lastSeen
}

func parseCSVTimeFlexible(s string) (time.Time, bool) {
	loc := time.Local
	layouts := []string{
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, L := range layouts {
		if t, err := time.ParseInLocation(L, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// friendly "August 14, 2025 11:41 PM EDT" or without TZ; default to local TZ
func parseFriendlyFrom(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"January 2, 2006 3:04 PM MST",
		"January 2, 2006 3:04 PM",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, L := range layouts {
		if t, err := time.Parse(L, s); err == nil {
			if !strings.ContainsAny(s, "Z+-") && !hasTZAbbrev(s) {
				return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local), nil
			}
			return t, nil
		}
	}
	if t, ok := parseWithAbbrevMap(s); ok {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse: %q", s)
}
func hasTZAbbrev(s string) bool {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return false
	}
	last := parts[len(parts)-1]
	if len(last) >= 2 && len(last) <= 4 {
		for _, r := range last {
			if r < 'A' || r > 'Z' {
				return false
			}
		}
		return true
	}
	return false
}
func parseWithAbbrevMap(s string) (time.Time, bool) {
	abbr := map[string]int{"UTC": 0, "GMT": 0, "EDT": -4 * 60, "EST": -5 * 60, "CDT": -5 * 60, "CST": -6 * 60, "MDT": -6 * 60, "MST": -7 * 60, "PDT": -7 * 60, "PST": -8 * 60}
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return time.Time{}, false
	}
	last := fields[len(fields)-1]
	offsetMin, ok := abbr[last]
	if !ok {
		return time.Time{}, false
	}
	base := strings.TrimSpace(strings.TrimSuffix(s, " "+last))
	layouts := []string{"January 2, 2006 3:04 PM", "2006-01-02 15:04:05.000", "2006-01-02 15:04:05"}
	var t time.Time
	var err error
	for _, L := range layouts {
		t, err = time.ParseInLocation(L, base, time.UTC)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, false
	}
	loc := time.FixedZone(last, offsetMin*60)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc), true
}

func exportCSVs(cfg Config, d Data) error {
	sf, err := os.Create("tps_summary.csv")
	if err != nil {
		return err
	}
	sw := csv.NewWriter(sf)
	defer func() { sw.Flush(); sf.Close() }()
	sw.Write([]string{"range_start", "range_end", "duration_secs", "total", "overall_tps", "peak_second", "peak_count"})
	dur := d.RangeEnd.Sub(d.RangeStart)
	if dur < 0 {
		dur = 0
	}
	sw.Write([]string{
		d.RangeStart.Format(time.RFC3339),
		d.RangeEnd.Format(time.RFC3339),
		fmt.Sprintf("%.0f", dur.Seconds()),
		fmt.Sprintf("%d", d.Total),
		fmt.Sprintf("%.6f", d.OverallTPS),
		d.PeakSecond.Start.Format(time.RFC3339),
		fmt.Sprintf("%d", d.PeakSecond.Count),
	})

	tf, err := os.Create("tps_timeline.csv")
	if err != nil {
		return err
	}
	tw := csv.NewWriter(tf)
	defer func() { tw.Flush(); tf.Close() }()
	tw.Write([]string{"second", "count"})
	for _, b := range d.SecondCounts {
		tw.Write([]string{b.Start.Format(time.RFC3339), fmt.Sprintf("%d", b.Count)})
	}
	return nil
}

/* =================================== TUI =================================== */

type page int

const (
	pageForm page = iota
	pageSummary
)

type model struct {
	page          page
	width, height int

	// form
	fp      filepicker.Model
	inputs  []textinput.Model // 0: Path, 1: From, 2: ±Window, 3: App, 4: Key
	cursor  int
	errMsg  string
	infoMsg string

	// resolved
	cfg   Config
	data  Data
	table table.Model // busy seconds (>1)
}

func initialModel() model {
	fp := filepicker.New()
	fp.CurrentDirectory, _ = os.Getwd()
	fp.AllowedTypes = []string{".csv"}
	fp.ShowHidden = false
	fp.Height = 14 // overridden by resize

	makeTI := func(ph string, w int) textinput.Model {
		t := textinput.New()
		t.Placeholder = ph
		t.CharLimit = 0
		t.Width = w
		return t
	}
	inp := []textinput.Model{
		makeTI("Path override (paste full path or leave empty to use picker)", 80),
		makeTI("From, e.g., \"August 14, 2025 11:41 PM EDT\"", 50),
		makeTI("±Window (e.g., 30m, 1h, 2h30m) — blank = from onward", 40),
		makeTI("Filter app_name (optional)", 32),
		makeTI("Filter api_key_name (optional)", 32),
	}
	inp[0].Focus()

	return model{page: pageForm, fp: fp, inputs: inp}
}

func (m model) Init() tea.Cmd { return m.fp.Init() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		fpH := msg.Height - 14
		if fpH < 8 {
			fpH = 8
		}
		m.fp.Height = fpH
		return m, nil
	}
	switch m.page {
	case pageForm:
		return m.updateForm(msg)
	case pageSummary:
		return m.updateSummary(msg)
	default:
		return m, nil
	}
}

func (m model) View() string {
	switch m.page {
	case pageForm:
		return m.viewForm()
	case pageSummary:
		return m.viewSummary()
	default:
		return ""
	}
}

/* -------------------------------- form page -------------------------------- */

func (m model) viewForm() string {
	sb := &strings.Builder{}
	title := lipgloss.NewStyle().Bold(true).Render("TPS — Simple")
	sb.WriteString(title + "\n\n")

	cwd, _ := os.Getwd()
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Current directory: "+cwd) + "\n\n")

	labels := []string{"Path:", "From:", "±Window:", "App:", "Key:"}
	for i, in := range m.inputs {
		sb.WriteString(fmt.Sprintf("%-10s %s\n", labels[i], in.View()))
	}
	sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("…or pick a CSV below:") + "\n")
	sb.WriteString(m.fp.View() + "\n")

	sb.WriteString("\nTab/Shift+Tab move  •  Enter: select in picker / load if a file is selected  •  l: load  •  q: quit\n")

	if m.errMsg != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("\n"+m.errMsg) + "\n")
	}
	if m.infoMsg != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("\n"+m.infoMsg) + "\n")
	}
	return sb.String()
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch k := msg.(type) {
	case tea.KeyMsg:
		switch k.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.cursor = (m.cursor + 1) % len(m.inputs)
			for i := range m.inputs {
				if i == m.cursor {
					m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
		case "shift+tab":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.inputs) - 1
			}
			for i := range m.inputs {
				if i == m.cursor {
					m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
		case "l":
			return m.tryLoad()
		case "enter":
			// IMPORTANT: first let the picker process Enter so Path is updated
			var c tea.Cmd
			m.fp, c = m.fp.Update(msg)
			cmds = append(cmds, c)

			// If a file is now selected (not a dir) and no manual path override, load immediately
			if m.inputs[0].Value() == "" && m.fp.Path != "" && !isDir(m.fp.Path) {
				return m.tryLoad()
			}
			// Otherwise, treat Enter as "load" too (works for pasted path)
			return m.tryLoad()
		}
	}

	// regular updates (non-enter keys)
	var c tea.Cmd
	m.fp, c = m.fp.Update(msg)
	cmds = append(cmds, c)
	m.inputs[m.cursor], c = m.inputs[m.cursor].Update(msg)
	cmds = append(cmds, c)

	// informational message on selection
	if didSelect(msg) && m.fp.Path != "" {
		if isDir(m.fp.Path) {
			m.infoMsg = "Opened directory: " + m.fp.Path
		} else {
			m.infoMsg = "Selected: " + m.fp.Path
		}
	}

	switch x := msg.(type) {
	case dataMsg:
		m = m.updateAfterCompute(Data(x))
	case errMsg:
		m.errMsg = string(x)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) tryLoad() (tea.Model, tea.Cmd) {
	cfg, err := m.resolveConfig()
	if err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	m.cfg = cfg
	m.errMsg = ""
	return m, tea.Sequence(func() tea.Msg {
		d, err := compute(cfg)
		if err != nil {
			return errMsg(err.Error())
		}
		return dataMsg(d)
	})
}

func didSelect(msg tea.Msg) bool { k, ok := msg.(tea.KeyMsg); return ok && k.Type == tea.KeyEnter }
func isDir(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func (m *model) resolveConfig() (Config, error) {
	// path: manual override wins; else picker selection
	path := strings.TrimSpace(m.inputs[0].Value())
	if path == "" {
		path = m.fp.Path
		if path == "" || isDir(path) {
			return Config{}, fmt.Errorf("select a file in the picker OR paste a full path in 'Path'")
		}
	} else {
		path = strings.Trim(path, `"'`)
		if strings.HasPrefix(path, "~") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
		if !filepath.IsAbs(path) {
			cwd, _ := os.Getwd()
			path = filepath.Join(cwd, path)
		}
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			return Config{}, fmt.Errorf("path must be an existing file")
		}
	}
	fromStr := strings.TrimSpace(m.inputs[1].Value())
	if fromStr == "" {
		return Config{}, fmt.Errorf("From is required")
	}
	from, err := parseFriendlyFrom(fromStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid From: %v", err)
	}

	// window (±)
	var win time.Duration
	winStr := strings.TrimSpace(m.inputs[2].Value())
	if winStr != "" {
		w, err := time.ParseDuration(winStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ±Window: %v", err)
		}
		win = w
	}

	return Config{
		CSVPath: path,
		From:    from,
		Window:  win,
		App:     strings.TrimSpace(m.inputs[3].Value()),
		Key:     strings.TrimSpace(m.inputs[4].Value()),
	}, nil
}

/* ------------------------------- summary page ------------------------------ */

type dataMsg Data
type errMsg string

func (m model) viewSummary() string {
	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("TPS — Simple  ±window=%s  app=%q key=%q  |  Esc:Back  e:Export  q:Quit",
			func() string {
				if m.cfg.Window == 0 {
					return "none"
				}
				return m.cfg.Window.String()
			}(),
			m.cfg.App, m.cfg.Key),
	)
	summary := fmt.Sprintf(
		"\nRange: %s → %s (%.0fs)\nTotal gens: %d\nOverall TPS: %.4f\nPeak 1s: %d at %s\n",
		m.data.RangeStart.Format(time.RFC3339),
		m.data.RangeEnd.Format(time.RFC3339),
		m.data.RangeEnd.Sub(m.data.RangeStart).Seconds(),
		m.data.Total,
		m.data.OverallTPS,
		m.data.PeakSecond.Count,
		func() string {
			if m.data.PeakSecond.Start.IsZero() {
				return "-"
			}
			return m.data.PeakSecond.Start.Format(time.RFC3339)
		}(),
	)

	return header + "\n" + summary + "\n" + m.table.View() + "\n"
}

func (m model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch k := msg.(type) {
	case tea.KeyMsg:
		switch k.String() {
		case "esc":
			return m.toForm(), nil
		case "q", "ctrl+c":
			return m, tea.Quit
		case "e":
			if err := exportCSVs(m.cfg, m.data); err != nil {
				m.errMsg = "export failed: " + err.Error()
			} else {
				m.errMsg = "exported tps_summary.csv & tps_timeline.csv"
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) makeBusySecondsTable(d Data) table.Model {
	type kv struct {
		T time.Time
		C int
	}
	var arr []kv
	for _, b := range d.SecondCounts {
		if b.Count > 1 {
			arr = append(arr, kv{b.Start, b.Count})
		}
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].C != arr[j].C {
			return arr[i].C > arr[j].C
		}
		return arr[i].T.Before(arr[j].T)
	})

	cols := []table.Column{
		{Title: "Busy Seconds (>1)", Width: 24},
		{Title: "Count", Width: 8},
	}
	rows := make([]table.Row, 0, len(arr))
	for _, x := range arr {
		rows = append(rows, table.Row{x.T.Format("2006-01-02 15:04:05"), fmt.Sprintf("%d", x.C)})
	}
	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(false))
	t.SetStyles(table.DefaultStyles())
	h := m.height - 10
	if h < 4 {
		h = 4
	}
	t.SetHeight(h)
	return t
}

func (m model) updateAfterCompute(d Data) model {
	m.data = d
	m.table = m.makeBusySecondsTable(d)
	m.page = pageSummary
	return m
}

func (m model) toForm() model { m.page = pageForm; return m }

/* ---------------------------------- main ---------------------------------- */

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
