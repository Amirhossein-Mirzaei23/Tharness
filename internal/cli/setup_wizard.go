package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/browser"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/theme"
)

// wizard steps, in order; stepKey is skipped for keyless providers and when
// a key is already stored.
type wizardStep int

const (
	stepProvider wizardStep = iota
	stepKey
	stepBaseURL
	stepModel
	stepBrowser
	stepTheme
	stepConfirm
)

var stepLabels = map[wizardStep]string{
	stepProvider: "provider",
	stepKey:      "api key",
	stepBaseURL:  "base url",
	stepModel:    "model",
	stepBrowser:  "browser",
	stepTheme:    "theme",
	stepConfirm:  "confirm",
}

// wizardOption is one selectable row in a list step.
type wizardOption struct {
	label  string
	detail string
	value  string
}

// wizardModel is the bubbletea model driving `tharness setup`.
type wizardModel struct {
	cfgPath string
	cfg     config.Config
	out     io.Writer

	order    []wizardStep    // remaining steps, fixed after provider choice
	step     wizardStep      // current step (0 until provider chosen)
	started  bool            // provider chosen
	options  []wizardOption  // rows for the current list step
	cursor   int             // list selection
	input    textinput.Model // key / base-url / model entry
	notice   string          // transient validation message
	quitting bool
	saved    bool
}

// runSetupWizard runs the interactive `tharness setup` wizard and writes the
// config when confirmed.
func runSetupWizard(cfgPath string, out io.Writer) error {
	if !isInteractive() {
		return fmt.Errorf("setup: interactive wizard needs a terminal; pass --provider/--api-key flags instead")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("setup: load config: %w", err)
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]config.ProviderAuth{}
	}

	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 60

	m := wizardModel{
		cfgPath: cfgPath,
		cfg:     cfg,
		out:     out,
		step:    stepProvider,
		input:   ti,
	}
	m.loadStep()

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	return nil
}

// isInteractive reports whether stdin is a usable terminal. /dev/null is a
// character device too (CI, `go test`), so it is excluded explicitly.
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Open(os.DevNull); err == nil {
		defer null.Close() //nolint:errcheck // read-only handle
		if nfi, err := null.Stat(); err == nil && os.SameFile(fi, nfi) {
			return false
		}
	}
	return true
}

// planSteps returns the ordered steps for the chosen provider given stored
// credentials. stepProvider is implicit (already answered).
func planSteps(def ai.ProviderDef, stored config.ProviderAuth) []wizardStep {
	steps := []wizardStep{}
	if def.NeedsKey && stored.APIKey == "" {
		steps = append(steps, stepKey)
	}
	steps = append(steps, stepBaseURL, stepModel, stepBrowser, stepTheme, stepConfirm)
	return steps
}

// listOptions builds the selectable rows for list steps.
func listOptions(step wizardStep) []wizardOption {
	switch step {
	case stepProvider:
		opts := make([]wizardOption, 0, len(ai.Providers()))
		for _, p := range ai.Providers() {
			detail := p.EnvVar
			if detail == "" {
				detail = "no key needed"
			}
			opts = append(opts, wizardOption{label: p.Label, detail: detail, value: p.Name})
		}
		return opts
	case stepBrowser:
		opts := []wizardOption{{label: "(auto-detect)", detail: "pick a browser at runtime", value: "auto"}}
		for _, c := range browser.Candidates() {
			opts = append(opts, wizardOption{label: c, detail: "detected", value: c})
		}
		opts = append(opts, wizardOption{label: "skip", detail: "keep current setting", value: ""})
		return opts
	case stepTheme:
		opts := make([]wizardOption, 0, len(theme.Palettes()))
		for _, p := range theme.Palettes() {
			opts = append(opts, wizardOption{label: p.Name, detail: p.Desc, value: p.Name})
		}
		return opts
	}
	return nil
}

// Init blinks the caret in text steps.
func (m wizardModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles keys per step.
func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			fmt.Fprintln(m.out, "setup cancelled; config unchanged")
			return m, tea.Quit
		case tea.KeyEsc:
			if !m.started || m.stepIdx() == 0 {
				m.quitting = true
				fmt.Fprintln(m.out, "setup cancelled; config unchanged")
				return m, tea.Quit
			}
			m.step = m.order[m.stepIdx()-1]
			m.loadStep()
			return m, nil
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
			return m, nil
		case tea.KeyEnter:
			return m.submit()
		}
		if m.step == stepKey || m.step == stepBaseURL || m.step == stepModel {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// stepIdx returns the position of the current step in the plan.
func (m wizardModel) stepIdx() int {
	for i, s := range m.order {
		if s == m.step {
			return i
		}
	}
	return 0
}

// submit advances from the current step.
func (m wizardModel) submit() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepProvider:
		def, _ := ai.ByName(m.options[m.cursor].value)
		m.cfg.Provider = def.Name
		m.order = planSteps(def, m.cfg.Providers[def.Name])
		m.started = true
		m.step = m.order[0]
		m.loadStep()
		return m, textinput.Blink
	case stepKey:
		key := strings.TrimSpace(m.input.Value())
		if key == "" && m.cfg.Providers[m.cfg.Provider].APIKey == "" {
			m.notice = "an API key is required for " + m.cfg.Provider
			return m, nil
		}
		if key != "" {
			auth := m.cfg.Providers[m.cfg.Provider]
			auth.APIKey = key
			m.cfg.Providers[m.cfg.Provider] = auth
		}
		m.advance()
		return m, textinput.Blink
	case stepBaseURL:
		v := strings.TrimSpace(m.input.Value())
		if v == "" {
			m.notice = "a base URL is required (restore the default: " + ai.MustByName(m.cfg.Provider).BaseURL + ")"
			return m, nil
		}
		auth := m.cfg.Providers[m.cfg.Provider]
		auth.BaseURL = v
		m.cfg.Providers[m.cfg.Provider] = auth
		m.advance()
		return m, textinput.Blink
	case stepModel:
		auth := m.cfg.Providers[m.cfg.Provider]
		auth.Model = strings.TrimSpace(m.input.Value())
		m.cfg.Providers[m.cfg.Provider] = auth
		m.advance()
		return m, textinput.Blink
	case stepBrowser:
		switch opt := m.options[m.cursor].value; opt {
		case "auto":
			m.cfg.Browser.Command = ""
		case "":
			// skip: keep the current setting untouched.
		default:
			m.cfg.Browser.Command = opt
		}
		m.advance()
		return m, nil
	case stepTheme:
		m.cfg.Theme = m.options[m.cursor].value
		m.advance()
		return m, nil
	case stepConfirm:
		return m.save()
	}
	return m, nil
}

// save writes the patched config to disk.
func (m wizardModel) save() (tea.Model, tea.Cmd) {
	if err := config.Save(m.cfg, m.cfgPath); err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.saved = true
	fmt.Fprintf(m.out, "\nsetup complete — config written%s\n", where(m.cfgPath))
	fmt.Fprintf(m.out, "provider %s (model %s) · api key %s · browser %s · theme %s\n",
		m.cfg.Provider, m.cfg.Providers[m.cfg.Provider].Model,
		maskKey(m.cfg.Providers[m.cfg.Provider].APIKey),
		browserDisplay(m.cfg.Browser.Command), m.cfg.Theme)
	return m, tea.Quit
}

// browserDisplay renders the stored browser setting for summaries.
func browserDisplay(command string) string {
	if command == "" {
		return "(auto-detect)"
	}
	return command
}

// configPathFor mirrors config.Save's default resolution for display.
func configPathFor(cfgPath string) string {
	if cfgPath != "" {
		return cfgPath
	}
	p, err := config.DefaultPath()
	if err != nil {
		return "(config)"
	}
	return p
}

// advance moves to the next step and prepares it.
func (m *wizardModel) advance() {
	m.notice = ""
	idx := m.stepIdx()
	if idx+1 < len(m.order) {
		m.step = m.order[idx+1]
	} else {
		m.step = stepConfirm
	}
	m.loadStep()
}

// loadStep prepares the model for the current step.
func (m *wizardModel) loadStep() {
	m.notice = ""
	m.cursor = 0
	m.options = listOptions(m.step)
	m.input.Blur()
	auth := m.cfg.Providers[m.cfg.Provider]
	def, known := ai.ByName(m.cfg.Provider)
	if !known {
		def = ai.ProviderDef{}
	}
	switch m.step {
	case stepKey:
		m.input.Focus()
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '•'
		m.input.Placeholder = "paste your " + m.cfg.Provider + " API key"
		m.input.SetValue("")
	case stepBaseURL:
		m.input.Focus()
		m.input.EchoMode = textinput.EchoNormal
		m.input.Placeholder = ""
		v := auth.BaseURL
		if v == "" {
			v = def.BaseURL
		}
		m.input.SetValue(v)
	case stepModel:
		m.input.Focus()
		m.input.EchoMode = textinput.EchoNormal
		v := auth.Model
		if v == "" {
			v = def.Model
		}
		m.input.SetValue(v)
	}
}

// View renders the current step.
func (m wizardModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	if m.started {
		fmt.Fprintf(&b, "tharness setup — %s (provider %s)\n\n", stepLabels[m.step], m.cfg.Provider)
	} else {
		fmt.Fprintf(&b, "tharness setup — %s\n\n", stepLabels[m.step])
	}

	switch m.step {
	case stepProvider, stepBrowser, stepTheme:
		for i, opt := range m.options {
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			detail := ""
			if opt.detail != "" {
				detail = "  · " + opt.detail
			}
			fmt.Fprintf(&b, "%s%s%s\n", prefix, opt.label, detail)
		}
		if m.step == stepTheme && m.cursor < len(m.options) {
			if p, ok := theme.Named(m.options[m.cursor].value); ok {
				b.WriteString("\n" + theme.Sample(p) + "\n")
			}
		}
	case stepKey, stepBaseURL, stepModel:
		b.WriteString(m.input.View() + "\n")
		if m.step == stepKey {
			b.WriteString("\nkeys are stored in " + configPathFor(m.cfgPath) + " (chmod 0600) — API keys only, no OAuth\n")
		}
	case stepConfirm:
		b.WriteString(m.confirmSummary())
	}

	if m.notice != "" {
		b.WriteString("\n" + m.notice + "\n")
	}
	b.WriteString("\n↑/↓ select · type to edit · enter confirm · esc back · ctrl+c cancel\n")
	return b.String()
}

// confirmSummary renders what will be written.
func (m wizardModel) confirmSummary() string {
	def, _ := ai.ByName(m.cfg.Provider)
	auth := m.cfg.Providers[m.cfg.Provider]

	model := def.Model
	if auth.Model != "" {
		model = auth.Model
	}
	base := def.BaseURL
	if auth.BaseURL != "" {
		base = auth.BaseURL
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  provider:  %s (%s)\n", m.cfg.Provider, base)
	fmt.Fprintf(&b, "  model:     %s\n", model)
	fmt.Fprintf(&b, "  api key:   %s\n", maskKey(auth.APIKey))
	fmt.Fprintf(&b, "  browser:   %s\n", browserDisplay(m.cfg.Browser.Command))
	fmt.Fprintf(&b, "  theme:     %s\n", m.cfg.Theme)
	fmt.Fprintf(&b, "\n  enter to write %s\n", configPathFor(m.cfgPath))
	return b.String()
}

// maskKey shows only the head and tail of a key.
func maskKey(key string) string {
	if key == "" {
		return "(none)"
	}
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}
