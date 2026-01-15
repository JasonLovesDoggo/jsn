package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type onboardingModel struct {
	path        string
	profileName string
	fields      []onboardingField
	index       int
	errText     string
	width       int
	height      int
	canceled    bool
}

type onboardingField struct {
	label    string
	help     string
	required bool
	input    textinput.Model
}

func NewOnboardingModel(path string, profileName string, profile Profile) onboardingModel {
	apiKey := textinput.New()
	apiKey.Placeholder = "tp_..."
	apiKey.Prompt = "api key> "
	apiKey.CharLimit = 256
	apiKey.EchoMode = textinput.EchoPassword
	apiKey.SetValue(profile.APIKey)

	region := textinput.New()
	region.Placeholder = "gcp-us-central1"
	region.Prompt = "region> "
	region.CharLimit = 128
	region.SetValue(profile.Region)

	baseURL := textinput.New()
	baseURL.Placeholder = "https://REGION.turbopuffer.com"
	baseURL.Prompt = "base url> "
	baseURL.CharLimit = 256
	baseURL.SetValue(profile.BaseURL)

	namespace := textinput.New()
	namespace.Placeholder = "optional"
	namespace.Prompt = "namespace> "
	namespace.CharLimit = 256
	namespace.SetValue(profile.Namespace)

	fields := []onboardingField{
		{
			label:    "API key",
			help:     "Required, looks like tp_...",
			required: true,
			input:    apiKey,
		},
		{
			label:    "Region",
			help:     "Required unless Base URL is set. Example: gcp-us-central1",
			required: false,
			input:    region,
		},
		{
			label:    "Base URL",
			help:     "Optional, overrides region. Example: https://REGION.turbopuffer.com",
			required: false,
			input:    baseURL,
		},
		{
			label:    "Namespace",
			help:     "Optional default namespace",
			required: false,
			input:    namespace,
		},
	}

	fields[0].input.Focus()

	return onboardingModel{
		path:        path,
		profileName: profileName,
		fields:      fields,
		index:       0,
	}
}

func (m onboardingModel) Init() tea.Cmd {
	return nil
}

func (m onboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			return m, tea.Quit
		case "tab":
			return m.nextField()
		case "shift+tab":
			return m.prevField()
		case "enter":
			if m.index == len(m.fields)-1 {
				if err := m.validate(); err != nil {
					m.errText = err.Error()
					return m, nil
				}
				return m, tea.Quit
			}
			return m.nextField()
		}
		var cmd tea.Cmd
		m.fields[m.index].input, cmd = m.fields[m.index].input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m onboardingModel) View() string {
	title := headerStyle.Render("tpuf onboarding")
	subtitle := subtleStyle.Render(fmt.Sprintf("profile: %s", m.profileName))

	lines := []string{title, subtitle, ""}
	if m.errText != "" {
		lines = append(lines, errorStyle.Render(m.errText), "")
	}

	for i, field := range m.fields {
		label := field.label
		if i == m.index {
			label = selectedStyle.Render(label)
		} else {
			label = subtleStyle.Render(label)
		}
		lines = append(lines, label)
		lines = append(lines, field.input.View())
		if field.help != "" {
			lines = append(lines, subtleStyle.Render(field.help))
		}
		lines = append(lines, "")
	}

	footer := subtleStyle.Render("tab/shift+tab to move, enter to continue, esc to cancel")
	lines = append(lines, footer)

	body := strings.Join(lines, "\n")
	if m.width == 0 || m.height == 0 {
		return body
	}
	style := lipgloss.NewStyle().Width(m.width).Height(m.height)
	return style.Render(body)
}

func (m onboardingModel) nextField() (tea.Model, tea.Cmd) {
	if m.index >= len(m.fields)-1 {
		return m, nil
	}
	m.fields[m.index].input.Blur()
	m.index++
	m.fields[m.index].input.Focus()
	return m, nil
}

func (m onboardingModel) prevField() (tea.Model, tea.Cmd) {
	if m.index <= 0 {
		return m, nil
	}
	m.fields[m.index].input.Blur()
	m.index--
	m.fields[m.index].input.Focus()
	return m, nil
}

func (m onboardingModel) validate() error {
	apiKey := strings.TrimSpace(m.fields[0].input.Value())
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}
	region := strings.TrimSpace(m.fields[1].input.Value())
	baseURL := strings.TrimSpace(m.fields[2].input.Value())
	if baseURL == "" && region == "" {
		return fmt.Errorf("Region is required unless Base URL is set")
	}
	return nil
}

func (m onboardingModel) profile() Profile {
	return Profile{
		APIKey:    strings.TrimSpace(m.fields[0].input.Value()),
		Region:    strings.TrimSpace(m.fields[1].input.Value()),
		BaseURL:   strings.TrimSpace(m.fields[2].input.Value()),
		Namespace: strings.TrimSpace(m.fields[3].input.Value()),
	}
}

func runOnboarding(path string, profileName string, cfg *Config, current Profile) (*Profile, bool, error) {
	model := NewOnboardingModel(path, profileName, current)
	final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, false, err
	}
	result, ok := final.(onboardingModel)
	if !ok {
		return nil, false, fmt.Errorf("onboarding: unexpected model")
	}
	if result.canceled {
		return nil, false, nil
	}
	input := result.profile()
	updated := current
	updated.APIKey = input.APIKey
	updated.Region = input.Region
	updated.BaseURL = input.BaseURL
	updated.Namespace = input.Namespace
	if updated.VectorAttr == "" {
		updated.VectorAttr = "vector"
	}
	if updated.TextAttr == "" {
		updated.TextAttr = "text"
	}
	if updated.TopK == 0 {
		updated.TopK = 20
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	cfg.Profiles[profileName] = updated
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = profileName
	}
	if err := WriteConfig(path, cfg); err != nil {
		return &updated, false, err
	}
	return &updated, true, nil
}
