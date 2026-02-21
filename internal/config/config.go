// Package config handles configuration loading and memory home resolution.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Config types
// ---------------------------------------------------------------------------

// EmbeddingConfig holds settings for the embedding provider.
type EmbeddingConfig struct {
	Provider string `yaml:"provider"` // "ollama" | "openai" | "openrouter"
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"` // #nosec G117 -- APIKey is an intentional field name for the embedding provider's authentication token
}

// ContextConfig controls how memories are retrieved for context injection.
type ContextConfig struct {
	Semantic    string `yaml:"semantic"`     // "auto" | "always" | "never"
	TopupRecent bool   `yaml:"topup_recent"` // also include recent memories
}

// MemoryConfig is the root per-vault configuration.
type MemoryConfig struct {
	Embedding EmbeddingConfig `yaml:"embedding"`
	Context   ContextConfig   `yaml:"context"`
}

// Default returns a MemoryConfig populated with sensible defaults.
func Default() *MemoryConfig {
	return &MemoryConfig{
		Embedding: EmbeddingConfig{
			Provider: "ollama",
			Model:    "nomic-embed-text",
			BaseURL:  "http://localhost:11434",
		},
		Context: ContextConfig{
			Semantic:    "auto",
			TopupRecent: true,
		},
	}
}

// Load reads a per-vault config.yaml from path.
// If the file does not exist it returns Default() with no error.
// Missing keys retain their default values.
func Load(path string) (*MemoryConfig, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal into a plain map so we can apply only the keys that are present.
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if emb, ok := raw["embedding"].(map[string]any); ok {
		if v, ok := emb["provider"].(string); ok && v != "" {
			cfg.Embedding.Provider = v
		}
		if v, ok := emb["model"].(string); ok && v != "" {
			cfg.Embedding.Model = v
		}
		if v, ok := emb["base_url"].(string); ok {
			cfg.Embedding.BaseURL = v
		}
		if v, ok := emb["api_key"].(string); ok {
			cfg.Embedding.APIKey = v
		}
	}

	if ctx, ok := raw["context"].(map[string]any); ok {
		if v, ok := ctx["semantic"].(string); ok && v != "" {
			cfg.Context.Semantic = v
		}
		if v, ok := ctx["topup_recent"].(bool); ok {
			cfg.Context.TopupRecent = v
		}
	}

	return cfg, nil
}

// ---------------------------------------------------------------------------
// Memory home resolution
// ---------------------------------------------------------------------------

// globalConfigPath returns the path to the global echovault config file.
// This file stores only memory_home (and future global settings).
func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "echovault", "config.yaml"), nil
}

// normalizePath expands ~ and makes the path absolute.
func normalizePath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(os.ExpandEnv(path))
}

// ResolveMemoryHome returns the memory home path and the source of the resolution.
// Priority: MEMORY_HOME env → persisted global config → ~/.memory
// source is one of "env", "config", or "default".
func ResolveMemoryHome() (path, source string) {
	if env := os.Getenv("MEMORY_HOME"); env != "" {
		p, err := normalizePath(env)
		if err == nil {
			return p, "env"
		}
	}

	if persisted, ok, _ := GetPersistedMemoryHome(); ok {
		return persisted, "config"
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".memory"), "default"
}

// GetMemoryHome returns the resolved memory home path.
func GetMemoryHome() string {
	path, _ := ResolveMemoryHome()
	return path
}

// GetPersistedMemoryHome reads memory_home from the global config.
// Returns ("", false, nil) if not set.
func GetPersistedMemoryHome() (string, bool, error) {
	cfgPath, err := globalConfigPath()
	if err != nil {
		return "", false, err
	}

	data, err := os.ReadFile(cfgPath)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", false, nil
	}

	val, _ := raw["memory_home"].(string)
	val = strings.TrimSpace(val)
	if val == "" {
		return "", false, nil
	}

	p, err := normalizePath(val)
	if err != nil {
		return "", false, err
	}
	return p, true, nil
}

// SetPersistedMemoryHome normalizes path and persists it in the global config.
// Returns the normalized path.
func SetPersistedMemoryHome(path string) (string, error) {
	normalized, err := normalizePath(path)
	if err != nil {
		return "", err
	}

	cfgPath, err := globalConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return "", err
	}

	// Read existing global config, preserving any other keys.
	var raw map[string]any
	if data, err := os.ReadFile(cfgPath); err == nil {
		_ = yaml.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	raw["memory_home"] = normalized

	out, err := yaml.Marshal(raw)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, out, 0o600); err != nil {
		return "", err
	}
	return normalized, nil
}

// ClearPersistedMemoryHome removes memory_home from the global config.
// Returns true if the key was present and removed.
// If the file becomes empty after removal it is deleted.
func ClearPersistedMemoryHome() (bool, error) {
	cfgPath, err := globalConfigPath()
	if err != nil {
		return false, err
	}

	data, err := os.ReadFile(cfgPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, nil
	}

	if _, ok := raw["memory_home"]; !ok {
		return false, nil
	}
	delete(raw, "memory_home")

	if len(raw) == 0 {
		_ = os.Remove(cfgPath)
		return true, nil
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(cfgPath, out, 0o600)
}
