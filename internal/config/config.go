package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level application configuration.
type Config struct {
	State      StateConfig      `yaml:"state"`
	Providers  []ProviderConfig `yaml:"providers"`
	Comparison ComparisonConfig `yaml:"comparison"`
	Schedules  []ScheduleConfig `yaml:"schedules"`
	Output     OutputConfig     `yaml:"output"`
	Database   DatabaseConfig   `yaml:"database"`
	API        APIConfig        `yaml:"api"`
}

type StateConfig struct {
	Source string `yaml:"source"`
	Path   string `yaml:"path"`
}

type ProviderConfig struct {
	Name          string            `yaml:"name"`
	Regions       []string          `yaml:"regions"`
	Credentials   string            `yaml:"credentials"`
	ResourceTypes []string          `yaml:"resource_types"`
	Extra         map[string]string `yaml:"extra,omitempty"`
}

type ComparisonConfig struct {
	IgnoreAttributes map[string][]string `yaml:"ignore_attributes"`
}

type ScheduleConfig struct {
	Name    string `yaml:"name"`
	Cron    string `yaml:"cron"`
	Enabled bool   `yaml:"enabled"`
}

type OutputConfig struct {
	Console  bool   `yaml:"console"`
	JSONPath string `yaml:"json_path"`
	APIStore bool   `yaml:"api_store"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type APIConfig struct {
	Addr   string `yaml:"addr"`
	APIKey string `yaml:"api_key"`
}

// Default returns sensible defaults.
func Default() *Config {
	return &Config{
		State: StateConfig{
			Source: "local",
			Path:   "./terraform.tfstate",
		},
		Providers: []ProviderConfig{
			{
				Name:        "aws",
				Regions:     []string{"us-east-1"},
				Credentials: "env",
			},
		},
		Comparison: ComparisonConfig{
			IgnoreAttributes: map[string][]string{
				"global": {
					"id", "arn", "tags_all", "tags", "timeouts",
					"region", "account_id", "unique_id", "owner_id",
				},
			},
		},
		Output: OutputConfig{
			Console: true,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "./drift.db",
		},
		API: APIConfig{
			Addr: ":8080",
		},
	}
}

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// PrimaryProvider returns the first configured provider.
func (c *Config) PrimaryProvider() *ProviderConfig {
	if len(c.Providers) == 0 {
		return nil
	}
	return &c.Providers[0]
}
