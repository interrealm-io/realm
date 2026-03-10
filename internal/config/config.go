package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Mode defines whether a realm participates in the public realmnet ledger.
type Mode string

const (
	ModePrivate Mode = "private" // default — local only, no ledger registration
	ModePublic  Mode = "public"  // opt-in — registers on realmnet, endpoint is public
)

// Config is the top-level structure of realm.yaml.
type Config struct {
	Realm        RealmConfig        `yaml:"realm"`
	Network      NetworkConfig      `yaml:"network"`
	Capabilities CapabilitiesConfig `yaml:"capabilities"`
	Observability ObservabilityConfig `yaml:"observability"`
}

type RealmConfig struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Mode    Mode   `yaml:"mode"`
	Keyfile string `yaml:"keyfile"`
}

type NetworkConfig struct {
	Port      int      `yaml:"port"`
	Endpoint  string   `yaml:"endpoint"`
	Bootstrap []string `yaml:"bootstrap"`
}

type CapabilitiesConfig struct {
	Enabled  bool         `yaml:"enabled"`
	BasePath string       `yaml:"basePath"`
	Tools    []ToolConfig `yaml:"tools"`
}

type ToolConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Path        string `yaml:"path"`
	Method      string `yaml:"method"`
	Public      bool   `yaml:"public"`
}

type ObservabilityConfig struct {
	LogLevel string `yaml:"logLevel"`
}

// Load reads and parses a realm.yaml file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Realm.ID == "" {
		return fmt.Errorf("realm.id is required")
	}
	if c.Realm.Keyfile == "" {
		return fmt.Errorf("realm.keyfile is required")
	}
	if c.Realm.Mode == ModePublic && c.Network.Endpoint == "" {
		return fmt.Errorf("network.endpoint is required when realm.mode is 'public'")
	}
	if c.Network.Port == 0 {
		c.Network.Port = 8080
	}
	if c.Capabilities.BasePath == "" {
		c.Capabilities.BasePath = "/capabilities"
	}
	return nil
}
