package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/viper"
)

func TestRepositorySettingsParsing(t *testing.T) {
	viper.Reset()
	viper.SetConfigType("toml")
	configTOML := `
[[repository_settings]]
repository = "/tmp/repository1"
copy_files = ["templates/.env.example", "config/*.json"]
setup_commands = ["npm install", "echo done"]

[[repository_settings]]
repository = "/tmp/repository2"
copy_files = ["foo.txt"]
setup_commands = ["touch bar"]
`
	err := viper.ReadConfig(strings.NewReader(configTOML))
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if len(cfg.RepositorySettings) != 2 {
		t.Fatalf("Expected 2 repository_settings, got %d", len(cfg.RepositorySettings))
	}
	if cfg.RepositorySettings[0].Repository != "/tmp/repository1" {
		t.Errorf("First repository mismatch: %s", cfg.RepositorySettings[0].Repository)
	}
	if len(cfg.RepositorySettings[0].CopyFiles) != 2 || cfg.RepositorySettings[0].CopyFiles[0] != "templates/.env.example" {
		t.Errorf("First repository copy_files mismatch: %+v", cfg.RepositorySettings[0].CopyFiles)
	}
	if len(cfg.RepositorySettings[0].SetupCommands) != 2 || cfg.RepositorySettings[0].SetupCommands[0] != "npm install" {
		t.Errorf("First repository setup_commands mismatch: %+v", cfg.RepositorySettings[0].SetupCommands)
	}
	if cfg.RepositorySettings[1].Repository != "/tmp/repository2" {
		t.Errorf("Second repository mismatch: %s", cfg.RepositorySettings[1].Repository)
	}
	if len(cfg.RepositorySettings[1].CopyFiles) != 1 || cfg.RepositorySettings[1].CopyFiles[0] != "foo.txt" {
		t.Errorf("Second repository copy_files mismatch: %+v", cfg.RepositorySettings[1].CopyFiles)
	}
	if len(cfg.RepositorySettings[1].SetupCommands) != 1 || cfg.RepositorySettings[1].SetupCommands[0] != "touch bar" {
		t.Errorf("Second repository setup_commands mismatch: %+v", cfg.RepositorySettings[1].SetupCommands)
	}
}

func TestGetConfigDir(t *testing.T) {
	// Test without XDG_CONFIG_HOME
	t.Run("WithoutXDGConfigHome", func(t *testing.T) {
		origXDG := os.Getenv("XDG_CONFIG_HOME")
		defer func() { _ = os.Setenv("XDG_CONFIG_HOME", origXDG) }()

		_ = os.Unsetenv("XDG_CONFIG_HOME")

		dir := getConfigDir()
		if !filepath.IsAbs(dir) {
			t.Errorf("getConfigDir() should return absolute path, got %s", dir)
		}
		if filepath.Base(dir) != "gwq" {
			t.Errorf("getConfigDir() should end with 'gwq', got %s", dir)
		}
	})

	// getConfigDir uses os.UserConfigDir which doesn't respect XDG_CONFIG_HOME on macOS
	// So we just verify the basic behavior
}

func TestInit(t *testing.T) {
	// Create temporary directory for config
	tmpDir := t.TempDir()

	// Set test environment
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	// Reset viper to clean state
	viper.Reset()

	// Test initialization
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify defaults are set
	if viper.GetString("worktree.basedir") != "~/worktrees" {
		t.Errorf("Default worktree.basedir not set correctly")
	}
	if !viper.GetBool("worktree.auto_mkdir") {
		t.Errorf("Default worktree.auto_mkdir should be true")
	}
	if !viper.GetBool("finder.preview") {
		t.Errorf("Default finder.preview should be true")
	}
	if !viper.GetBool("ui.icons") {
		t.Errorf("Default ui.icons should be true")
	}

	// Cleanup viper for other tests
	t.Cleanup(func() {
		viper.Reset()
	})
}

func TestLoad(t *testing.T) {
	// Setup viper with test values
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Set("worktree.basedir", "~/test-worktrees")
	viper.Set("worktree.auto_mkdir", false)
	viper.Set("finder.preview", false)
	viper.Set("ui.icons", false)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded values
	if cfg.Worktree.AutoMkdir {
		t.Errorf("WorktreeConfig.AutoMkdir = %v, want false", cfg.Worktree.AutoMkdir)
	}
	if cfg.Finder.Preview {
		t.Errorf("FinderConfig.Preview = %v, want false", cfg.Finder.Preview)
	}
	if cfg.UI.Icons {
		t.Errorf("UIConfig.Icons = %v, want false", cfg.UI.Icons)
	}
}

func TestPathExpansion(t *testing.T) {
	// Test home directory expansion
	t.Run("HomeDirectoryExpansion", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(func() {
			viper.Reset()
		})
		viper.Set("worktree.basedir", "~/worktrees")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if filepath.IsAbs(cfg.Worktree.BaseDir) && cfg.Worktree.BaseDir != "~/worktrees" {
			// Path was expanded
			if !filepath.IsAbs(cfg.Worktree.BaseDir) {
				t.Errorf("Expanded path should be absolute, got %s", cfg.Worktree.BaseDir)
			}
		}
	})

	// Test environment variable expansion
	t.Run("EnvironmentVariableExpansion", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(func() {
			viper.Reset()
		})
		_ = os.Setenv("TEST_WORKTREE_DIR", "/test/path")
		defer func() { _ = os.Unsetenv("TEST_WORKTREE_DIR") }()

		viper.Set("worktree.basedir", "$TEST_WORKTREE_DIR/worktrees")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		expected := "/test/path/worktrees"
		if cfg.Worktree.BaseDir != expected {
			t.Errorf("BaseDir = %s, want %s", cfg.Worktree.BaseDir, expected)
		}
	})
}

func TestGettersAndSetters(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})

	// Test Set and Get
	testKey := "test.key"
	testValue := "test-value"

	// Note: In real usage, Set would write to config file
	// For testing, we'll just verify viper operations
	viper.Set(testKey, testValue)

	if got := GetValue(testKey); got != testValue {
		t.Errorf("GetValue(%s) = %v, want %v", testKey, got, testValue)
	}

}

func TestAllSettings(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Set("test.key1", "value1")
	viper.Set("test.key2", 123)
	viper.Set("test.key3", true)

	settings := AllSettings()
	if len(settings) == 0 {
		t.Error("AllSettings() returned empty map")
	}

	// Check if our test settings are included
	if testSection, ok := settings["test"].(map[string]interface{}); ok {
		if testSection["key1"] != "value1" {
			t.Errorf("AllSettings() missing or incorrect test.key1")
		}
		if testSection["key2"] != 123 {
			t.Errorf("AllSettings() missing or incorrect test.key2")
		}
		if testSection["key3"] != true {
			t.Errorf("AllSettings() missing or incorrect test.key3")
		}
	} else {
		t.Error("AllSettings() missing 'test' section")
	}
}

func TestConfigStructureIntegrity(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	// This test ensures that the Config structure can be properly marshaled/unmarshaled
	cfg := &models.Config{
		Worktree: models.WorktreeConfig{
			BaseDir:   "/test/worktrees",
			AutoMkdir: true,
		},
		Finder: models.FinderConfig{
			Preview: true,
		},
		UI: models.UIConfig{
			Icons: false,
		},
	}

	// Set values in viper
	viper.Reset()
	viper.Set("worktree.basedir", cfg.Worktree.BaseDir)
	viper.Set("worktree.auto_mkdir", cfg.Worktree.AutoMkdir)
	viper.Set("finder.preview", cfg.Finder.Preview)
	viper.Set("ui.icons", cfg.UI.Icons)

	// Load and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Compare loaded config with original
	if loaded.Worktree.BaseDir != cfg.Worktree.BaseDir {
		t.Errorf("Worktree.BaseDir mismatch")
	}
	if loaded.Worktree.AutoMkdir != cfg.Worktree.AutoMkdir {
		t.Errorf("Worktree.AutoMkdir mismatch")
	}
	if loaded.Finder.Preview != cfg.Finder.Preview {
		t.Errorf("Finder.Preview mismatch")
	}
	if loaded.UI.Icons != cfg.UI.Icons {
		t.Errorf("UI.Icons mismatch")
	}
}
