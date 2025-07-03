package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/d-kuro/gwq/pkg/models"
)

// createTestStorage creates a storage instance for testing with a temporary directory
func createTestStorage(t *testing.T) *Storage {
	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}
	return storage
}

func TestNewTaskManager(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}

	tm := NewTaskManager(storage, config)

	if tm == nil {
		t.Fatal("NewTaskManager() returned nil")
	}
	if tm.storage == nil {
		t.Error("storage should be set")
	}
	if tm.config == nil {
		t.Error("config should be set")
	}
	// gitClient can be nil if not in a git repository
}

func TestTaskManager_CreateTask_Valid(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	req := &CreateTaskRequest{
		Name:       "Test Task",
		Worktree:   "test-branch",
		BaseBranch: "main",
		Priority:   50,
		Prompt:     "Do something",
		DependsOn:  []string{},
	}

	// Since we're running in a git repository, this should succeed
	task, err := tm.CreateTask(req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if task == nil {
		t.Fatal("Task should not be nil")
	}

	// Verify task properties
	if task.Name != "Test Task" {
		t.Errorf("Expected Name 'Test Task', got '%s'", task.Name)
	}
	if task.Worktree != "test-branch" {
		t.Errorf("Expected Worktree 'test-branch', got '%s'", task.Worktree)
	}
	if task.Priority != Priority(50) {
		t.Errorf("Expected Priority 50, got %v", task.Priority)
	}
	if task.Status != StatusPending {
		t.Errorf("Expected Status %v, got %v", StatusPending, task.Status)
	}
	if task.Prompt != "Do something" {
		t.Errorf("Expected Prompt 'Do something', got '%s'", task.Prompt)
	}
}

func TestTaskManager_CreateTask_Validation(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	tests := []struct {
		name      string
		req       *CreateTaskRequest
		expectErr string
	}{
		{
			name: "empty name",
			req: &CreateTaskRequest{
				Name:     "",
				Worktree: "test-branch",
				Priority: 50,
			},
			expectErr: "task name is required",
		},
		{
			name: "empty worktree",
			req: &CreateTaskRequest{
				Name:     "Test Task",
				Worktree: "",
				Priority: 50,
			},
			expectErr: "worktree must be specified",
		},
		{
			name: "invalid priority low",
			req: &CreateTaskRequest{
				Name:     "Test Task",
				Worktree: "test-branch",
				Priority: 0,
			},
			expectErr: "priority must be between 1 and 100",
		},
		{
			name: "invalid priority high",
			req: &CreateTaskRequest{
				Name:     "Test Task",
				Worktree: "test-branch",
				Priority: 101,
			},
			expectErr: "priority must be between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := tm.CreateTask(tt.req)
			if err == nil {
				t.Error("Expected error but got none")
			}
			if err != nil && err.Error() != tt.expectErr {
				t.Errorf("Expected error '%s', got '%s'", tt.expectErr, err.Error())
			}
			if task != nil {
				t.Error("Task should be nil when validation fails")
			}
		})
	}
}

func TestTaskManager_CreateTasksFromFile_InvalidFile(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	// Test non-existent file
	tasks, err := tm.CreateTasksFromFile("nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if tasks != nil {
		t.Error("Tasks should be nil when file read fails")
	}
}

func TestTaskManager_CreateTasksFromFile_InvalidYAML(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	// Create temporary file with invalid YAML
	tmpDir := t.TempDir()
	invalidYAMLFile := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(invalidYAMLFile, []byte("invalid: yaml: content:"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tasks, err := tm.CreateTasksFromFile(invalidYAMLFile)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
	if tasks != nil {
		t.Error("Tasks should be nil when YAML parsing fails")
	}
}

func TestTaskManager_CreateTasksFromFile_UnsupportedVersion(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	// Create temporary file with unsupported version
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "tasks.yaml")
	yamlContent := `version: "2.0"
repository: "."
tasks:
  - id: "task1"
    name: "Test Task"
    worktree: "test-branch"
    prompt: "Do something"
`
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tasks, err := tm.CreateTasksFromFile(yamlFile)
	if err == nil {
		t.Error("Expected error for unsupported version")
	}
	if err != nil && !strings.Contains(fmt.Sprintf("%v", err), "unsupported task file version") {
		t.Errorf("Expected version error, got: %v", err)
	}
	if tasks != nil {
		t.Error("Tasks should be nil when version is unsupported")
	}
}

func TestTaskManager_FindTaskByPattern_NotFound(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	task, err := tm.FindTaskByPattern("nonexistent")
	if err == nil {
		t.Error("Expected error when task not found")
	}
	if task != nil {
		t.Error("Task should be nil when not found")
	}
}

func TestTaskManager_FindTaskByPattern_ExactMatch(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	// Add a task to storage
	testTask := &Task{
		ID:        "test-123",
		Name:      "Test Task",
		Worktree:  "test-branch",
		Status:    StatusPending,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
	}
	err := storage.SaveTask(testTask)
	if err != nil {
		t.Fatalf("Failed to save test task: %v", err)
	}

	task, err := tm.FindTaskByPattern("test-123")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if task == nil {
		t.Fatal("Task should not be nil")
	}
	if task.ID != "test-123" {
		t.Errorf("Expected task ID 'test-123', got '%s'", task.ID)
	}
}

func TestTaskManager_FindTaskByPattern_MultipleMatches(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	// Add multiple tasks that would match pattern "test"
	tasks := []*Task{
		{
			ID:        "test-123",
			Name:      "Test Task 1",
			Worktree:  "test-branch-1",
			Status:    StatusPending,
			Priority:  PriorityNormal,
			CreatedAt: time.Now(),
		},
		{
			ID:        "test-456",
			Name:      "Test Task 2",
			Worktree:  "test-branch-2",
			Status:    StatusPending,
			Priority:  PriorityNormal,
			CreatedAt: time.Now(),
		},
	}

	for _, task := range tasks {
		err := storage.SaveTask(task)
		if err != nil {
			t.Fatalf("Failed to save test task: %v", err)
		}
	}

	task, err := tm.FindTaskByPattern("test")
	if err == nil {
		t.Error("Expected error for multiple matches")
	}
	if err != nil && !strings.Contains(fmt.Sprintf("%v", err), "multiple tasks match") {
		t.Errorf("Expected multiple matches error, got: %v", err)
	}
	if task != nil {
		t.Error("Task should be nil when multiple matches found")
	}
}

func TestTaskManager_FilterTasksByStatus(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	tasks := []*Task{
		{ID: "1", Status: StatusPending},
		{ID: "2", Status: StatusRunning},
		{ID: "3", Status: StatusCompleted},
		{ID: "4", Status: StatusPending},
	}

	filtered := tm.FilterTasksByStatus(tasks, string(StatusPending))
	if len(filtered) != 2 {
		t.Errorf("Expected 2 pending tasks, got %d", len(filtered))
	}

	filtered = tm.FilterTasksByStatus(tasks, string(StatusRunning))
	if len(filtered) != 1 {
		t.Errorf("Expected 1 running task, got %d", len(filtered))
	}

	filtered = tm.FilterTasksByStatus(tasks, string(StatusFailed))
	if len(filtered) != 0 {
		t.Errorf("Expected 0 failed tasks, got %d", len(filtered))
	}
}

func TestTaskManager_FilterTasksByPriority(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	tasks := []*Task{
		{ID: "1", Priority: Priority(25)},
		{ID: "2", Priority: Priority(50)},
		{ID: "3", Priority: Priority(75)},
		{ID: "4", Priority: Priority(100)},
	}

	filtered := tm.FilterTasksByPriority(tasks, 50)
	if len(filtered) != 3 {
		t.Errorf("Expected 3 tasks with priority >= 50, got %d", len(filtered))
	}

	filtered = tm.FilterTasksByPriority(tasks, 75)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 tasks with priority >= 75, got %d", len(filtered))
	}

	filtered = tm.FilterTasksByPriority(tasks, 101)
	if len(filtered) != 0 {
		t.Errorf("Expected 0 tasks with priority >= 101, got %d", len(filtered))
	}
}

func TestTaskManager_StorageErrors(t *testing.T) {
	// This test would require mocking storage errors, which is complex with real storage
	// For now, we test that the method handles empty/non-existent patterns correctly
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	// Test with non-existent pattern
	task, err := tm.FindTaskByPattern("nonexistent-pattern")
	if err == nil {
		t.Error("Expected error for non-existent pattern")
	}
	if task != nil {
		t.Error("Task should be nil when pattern not found")
	}
}

// Test edge cases and error conditions
func TestTaskManager_EdgeCases(t *testing.T) {
	storage := createTestStorage(t)
	config := &models.Config{}
	tm := NewTaskManager(storage, config)

	// Test empty pattern search
	task, err := tm.FindTaskByPattern("")
	if err == nil {
		t.Error("Expected error for empty pattern")
	}
	if task != nil {
		t.Error("Task should be nil for empty pattern")
	}

	// Test filtering empty task list
	emptyTasks := []*Task{}
	filtered := tm.FilterTasksByStatus(emptyTasks, string(StatusPending))
	if len(filtered) != 0 {
		t.Errorf("Expected 0 tasks from empty list, got %d", len(filtered))
	}

	filtered = tm.FilterTasksByPriority(emptyTasks, 50)
	if len(filtered) != 0 {
		t.Errorf("Expected 0 tasks from empty list, got %d", len(filtered))
	}
}

// Benchmark tests
func BenchmarkTaskManager_FilterTasksByStatus(b *testing.B) {
	// For benchmarks, we don't need real storage, just the TaskManager methods
	tm := &TaskManager{}

	// Create large task list
	tasks := make([]*Task, 1000)
	for i := 0; i < 1000; i++ {
		var status Status
		switch i % 3 {
		case 0:
			status = StatusCompleted
		case 1:
			status = StatusRunning
		default:
			status = StatusPending
		}
		tasks[i] = &Task{
			ID:     fmt.Sprintf("task-%d", i),
			Status: status,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.FilterTasksByStatus(tasks, string(StatusPending))
	}
}

func BenchmarkTaskManager_FilterTasksByPriority(b *testing.B) {
	// For benchmarks, we don't need real storage, just the TaskManager methods
	tm := &TaskManager{}

	// Create large task list
	tasks := make([]*Task, 1000)
	for i := 0; i < 1000; i++ {
		tasks[i] = &Task{
			ID:       fmt.Sprintf("task-%d", i),
			Priority: Priority(i%100 + 1),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.FilterTasksByPriority(tasks, 50)
	}
}
