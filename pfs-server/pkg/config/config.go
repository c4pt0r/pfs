package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the entire configuration file
type Config struct {
	Server  ServerConfig            `yaml:"server"`
	Plugins map[string]PluginConfig `yaml:"plugins"`
}

// ServerConfig contains server-level configuration
type ServerConfig struct {
	Address  string `yaml:"address"`
	LogLevel string `yaml:"log_level"`
}

// PluginConfig can be either a single plugin or an array of plugin instances
type PluginConfig struct {
	// For single instance plugins
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
	Config  map[string]interface{} `yaml:"config"`

	// For multi-instance plugins (array format)
	Instances []PluginInstance `yaml:"-"`
}

// PluginInstance represents a single instance of a plugin
type PluginInstance struct {
	Name    string                 `yaml:"name"`
	Enabled bool                   `yaml:"enabled"`
	Path    string                 `yaml:"path"`
	Config  map[string]interface{} `yaml:"config"`
}

// UnmarshalYAML implements custom unmarshaling to support both single plugin and array formats
func (p *PluginConfig) UnmarshalYAML(node *yaml.Node) error {
	// Try to unmarshal as array first
	var instances []PluginInstance
	if err := node.Decode(&instances); err == nil && len(instances) > 0 {
		p.Instances = instances
		return nil
	}

	// Otherwise, unmarshal as single plugin config
	type pluginConfigAlias PluginConfig
	aux := (*pluginConfigAlias)(p)
	return node.Decode(aux)
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// GetPluginConfig returns the configuration for a specific plugin
func (c *Config) GetPluginConfig(pluginName string) (PluginConfig, bool) {
	cfg, ok := c.Plugins[pluginName]
	return cfg, ok
}
