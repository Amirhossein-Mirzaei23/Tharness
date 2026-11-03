// Package config loads and saves tharness configuration from a YAML file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// DefaultConfigDir is the directory (under $HOME) holding tharness state.
const DefaultConfigDir = ".tharness"

// DefaultModel is the fallback model when neither the config file nor the
// selected provider specifies one.
const DefaultModel = "zai/glm-5.3-flash"

// DefaultProvider is the provider used when the config file selects none.
const DefaultProvider = "zai"

// DefaultTheme is the TUI theme used when the config file selects none.
const DefaultTheme = "dark"

// Config holds the resolved tharness configuration.
type Config struct {
	// Model is the global fallback model for one-shot and chat runs.
	Model   string `mapstructure:"model"`
	DataDir string `mapstructure:"data_dir"`
	// BaseURL is the legacy flat provider endpoint override (pre-setup).
	BaseURL string `mapstructure:"base_url"`
	// APIKeyEnv names an environment variable holding the API key
	// (legacy flat form, pre-setup).
	APIKeyEnv string `mapstructure:"api_key_env"`
	// Provider selects the active provider by name (see internal/ai).
	Provider string `mapstructure:"provider"`
	// Providers holds per-provider credentials written by `tharness setup`.
	Providers map[string]ProviderAuth `mapstructure:"providers"`
	// Browser holds the configured browser executable.
	Browser Browser `mapstructure:"browser"`
	// Theme selects the TUI color palette (see internal/theme).
	Theme string `mapstructure:"theme"`
	// Permissions holds per-tool approval policies ("auto"/"ask"/"deny").
	Permissions Permissions `mapstructure:"permissions"`
}

// ProviderAuth holds one provider's API-key credentials.
type ProviderAuth struct {
	// APIKey is the provider API key (plain, file stored 0600).
	APIKey string `mapstructure:"api_key"`
	// BaseURL overrides the provider's default endpoint.
	BaseURL string `mapstructure:"base_url"`
	// Model pins the model used for this provider.
	Model string `mapstructure:"model"`
}

// Browser holds the browser configuration written by `tharness setup`.
type Browser struct {
	// Command is the browser executable (name on PATH or absolute path).
	// Empty means auto-detect at use time.
	Command string `mapstructure:"command"`
}

// Permissions holds the per-tool approval policies for gated tools.
type Permissions struct {
	Bash  string `mapstructure:"bash"`
	Write string `mapstructure:"write"`
	Edit  string `mapstructure:"edit"`
	// Browser gates the agent's browser (open URL) tool.
	Browser string `mapstructure:"browser"`
}

// DefaultPath returns the default config file location:
// $HOME/.tharness/config.yaml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve home: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir, "config.yaml"), nil
}

// Load reads configuration from the YAML file at path. An empty path means
// the default location ($HOME/.tharness/config.yaml). Missing files are not
// an error; only defaults apply.
func Load(path string) (Config, error) {
	if path == "" {
		def, err := DefaultPath()
		if err != nil {
			return Config{}, err
		}
		path = def
	}

	v := viper.New()
	v.SetDefault("model", "")
	v.SetDefault("provider", DefaultProvider)
	v.SetDefault("theme", DefaultTheme)
	v.SetDefault("permissions.bash", "auto")
	v.SetDefault("permissions.write", "auto")
	v.SetDefault("permissions.edit", "auto")
	v.SetDefault("permissions.browser", "auto")
	v.SetConfigType("yaml")
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return Config{}, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.DataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, err
		}
		cfg.DataDir = filepath.Join(home, DefaultConfigDir)
	}
	return cfg, nil
}

// Save writes cfg as YAML to path (an empty path means the default
// location), creating parent directories and restricting the file to 0600
// because it may contain API keys.
func Save(cfg Config, path string) error {
	if path == "" {
		def, err := DefaultPath()
		if err != nil {
			return err
		}
		path = def
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("config: create %s: %w", dir, err)
		}
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.Set("model", cfg.Model)
	v.Set("provider", cfg.Provider)
	v.Set("theme", cfg.Theme)
	if cfg.BaseURL != "" {
		v.Set("base_url", cfg.BaseURL)
	}
	if cfg.APIKeyEnv != "" {
		v.Set("api_key_env", cfg.APIKeyEnv)
	}
	v.Set("browser.command", cfg.Browser.Command)
	for name, auth := range cfg.Providers {
		v.Set(fmt.Sprintf("providers.%s.api_key", name), auth.APIKey)
		if auth.BaseURL != "" {
			v.Set(fmt.Sprintf("providers.%s.base_url", name), auth.BaseURL)
		}
		if auth.Model != "" {
			v.Set(fmt.Sprintf("providers.%s.model", name), auth.Model)
		}
	}
	v.Set("permissions.bash", cfg.Permissions.Bash)
	v.Set("permissions.write", cfg.Permissions.Write)
	v.Set("permissions.edit", cfg.Permissions.Edit)
	v.Set("permissions.browser", cfg.Permissions.Browser)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("config: chmod %s: %w", path, err)
	}
	return nil
}
