// Package config loads tabdl configuration from an optional YAML file.
package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// DefaultConfigDir is the directory (under $HOME) holding tabdl state.
const DefaultConfigDir = ".tabdl"

// DefaultModel is the model used when no configuration file specifies one.
const DefaultModel = "zai/glm-5.3-flash"

// Config holds the resolved tabdl configuration.
type Config struct {
	Model     string
	DataDir   string
	BaseURL   string
	APIKeyEnv string
	// Permissions holds per-tool approval policies ("auto"/"ask"/"deny").
	Permissions Permissions
}

// Permissions holds the per-tool approval policies for gated tools.
type Permissions struct {
	Bash  string
	Write string
	Edit  string
}

// Load reads configuration from the YAML file at path. An empty path means no
// file is read and only defaults apply. Missing files are not an error.
func Load(path string) (Config, error) {
	v := viper.New()
	v.SetDefault("model", DefaultModel)
	v.SetDefault("base_url", "")
	v.SetDefault("api_key_env", "")
	v.SetDefault("permissions.bash", "auto")
	v.SetDefault("permissions.write", "auto")
	v.SetDefault("permissions.edit", "auto")
	v.SetConfigType("yaml")

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
			return Config{}, err
		}
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
