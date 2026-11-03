package cli

import (
	"fmt"
	"strings"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/browser"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/theme"
	"github.com/spf13/cobra"
)

// setupOptions collects the non-interactive `tharness setup` flags.
type setupOptions struct {
	provider    string
	apiKey      string
	baseURL     string
	model       string
	browser     string
	themeName   string
	listProv    bool
	listThemes  bool
	listBrowser bool
}

// newSetupCommand constructs `tharness setup`: an interactive wizard (or
// fully flagged, non-interactive) that writes provider API keys, the
// browser, and the TUI theme to the config file. No OAuth flows — every
// provider is configured with a plain API key.
func newSetupCommand() *cobra.Command {
	opts := &setupOptions{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure providers (API keys), browser, and theme",
		Long: `Configure tharness: provider API keys, browser, and TUI theme.

Run with no flags for the interactive wizard, or pass flags for a
non-interactive setup:

  tharness setup --provider openai --api-key sk-... --browser firefox --theme dracula

Every provider is configured with an API key only — tharness never uses
OAuth logins. The config file is written 0600 because it stores keys.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.provider, "provider", "", "provider to activate (see --list-providers)")
	cmd.Flags().StringVar(&opts.apiKey, "api-key", "", "API key for the selected provider")
	cmd.Flags().StringVar(&opts.baseURL, "base-url", "", "override the provider endpoint (custom providers)")
	cmd.Flags().StringVar(&opts.model, "model", "", "default model override")
	cmd.Flags().StringVar(&opts.browser, "browser", "", "browser executable (name on PATH or absolute path; see --list-browsers)")
	cmd.Flags().StringVar(&opts.themeName, "theme", "", "TUI theme (see --list-themes)")
	cmd.Flags().BoolVar(&opts.listProv, "list-providers", false, "list supported providers and exit")
	cmd.Flags().BoolVar(&opts.listThemes, "list-themes", false, "list themes and exit")
	cmd.Flags().BoolVar(&opts.listBrowser, "list-browsers", false, "list detected browsers and exit")
	return cmd
}

// runSetup dispatches listing, non-interactive, and wizard setup paths.
func runSetup(cmd *cobra.Command, opts *setupOptions) error {
	switch {
	case opts.listProv:
		for _, p := range ai.Providers() {
			key := fmt.Sprintf("key: %s", p.EnvVar)
			if !p.NeedsKey {
				key = "no key needed"
			}
			model := p.Model
			if model == "" {
				model = "(choose per install)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-28s %s\n  default model: %s\n", p.Name, p.Label, key, model)
		}
		return nil
	case opts.listThemes:
		for _, p := range theme.Palettes() {
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s\n", p.Name, p.Desc)
		}
		return nil
	case opts.listBrowser:
		found := browser.Candidates()
		if len(found) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no browser detected (set $BROWSER or configure a path)")
			return nil
		}
		for _, b := range found {
			fmt.Fprintln(cmd.OutOrStdout(), b)
		}
		return nil
	}

	cfgPath, _ := cmd.Flags().GetString("config")
	out := cmd.OutOrStdout()

	if opts.provider == "" {
		// Nothing actionable on the CLI: launch the interactive wizard.
		return runSetupWizard(cfgPath, out)
	}

	// Non-interactive path: validate and patch the config file.
	def, ok := ai.ByName(opts.provider)
	if !ok {
		return fmt.Errorf("setup: unknown provider %q (see --list-providers)", opts.provider)
	}
	if def.NeedsKey && def.Name != "custom" && opts.apiKey == "" {
		return fmt.Errorf("setup: provider %q requires --api-key (or run the interactive wizard)", opts.provider)
	}
	if def.Name == "custom" && opts.baseURL == "" {
		return fmt.Errorf("setup: custom providers require --base-url")
	}
	if opts.themeName != "" {
		if _, ok := theme.Named(opts.themeName); !ok {
			return fmt.Errorf("setup: unknown theme %q (see --list-themes)", opts.themeName)
		}
	}
	if opts.browser != "" && opts.browser != "auto" {
		if _, err := browser.Resolve(opts.browser); err != nil {
			return fmt.Errorf("setup: %v", err)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("setup: load config: %w", err)
	}
	applySetupInputs(&cfg, setupInputs{
		Provider: opts.provider,
		APIKey:   opts.apiKey,
		BaseURL:  opts.baseURL,
		Model:    opts.model,
		Browser:  opts.browser,
		Theme:    opts.themeName,
	})
	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	fmt.Fprintf(out, "configured provider %s (model %s)\n", opts.provider, cfg.Providers[opts.provider].Model)
	if cfg.Browser.Command != "" {
		fmt.Fprintf(out, "browser: %s\n", cfg.Browser.Command)
	}
	if cfg.Theme != "" {
		fmt.Fprintf(out, "theme: %s\n", cfg.Theme)
	}
	fmt.Fprintf(out, "config written%s\n", where(cfgPath))
	return nil
}

// where renders the "(at path)" suffix for setup output.
func where(cfgPath string) string {
	if cfgPath == "" {
		def, err := config.DefaultPath()
		if err != nil {
			return ""
		}
		return " (at " + def + ")"
	}
	return " (at " + cfgPath + ")"
}

// setupInputs is one batch of setup values; zero values leave the config
// untouched. Used by both the wizard and the flag path.
type setupInputs struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
	Browser  string
	Theme    string
}

// applySetupInputs patches cfg in place: it activates the provider, stores
// its credentials, and applies browser/theme when given. Per-provider
// BaseURL/Model only override when non-empty so provider defaults stay
// authoritative. Browser accepts "auto" to clear the stored command
// (auto-detect at use time); any other non-empty value is stored verbatim.
func applySetupInputs(cfg *config.Config, in setupInputs) {
	def, _ := ai.ByName(in.Provider)
	auth := cfg.Providers[in.Provider]
	if in.APIKey != "" {
		auth.APIKey = strings.TrimSpace(in.APIKey)
	}
	if in.BaseURL != "" {
		auth.BaseURL = strings.TrimSpace(in.BaseURL)
	}
	if in.Model != "" {
		auth.Model = strings.TrimSpace(in.Model)
	} else if auth.Model == "" && def.Model != "" {
		auth.Model = def.Model
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]config.ProviderAuth{}
	}
	cfg.Providers[in.Provider] = auth
	cfg.Provider = in.Provider
	switch in.Browser {
	case "auto":
		cfg.Browser.Command = ""
	case "":
		// leave the current setting untouched
	default:
		cfg.Browser.Command = in.Browser
	}
	if in.Theme != "" {
		cfg.Theme = in.Theme
	}
}
