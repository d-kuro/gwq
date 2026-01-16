package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/viper"
)

// changeDir changes to the specified directory and returns a cleanup function
// that restores the original working directory.
func changeDir(t *testing.T, dir string) {
	t.Helper()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
}

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

func TestLoadIgnoresLegacyClaudeSettings(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.SetConfigType("toml")
	configTOML := `
[worktree]
basedir = "/tmp/worktrees"
auto_mkdir = true

[claude]
executable = "claude"
config_dir = "~/.config/gwq/claude"
max_parallel = 4

[claude.queue]
queue_dir = "~/.config/gwq/claude/queue"
`
	if err := viper.ReadConfig(strings.NewReader(configTOML)); err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Worktree.BaseDir != "/tmp/worktrees" {
		t.Errorf("Worktree.BaseDir = %s, want /tmp/worktrees", cfg.Worktree.BaseDir)
	}
	if !cfg.Worktree.AutoMkdir {
		t.Error("Worktree.AutoMkdir should be true")
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

func TestMergeLocalConfig(t *testing.T) {
	t.Run("NoLocalConfig", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(func() { viper.Reset() })

		// Change to temp directory (no local config)
		tmpDir := t.TempDir()
		changeDir(t, tmpDir)

		// Load global settings
		viper.SetConfigType("toml")
		viper.Set("worktree.basedir", "~/global-worktrees")

		// Merge local config (no file exists)
		if err := mergeLocalConfig(); err != nil {
			t.Fatalf("mergeLocalConfig() error = %v", err)
		}

		// Verify global config is preserved
		if viper.GetString("worktree.basedir") != "~/global-worktrees" {
			t.Errorf("Global config should be preserved, got %s", viper.GetString("worktree.basedir"))
		}
	})

	t.Run("WithLocalConfig", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(func() { viper.Reset() })

		// Create local config in temp directory
		tmpDir := t.TempDir()
		localConfigPath := filepath.Join(tmpDir, ".gwq.toml")
		localConfig := []byte(`
[worktree]
basedir = "~/local-worktrees"

[finder]
preview = false
`)
		if err := os.WriteFile(localConfigPath, localConfig, 0644); err != nil {
			t.Fatalf("Failed to create local config: %v", err)
		}
		changeDir(t, tmpDir)

		// Load global settings
		viper.SetConfigType("toml")
		viper.Set("worktree.basedir", "~/global-worktrees")
		viper.Set("worktree.auto_mkdir", true)
		viper.Set("finder.preview", true)

		// Merge local config
		if err := mergeLocalConfig(); err != nil {
			t.Fatalf("mergeLocalConfig() error = %v", err)
		}

		// Verify local config overrides global
		if viper.GetString("worktree.basedir") != "~/local-worktrees" {
			t.Errorf("Local config should override global: got %s", viper.GetString("worktree.basedir"))
		}
		if viper.GetBool("finder.preview") != false {
			t.Errorf("Local config should override finder.preview to false")
		}
		// Verify global-only settings are preserved
		if viper.GetBool("worktree.auto_mkdir") != true {
			t.Errorf("Global-only config should be preserved")
		}
	})

	t.Run("PartialOverride", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(func() { viper.Reset() })

		// Create partial local config in temp directory
		tmpDir := t.TempDir()
		localConfigPath := filepath.Join(tmpDir, ".gwq.toml")
		localConfig := []byte(`
[naming]
template = "{{.Repository}}/{{.Branch}}"
`)
		if err := os.WriteFile(localConfigPath, localConfig, 0644); err != nil {
			t.Fatalf("Failed to create local config: %v", err)
		}
		changeDir(t, tmpDir)

		// Load global settings
		viper.SetConfigType("toml")
		viper.Set("worktree.basedir", "~/global-worktrees")
		viper.Set("naming.template", "{{.Host}}/{{.Owner}}/{{.Repository}}/{{.Branch}}")
		viper.Set("naming.sanitize_chars", map[string]string{"/": "-"})

		// Merge local config
		if err := mergeLocalConfig(); err != nil {
			t.Fatalf("mergeLocalConfig() error = %v", err)
		}

		// Verify only naming.template is overridden
		if viper.GetString("naming.template") != "{{.Repository}}/{{.Branch}}" {
			t.Errorf("naming.template should be overridden, got %s", viper.GetString("naming.template"))
		}
		// Verify worktree.basedir is preserved
		if viper.GetString("worktree.basedir") != "~/global-worktrees" {
			t.Errorf("worktree.basedir should be preserved, got %s", viper.GetString("worktree.basedir"))
		}
	})
}

func TestMergeLocalConfigRepositorySettings(t *testing.T) {
	t.Run("MergeByRepositoryKey", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(func() { viper.Reset() })

		// Create local config in temp directory
		tmpDir := t.TempDir()
		localConfigPath := filepath.Join(tmpDir, ".gwq.toml")
		localConfig := []byte(`
[[repository_settings]]
repository = "/shared/repo"
setup_commands = ["npm install"]
copy_files = [".env.local"]

[[repository_settings]]
repository = "/local/repo"
setup_commands = ["make build"]
`)
		if err := os.WriteFile(localConfigPath, localConfig, 0644); err != nil {
			t.Fatalf("Failed to create local config: %v", err)
		}
		changeDir(t, tmpDir)

		// Load global settings (including repository_settings)
		viper.SetConfigType("toml")
		globalConfig := `
[[repository_settings]]
repository = "/global/repo"
setup_commands = ["yarn install"]
copy_files = [".env.global"]

[[repository_settings]]
repository = "/shared/repo"
setup_commands = ["old command"]
copy_files = [".env.old"]
`
		if err := viper.ReadConfig(strings.NewReader(globalConfig)); err != nil {
			t.Fatalf("Failed to read global config: %v", err)
		}

		// Merge local config
		if err := mergeLocalConfig(); err != nil {
			t.Fatalf("mergeLocalConfig() error = %v", err)
		}

		// Load as struct
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Verify repository settings are merged by repository key:
		// - Global only: /global/repo
		// - Local only: /local/repo
		// - Both: /shared/repo -> overridden by local
		if len(cfg.RepositorySettings) != 3 {
			t.Errorf("Expected 3 repository_settings (merged), got %d", len(cfg.RepositorySettings))
			for _, rs := range cfg.RepositorySettings {
				t.Logf("  - %s", rs.Repository)
			}
		}

		// Verify each setting
		found := make(map[string]bool)
		for _, rs := range cfg.RepositorySettings {
			found[rs.Repository] = true

			switch rs.Repository {
			case "/global/repo":
				// Global-only settings are preserved
				if len(rs.SetupCommands) != 1 || rs.SetupCommands[0] != "yarn install" {
					t.Errorf("/global/repo setup_commands should be preserved: %v", rs.SetupCommands)
				}
			case "/local/repo":
				// Local-only settings are added
				if len(rs.SetupCommands) != 1 || rs.SetupCommands[0] != "make build" {
					t.Errorf("/local/repo setup_commands mismatch: %v", rs.SetupCommands)
				}
			case "/shared/repo":
				// Same repository is overridden by local
				if len(rs.SetupCommands) != 1 || rs.SetupCommands[0] != "npm install" {
					t.Errorf("/shared/repo should be overridden by local: %v", rs.SetupCommands)
				}
				if len(rs.CopyFiles) != 1 || rs.CopyFiles[0] != ".env.local" {
					t.Errorf("/shared/repo copy_files should be overridden: %v", rs.CopyFiles)
				}
			}
		}

		if !found["/global/repo"] {
			t.Error("/global/repo should be present (from global)")
		}
		if !found["/local/repo"] {
			t.Error("/local/repo should be present (from local)")
		}
		if !found["/shared/repo"] {
			t.Error("/shared/repo should be present (overridden by local)")
		}
	})

	t.Run("NoLocalRepositorySettingsKeepsGlobal", func(t *testing.T) {
		viper.Reset()
		t.Cleanup(func() { viper.Reset() })

		// Create local config without repository_settings in temp directory
		tmpDir := t.TempDir()
		localConfigPath := filepath.Join(tmpDir, ".gwq.toml")
		localConfig := []byte(`
[worktree]
basedir = "~/local-worktrees"
`)
		if err := os.WriteFile(localConfigPath, localConfig, 0644); err != nil {
			t.Fatalf("Failed to create local config: %v", err)
		}
		changeDir(t, tmpDir)

		// Load global settings
		viper.SetConfigType("toml")
		globalConfig := `
[[repository_settings]]
repository = "/global/repo1"
setup_commands = ["yarn install"]
`
		if err := viper.ReadConfig(strings.NewReader(globalConfig)); err != nil {
			t.Fatalf("Failed to read global config: %v", err)
		}

		// Merge local config
		if err := mergeLocalConfig(); err != nil {
			t.Fatalf("mergeLocalConfig() error = %v", err)
		}

		// Load as struct
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Verify global repository_settings is preserved
		if len(cfg.RepositorySettings) != 1 {
			t.Errorf("Expected 1 repository_settings (global), got %d", len(cfg.RepositorySettings))
		}
		if len(cfg.RepositorySettings) > 0 && cfg.RepositorySettings[0].Repository != "/global/repo1" {
			t.Errorf("Global repository_settings should be preserved, got %s", cfg.RepositorySettings[0].Repository)
		}

		// Verify worktree.basedir is overridden by local
		// (Check viper directly since Load() expands paths)
		if viper.GetString("worktree.basedir") != "~/local-worktrees" {
			t.Errorf("Local worktree.basedir should override global, got %s", viper.GetString("worktree.basedir"))
		}
	})
}

func TestGetLocalConfigPath(t *testing.T) {
	t.Run("FileExists", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Resolve symlinks for macOS where /var -> /private/var
		tmpDir, err := filepath.EvalSymlinks(tmpDir)
		if err != nil {
			t.Fatalf("Failed to resolve symlinks: %v", err)
		}

		localConfigPath := filepath.Join(tmpDir, ".gwq.toml")
		if err := os.WriteFile(localConfigPath, []byte{}, 0644); err != nil {
			t.Fatalf("Failed to create local config: %v", err)
		}
		changeDir(t, tmpDir)

		path := getLocalConfigPath()
		if path != localConfigPath {
			t.Errorf("getLocalConfigPath() = %s, want %s", path, localConfigPath)
		}
	})

	t.Run("FileNotExists", func(t *testing.T) {
		tmpDir := t.TempDir()
		changeDir(t, tmpDir)

		path := getLocalConfigPath()
		if path != "" {
			t.Errorf("getLocalConfigPath() = %s, want empty string", path)
		}
	})
}

func TestSetGlobalDoesNotWriteLocalSettings(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Create temp directory and isolate from real user config
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	localDir := filepath.Join(tmpDir, "local")

	// Create isolated HOME directory structure
	if err := os.MkdirAll(filepath.Join(homeDir, ".config", "gwq"), 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("Failed to create local directory: %v", err)
	}
	// Set HOME for Unix and USERPROFILE for Windows to ensure os.UserHomeDir() is isolated
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	// Create global config in isolated HOME
	globalConfigPath := filepath.Join(homeDir, ".config", "gwq", "config.toml")
	globalConfig := []byte(`
[finder]
preview = true

[ui]
icons = true
`)
	if err := os.WriteFile(globalConfigPath, globalConfig, 0644); err != nil {
		t.Fatalf("Failed to create global config: %v", err)
	}

	// Create local config with different value
	localConfigPath := filepath.Join(localDir, ".gwq.toml")
	localConfig := []byte(`
[finder]
preview = false
`)
	if err := os.WriteFile(localConfigPath, localConfig, 0644); err != nil {
		t.Fatalf("Failed to create local config: %v", err)
	}
	changeDir(t, localDir)

	// Setup viper with global config and merge local
	viper.SetConfigType("toml")
	viper.SetConfigFile(globalConfigPath)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read global config: %v", err)
	}

	// Merge local config (simulating Init behavior)
	if err := mergeLocalConfig(); err != nil {
		t.Fatalf("Failed to merge local config: %v", err)
	}

	// Verify local config is merged (finder.preview = false)
	if viper.GetBool("finder.preview") != false {
		t.Fatal("Local config should have been merged")
	}

	// Call SetGlobal with a new value (writes to isolated HOME, not real user config)
	if err := SetGlobal("ui.tilde_home", true); err != nil {
		t.Fatalf("SetGlobal() error = %v", err)
	}

	// Read the global config file directly to verify it wasn't polluted
	globalViper := viper.New()
	globalViper.SetConfigFile(globalConfigPath)
	globalViper.SetConfigType("toml")
	if err := globalViper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read global config after SetGlobal: %v", err)
	}

	// The global config should NOT have finder.preview = false (local setting)
	// It should keep its original value of true
	if globalViper.GetBool("finder.preview") != true {
		t.Error("Global config should NOT contain local settings (finder.preview should be true)")
	}

	// Verify the new value was written
	if globalViper.GetBool("ui.tilde_home") != true {
		t.Error("SetGlobal should have written the new value")
	}
}

func TestSetLocal(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Create temp directory for local config
	tmpDir := t.TempDir()
	changeDir(t, tmpDir)

	// Call SetLocal to create local config
	if err := SetLocal("finder.preview", false); err != nil {
		t.Fatalf("SetLocal() error = %v", err)
	}

	// Verify local config file was created
	localConfigPath := filepath.Join(tmpDir, ".gwq.toml")
	if _, err := os.Stat(localConfigPath); os.IsNotExist(err) {
		t.Fatal("Local config file should have been created")
	}

	// Read local config and verify the value
	localViper := viper.New()
	localViper.SetConfigFile(localConfigPath)
	localViper.SetConfigType("toml")
	if err := localViper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read local config: %v", err)
	}

	if localViper.GetBool("finder.preview") != false {
		t.Error("Local config should contain the set value (finder.preview = false)")
	}
}
