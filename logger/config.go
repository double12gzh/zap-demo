package logger

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// YamlConfig represents the YAML configuration structure
type YamlConfig struct {
	Logger Config `yaml:"logger"`
}

// LoadConfigFromYaml loads logger configuration from a YAML file
func LoadConfigFromYaml(configPath string) (*Config, error) {
	// Read the YAML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the YAML data
	var yamlConfig YamlConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Convert YAML config to logger Config
	config := &yamlConfig.Logger
	return config, nil
}

// InitLoggerFromYaml initializes the logger from a YAML configuration file.
// If configPath is empty or the file does not exist, it falls back to the default config.
func InitLoggerFromYaml(configPath string) error {
	if configPath == "" {
		return InitLogger(nil)
	}

	// Ensure the config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Fallback to default configuration instead of failing
		return InitLogger(nil)
	}

	// Load configuration from YAML
	config, err := LoadConfigFromYaml(configPath)
	if err != nil {
		return fmt.Errorf("failed to load logger config: %w", err)
	}

	// Initialize logger with the loaded configuration
	return InitLogger(config)
}
