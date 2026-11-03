package ai

import (
	"testing"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
)

func getenvFactory(vals map[string]string) func(string) string {
	return func(key string) string { return vals[key] }
}

func TestProvidersListIsComplete(t *testing.T) {
	list := Providers()
	if len(list) < 10 {
		t.Fatalf("Providers() = %d entries, want a full catalog", len(list))
	}
	for _, p := range list {
		if p.Name == "" || p.Label == "" {
			t.Errorf("provider entry %+v missing name/label", p)
		}
		if p.NeedsKey && p.BaseURL == "" && p.Name != "custom" {
			t.Errorf("provider %q needs a key but has no default base URL", p.Name)
		}
	}
	if _, ok := ByName("openai"); !ok {
		t.Error("ByName(openai) missing")
	}
	if _, ok := ByName("no-such"); ok {
		t.Error("ByName(no-such) should not resolve")
	}
	if MustByName("ollama").Name != "ollama" {
		t.Error("MustByName(ollama) = wrong entry")
	}
}

func TestResolveProviderDefaultsToZai(t *testing.T) {
	res, err := ResolveProvider(config.Config{}, getenvFactory(nil))
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	def, _ := ByName(config.DefaultProvider)
	if res.BaseURL != def.BaseURL {
		t.Errorf("BaseURL = %q, want provider default %q", res.BaseURL, def.BaseURL)
	}
	if res.Model != def.Model {
		t.Errorf("Model = %q, want provider default %q", res.Model, def.Model)
	}
	if res.APIKey != "" {
		t.Errorf("APIKey = %q, want empty without setup", res.APIKey)
	}
}

func TestResolveProviderStoredEntryWinsOverEnvVar(t *testing.T) {
	cfg := config.Config{
		Provider: "openai",
		Providers: map[string]config.ProviderAuth{
			"openai": {APIKey: "stored-key", Model: "gpt-4.1"},
		},
	}
	env := getenvFactory(map[string]string{"OPENAI_API_KEY": "env-key"})
	res, err := ResolveProvider(cfg, env)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if res.APIKey != "stored-key" {
		t.Errorf("APIKey = %q, want stored key to beat provider env var", res.APIKey)
	}
	if res.Model != "gpt-4.1" {
		t.Errorf("Model = %q, want stored per-provider model", res.Model)
	}
	if res.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want provider default", res.BaseURL)
	}
}

func TestResolveProviderFallsBackToProviderEnvVar(t *testing.T) {
	cfg := config.Config{Provider: "groq"}
	env := getenvFactory(map[string]string{"GROQ_API_KEY": "groq-key"})
	res, err := ResolveProvider(cfg, env)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if res.APIKey != "groq-key" {
		t.Errorf("APIKey = %q, want provider env var fallback", res.APIKey)
	}
}

func TestResolveProviderHarnessEnvOverridesEverything(t *testing.T) {
	cfg := config.Config{
		Provider: "openai",
		Providers: map[string]config.ProviderAuth{
			"openai": {APIKey: "stored", BaseURL: "http://stored"},
		},
	}
	env := getenvFactory(map[string]string{
		"THARNESS_BASE_URL": "http://override",
		"THARNESS_API_KEY":  "override-key",
	})
	res, err := ResolveProvider(cfg, env)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if res.BaseURL != "http://override" || res.APIKey != "override-key" {
		t.Errorf("got (%q, %q), want THARNESS_* env to override config", res.BaseURL, res.APIKey)
	}
}

func TestResolveProviderLegacyFlatConfig(t *testing.T) {
	cfg := config.Config{
		Provider:  "custom",
		BaseURL:   "http://legacy/v1",
		APIKeyEnv: "LEGACY_KEY",
		Providers: map[string]config.ProviderAuth{"custom": {}},
	}
	env := getenvFactory(map[string]string{"LEGACY_KEY": "legacy-key"})
	res, err := ResolveProvider(cfg, env)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if res.BaseURL != "http://legacy/v1" || res.APIKey != "legacy-key" {
		t.Errorf("got (%q, %q), want legacy flat config honored", res.BaseURL, res.APIKey)
	}
}

func TestResolveProviderKeylessOllama(t *testing.T) {
	cfg := config.Config{Provider: "ollama"}
	res, err := ResolveProvider(cfg, getenvFactory(nil))
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	def, _ := ByName("ollama")
	if res.BaseURL != def.BaseURL || res.APIKey != "" {
		t.Errorf("got (%q, %q), want ollama default endpoint without a key", res.BaseURL, res.APIKey)
	}
	if def.NeedsKey {
		t.Error("ollama should be keyless")
	}
}

func TestResolveProviderCustomWithoutBaseURLFails(t *testing.T) {
	cfg := config.Config{Provider: "custom"}
	if _, err := ResolveProvider(cfg, getenvFactory(nil)); err == nil {
		t.Fatal("custom provider without any base URL should fail with a setup hint")
	}
}

func TestResolveProviderUnknownNameFails(t *testing.T) {
	cfg := config.Config{Provider: "no-such"}
	if _, err := ResolveProvider(cfg, getenvFactory(nil)); err == nil {
		t.Fatal("unknown provider should fail")
	}
}
