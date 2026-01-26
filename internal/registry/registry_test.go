package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorktreeEntry_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "nil expiration",
			expiresAt: nil,
			want:      false,
		},
		{
			name:      "expired",
			expiresAt: timePtr(time.Now().Add(-time.Hour)),
			want:      true,
		},
		{
			name:      "not expired",
			expiresAt: timePtr(time.Now().Add(time.Hour)),
			want:      false,
		},
		{
			name:      "just expired",
			expiresAt: timePtr(time.Now().Add(-time.Second)),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &WorktreeEntry{
				ExpiresAt: tt.expiresAt,
			}
			if got := e.IsExpired(); got != tt.want {
				t.Errorf("WorktreeEntry.IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_ListExpired(t *testing.T) {
	// Create a temporary registry
	tmpDir, err := os.MkdirTemp("", "registry-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	registryPath := filepath.Join(tmpDir, "registry.json")

	pastTime := time.Now().Add(-time.Hour)
	futureTime := time.Now().Add(time.Hour)

	entries := []*WorktreeEntry{
		{
			Path:         "/path/to/expired1",
			Branch:       "expired-branch-1",
			RegisteredAt: time.Now(),
			ExpiresAt:    &pastTime,
		},
		{
			Path:         "/path/to/not-expired",
			Branch:       "not-expired-branch",
			RegisteredAt: time.Now(),
			ExpiresAt:    &futureTime,
		},
		{
			Path:         "/path/to/expired2",
			Branch:       "expired-branch-2",
			RegisteredAt: time.Now(),
			ExpiresAt:    &pastTime,
		},
		{
			Path:         "/path/to/no-expiration",
			Branch:       "no-expiration-branch",
			RegisteredAt: time.Now(),
			ExpiresAt:    nil,
		},
	}

	// Write entries to registry file
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal entries: %v", err)
	}
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		t.Fatalf("Failed to write registry: %v", err)
	}

	// Create registry and load
	r := &Registry{
		entries: make(map[string]*WorktreeEntry),
		path:    registryPath,
	}
	if err := r.load(); err != nil {
		t.Fatalf("Failed to load registry: %v", err)
	}

	// Test ListExpired
	expired := r.ListExpired()
	if len(expired) != 2 {
		t.Errorf("ListExpired() returned %d entries, want 2", len(expired))
	}

	// Check that only expired entries are returned
	expiredPaths := make(map[string]bool)
	for _, e := range expired {
		expiredPaths[e.Path] = true
	}

	if !expiredPaths["/path/to/expired1"] {
		t.Error("ListExpired() missing /path/to/expired1")
	}
	if !expiredPaths["/path/to/expired2"] {
		t.Error("ListExpired() missing /path/to/expired2")
	}
	if expiredPaths["/path/to/not-expired"] {
		t.Error("ListExpired() should not include /path/to/not-expired")
	}
	if expiredPaths["/path/to/no-expiration"] {
		t.Error("ListExpired() should not include /path/to/no-expiration")
	}
}

func TestWorktreeEntry_ExpiresAt_JSONMarshal(t *testing.T) {
	// Test that ExpiresAt is omitted when nil (backwards compatibility)
	entry := &WorktreeEntry{
		Repository:   "https://github.com/test/repo",
		Branch:       "main",
		Path:         "/path/to/worktree",
		RegisteredAt: time.Now(),
		ExpiresAt:    nil,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	// Check that expires_at is not present in JSON
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if _, ok := m["expires_at"]; ok {
		t.Error("expires_at should be omitted when nil")
	}

	// Test that ExpiresAt is included when set
	expiresAt := time.Now().Add(time.Hour)
	entry.ExpiresAt = &expiresAt

	data, err = json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry with expiration: %v", err)
	}

	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if _, ok := m["expires_at"]; !ok {
		t.Error("expires_at should be present when set")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
