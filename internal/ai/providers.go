package ai

import (
	"fmt"
	"os"
	"strings"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
)

// ProviderDef describes one supported LLM provider. Every provider is used
// through its OpenAI-compatible endpoint with a plain API key — tharness
// never performs OAuth logins.
type ProviderDef struct {
	// Name is the config key ("providers.<name>") and CLI identifier.
	Name string
	// Label is the human-readable display name.
	Label string
	// BaseURL is the provider's default OpenAI-compatible endpoint.
	BaseURL string
	// EnvVar names the environment variable holding the API key as a
	// fallback when no key is stored in the config file.
	EnvVar string
	// Model is the suggested default model for this provider.
	Model string
	// NeedsKey is false only for local servers that run without a key.
	NeedsKey bool
}

// providers lists every supported provider in setup-wizard order.
var providers = []ProviderDef{
	{Name: "zai", Label: "Z.ai (GLM)", BaseURL: "https://api.z.ai/api/paas/v4", EnvVar: "ZAI_API_KEY", Model: "glm-5.3-flash", NeedsKey: true},
	{Name: "openai", Label: "OpenAI", BaseURL: "https://api.openai.com/v1", EnvVar: "OPENAI_API_KEY", Model: "gpt-4o-mini", NeedsKey: true},
	{Name: "anthropic", Label: "Anthropic", BaseURL: "https://api.anthropic.com/v1", EnvVar: "ANTHROPIC_API_KEY", Model: "claude-sonnet-4-20250514", NeedsKey: true},
	{Name: "gemini", Label: "Google Gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", EnvVar: "GEMINI_API_KEY", Model: "gemini-2.5-flash", NeedsKey: true},
	{Name: "openrouter", Label: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", EnvVar: "OPENROUTER_API_KEY", Model: "openai/gpt-4o-mini", NeedsKey: true},
	{Name: "groq", Label: "Groq", BaseURL: "https://api.groq.com/openai/v1", EnvVar: "GROQ_API_KEY", Model: "llama-3.3-70b-versatile", NeedsKey: true},
	{Name: "mistral", Label: "Mistral AI", BaseURL: "https://api.mistral.ai/v1", EnvVar: "MISTRAL_API_KEY", Model: "mistral-large-latest", NeedsKey: true},
	{Name: "deepseek", Label: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", EnvVar: "DEEPSEEK_API_KEY", Model: "deepseek-chat", NeedsKey: true},
	{Name: "xai", Label: "xAI (Grok)", BaseURL: "https://api.x.ai/v1", EnvVar: "XAI_API_KEY", Model: "grok-2-latest", NeedsKey: true},
	{Name: "together", Label: "Together AI", BaseURL: "https://api.together.xyz/v1", EnvVar: "TOGETHER_API_KEY", Model: "meta-llama/Llama-3.3-70B-Instruct-Turbo", NeedsKey: true},
	{Name: "ollama", Label: "Ollama (local, no key)", BaseURL: "http://localhost:11434/v1", EnvVar: "", Model: "", NeedsKey: false},
	{Name: "custom", Label: "Custom OpenAI-compatible endpoint", BaseURL: "", EnvVar: "", Model: "", NeedsKey: true},
}

// Providers returns every supported provider in setup order.
func Providers() []ProviderDef {
	out := make([]ProviderDef, len(providers))
	copy(out, providers)
	return out
}

// Provider returns the definition for the named provider.
func ByName(name string) (ProviderDef, bool) {
	for _, p := range providers {
		if p.Name == name {
			return p, true
		}
	}
	return ProviderDef{}, false
}

// MustProvider returns the definition for a known provider name, or the zero
// ProviderDef when unknown. For display fallbacks only; resolution paths use
// Provider and handle the unknown case explicitly.
func MustByName(name string) ProviderDef {
	p, _ := ByName(name)
	return p
}

// Resolved holds the effective provider settings after applying the
// precedence rules of ResolveProvider.
type Resolved struct {
	// Provider is the active provider name (possibly unknown when BaseURL
	// comes from the legacy flat config).
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
}

// ResolveProvider applies tharness's provider precedence:
//
//	THARNESS_BASE_URL / THARNESS_API_KEY env overrides
//	> per-provider entry from cfg.Providers[cfg.Provider]
//	> the provider's own env var (def.EnvVar)
//	> legacy flat config (cfg.BaseURL, cfg.APIKeyEnv-named env var)
//	> provider defaults
//
// The model follows the same chain (per-provider > cfg.Model > provider
// default > config.DefaultModel). getenv defaults to os.Getenv in production
// and is a parameter for tests.
func ResolveProvider(cfg config.Config, getenv func(string) string) (Resolved, error) {
	if getenv == nil {
		getenv = defaultGetenv
	}
	name := cfg.Provider
	if name == "" {
		name = config.DefaultProvider
	}
	def, known := ByName(name)
	if !known && cfg.BaseURL == "" && (len(cfg.Providers) == 0 || cfg.Providers[name].BaseURL == "") {
		return Resolved{}, fmt.Errorf("ai: unknown provider %q (run `tharness setup`)", name)
	}

	auth := cfg.Providers[name]
	res := Resolved{Provider: name}

	// Base URL: env override > stored provider entry > provider default >
	// legacy flat config.
	res.BaseURL = first(getenv("THARNESS_BASE_URL"), auth.BaseURL, def.BaseURL, cfg.BaseURL)
	if res.BaseURL == "" {
		return Resolved{}, fmt.Errorf("ai: no base URL for provider %q (run `tharness setup`)", name)
	}

	// API key: env override > stored key > provider env var > legacy env.
	res.APIKey = first(getenv("THARNESS_API_KEY"), auth.APIKey, getenv(def.EnvVar), getenv(cfg.APIKeyEnv))

	// Model: stored per-provider > config > provider suggestion > harness
	// default.
	res.Model = first(auth.Model, cfg.Model, def.Model, config.DefaultModel)
	return res, nil
}

// HasKey reports whether the resolved configuration carries an API key or
// the provider runs keyless.
func (r Resolved) HasKey(def ProviderDef) bool {
	return r.APIKey != "" || !def.NeedsKey
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func defaultGetenv(key string) string { return os.Getenv(key) }
