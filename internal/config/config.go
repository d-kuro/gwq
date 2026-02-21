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
	configName      = "config"
	configType      = "toml"
	localConfigName = ".gwq"
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

// getLocalConfigPath returns the path to the local config file if it exists.
// Returns empty string if no local config is found.
func getLocalConfigPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	localConfigPath := filepath.Join(cwd, localConfigName+"."+configType)
	if _, err := os.Stat(localConfigPath); os.IsNotExist(err) {
		return ""
	}

	return localConfigPath
}

// mergeLocalConfig merges the local config file (.gwq.toml) from the current directory.
// Local config takes precedence over the global config.
// For repository_settings, merging is done by the "repository" field as the key.
func mergeLocalConfig() error {
	localConfigPath := getLocalConfigPath()
	if localConfigPath == "" {
		return nil
	}

	localViper := viper.New()
	localViper.SetConfigFile(localConfigPath)
	localViper.SetConfigType(configType)

	if err := localViper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read local config %s: %w", localConfigPath, err)
		}
		return nil
	}

	for _, key := range localViper.AllKeys() {
		if key == "repository_settings" {
			mergeRepositorySettings(localViper)
		} else {
			viper.Set(key, localViper.Get(key))
		}
	}

	return nil
}

// mergeRepositorySettings merges repository_settings from local config into global config.
// The "repository" field is used as the key for merging:
// - Same repository: local overrides global
// - Different repository: both are kept
func mergeRepositorySettings(localViper *viper.Viper) {
	var globalSettings, localSettings []models.RepositorySetting

	if err := viper.UnmarshalKey("repository_settings", &globalSettings); err != nil {
		globalSettings = nil
	}

	if err := localViper.UnmarshalKey("repository_settings", &localSettings); err != nil {
		return
	}

	localMap := make(map[string]models.RepositorySetting, len(localSettings))
	for _, ls := range localSettings {
		localMap[ls.Repository] = ls
	}

	merged := make([]models.RepositorySetting, 0, len(globalSettings)+len(localSettings))
	overridden := make(map[string]bool, len(localSettings))

	for _, gs := range globalSettings {
		if ls, exists := localMap[gs.Repository]; exists {
			merged = append(merged, ls)
			overridden[gs.Repository] = true
		} else {
			merged = append(merged, gs)
		}
	}

	for _, ls := range localSettings {
		if !overridden[ls.Repository] {
			merged = append(merged, ls)
		}
	}

	viper.Set("repository_settings", merged)
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

	viper.SetDefault("cd.launch_shell", true)
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

	// Merge local config from current directory if present
	if err := mergeLocalConfig(); err != nil {
		return fmt.Errorf("failed to merge local config: %w", err)
	}

	return nil
}

// Load loads and returns the current configuration.
func Load() (*models.Config, error) {
	var cfg models.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := expandConfigPaths(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// expandConfigPaths expands all path fields in the configuration.
func expandConfigPaths(cfg *models.Config) error {
	expandedPath, err := utils.ExpandPath(cfg.Worktree.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand worktree base dir: %w", err)
	}
	cfg.Worktree.BaseDir = expandedPath

	for i := range cfg.RepositorySettings {
		expandedPath, err = utils.ExpandPath(cfg.RepositorySettings[i].Repository)
		if err != nil {
			return fmt.Errorf("failed to expand repository setting path: %w", err)
		}
		cfg.RepositorySettings[i].Repository = expandedPath
	}
	return nil
}

// SetGlobal sets a configuration value and writes to the global config file only.
// This uses a separate viper instance to avoid writing merged local settings.
func SetGlobal(key string, value any) error {
	globalViper := viper.New()
	globalViper.SetConfigName(configName)
	globalViper.SetConfigType(configType)
	globalViper.AddConfigPath(getConfigDir())

	// Read only global config (ignore error if file doesn't exist)
	_ = globalViper.ReadInConfig()
	globalViper.Set(key, value)

	configPath := filepath.Join(getConfigDir(), configName+"."+configType)
	if err := globalViper.WriteConfigAs(configPath); err != nil {
		return err
	}

	// Update main viper instance as well
	viper.Set(key, value)
	return nil
}

// SetLocal sets a configuration value and writes to the local config file (.gwq.toml).
func SetLocal(key string, value any) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	localConfigPath := filepath.Join(cwd, localConfigName+"."+configType)

	localViper := viper.New()
	localViper.SetConfigFile(localConfigPath)
	localViper.SetConfigType(configType)

	_ = localViper.ReadInConfig()
	localViper.Set(key, value)

	if err := localViper.WriteConfigAs(localConfigPath); err != nil {
		return fmt.Errorf("failed to write local config: %w", err)
	}

	// Update main viper instance as well
	viper.Set(key, value)
	return nil
}

// Set sets a configuration value (defaults to global).
func Set(key string, value any) error {
	return SetGlobal(key, value)
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
			return &models.Config{}
		}

		// Apply path expansions to defaults, ignoring errors
		_ = expandConfigPaths(&defaultCfg)
		return &defaultCfg
	}
	return cfg
}
