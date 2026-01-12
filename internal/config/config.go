// Package config provides configuration management for the gwq application.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/d-kuro/gwq/internal/utils"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/viper"
)

const (
	configName = "config"
	configType = "toml"
)

// getConfigDir returns the configuration directory path.
func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home is not available
		return filepath.Join(".", ".config", "gwq")
	}
	return filepath.Join(home, ".config", "gwq")
}

// Init initializes the configuration system, creating default config if needed.
func Init() error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	viper.SetConfigName(configName)
	viper.SetConfigType(configType)
	viper.AddConfigPath(configDir)

	viper.SetDefault("worktree.basedir", "~/worktrees")
	viper.SetDefault("worktree.auto_mkdir", true)
	viper.SetDefault("finder.preview", true)
	viper.SetDefault("ui.icons", true)
	viper.SetDefault("ui.tilde_home", true)

	// Naming defaults
	viper.SetDefault("naming.template", "{{.Host}}/{{.Owner}}/{{.Repository}}/{{.Branch}}")
	viper.SetDefault("naming.sanitize_chars", map[string]string{
		"/": "-",
		":": "-",
	})

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			configPath := filepath.Join(configDir, configName+"."+configType)
			if err := viper.SafeWriteConfig(); err != nil {
				if err := viper.WriteConfigAs(configPath); err != nil {
					return fmt.Errorf("failed to create config file: %w", err)
				}
			}
		} else {
			return fmt.Errorf("failed to read config: %w", err)
		}
	}

	return nil
}

// Load loads and returns the current configuration.
func Load() (*models.Config, error) {
	var cfg models.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	expandedPath, err := utils.ExpandPath(cfg.Worktree.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand worktree base dir: %w", err)
	}
	cfg.Worktree.BaseDir = expandedPath

	// Expand repository settings paths
	for i := range cfg.RepositorySettings {
		expandedPath, err = utils.ExpandPath(cfg.RepositorySettings[i].Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to expand repository setting path: %w", err)
		}
		cfg.RepositorySettings[i].Repository = expandedPath
	}
	return &cfg, nil
}

// Set sets a configuration value by key.
func Set(key string, value any) error {
	viper.Set(key, value)
	return viper.WriteConfig()
}

// GetValue retrieves a configuration value by key.
func GetValue(key string) any {
	return viper.Get(key)
}

// AllSettings returns all configuration settings.
func AllSettings() map[string]any {
	return viper.AllSettings()
}

// Get returns the current loaded configuration, loading it if necessary.
func Get() *models.Config {
	cfg, err := Load()
	if err != nil {
		// Initialize with viper defaults if config cannot be loaded
		var defaultCfg models.Config
		if err := viper.Unmarshal(&defaultCfg); err != nil {
			// Fallback to empty config if unmarshal fails
			return &models.Config{}
		}

		// Apply path expansions to defaults
		expandedPath, err := utils.ExpandPath(defaultCfg.Worktree.BaseDir)
		if err == nil {
			defaultCfg.Worktree.BaseDir = expandedPath
		}

		// Expand repository settings paths
		for i := range defaultCfg.RepositorySettings {
			expandedPath, err = utils.ExpandPath(defaultCfg.RepositorySettings[i].Repository)
			if err == nil {
				defaultCfg.RepositorySettings[i].Repository = expandedPath
			}
		}
		return &defaultCfg
	}
	return cfg
}
