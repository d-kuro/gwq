package claude

import (
	"testing"
	"time"
)

func TestNewSimplifiedTask(t *testing.T) {
	id := "test-123"
	name := "Test Task"
	worktree := "test-branch"
	prompt := "Do something"
	priority := PriorityHigh

	task := NewSimplifiedTask(id, name, worktree, prompt, priority)

	if task == nil {
		t.Fatal("NewSimplifiedTask() returned nil")
	}
	if task.ID != id {
		t.Errorf("Expected ID '%s', got '%s'", id, task.ID)
	}
	if task.Name != name {
		t.Errorf("Expected Name '%s', got '%s'", name, task.Name)
	}
	if task.Worktree != worktree {
		t.Errorf("Expected Worktree '%s', got '%s'", worktree, task.Worktree)
	}
	if task.Prompt != prompt {
		t.Errorf("Expected Prompt '%s', got '%s'", prompt, task.Prompt)
	}
	if task.Priority != priority {
		t.Errorf("Expected Priority %v, got %v", priority, task.Priority)
	}
	if task.Status != StatusPending {
		t.Errorf("Expected Status %v, got %v", StatusPending, task.Status)
	}
	if task.DependsOn == nil {
		t.Error("DependsOn should be initialized")
	}
	if len(task.DependsOn) != 0 {
		t.Errorf("Expected empty DependsOn, got %v", task.DependsOn)
	}
	if task.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestSimplifiedTask_GetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		taskName string
		prompt   string
		expected string
	}{
		{
			name:     "with name",
			taskName: "Test Task",
			prompt:   "Do something",
			expected: "Test Task",
		},
		{
			name:     "without name, short prompt",
			taskName: "",
			prompt:   "Short prompt",
			expected: "Short prompt",
		},
		{
			name:     "without name, long prompt",
			taskName: "",
			prompt:   "This is a very long prompt that should be truncated because it exceeds the maximum length",
			expected: "This is a very long prompt that should be trunc...",
		},
		{
			name:     "empty name and prompt",
			taskName: "",
			prompt:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SimplifiedTask{
				Name:   tt.taskName,
				Prompt: tt.prompt,
			}
			result := task.GetDisplayName()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSimplifiedTask_StatusChecks(t *testing.T) {
	tests := []struct {
		name        string
		status      Status
		isCompleted bool
		isFailed    bool
		isRunning   bool
	}{
		{
			name:        "pending",
			status:      StatusPending,
			isCompleted: false,
			isFailed:    false,
			isRunning:   false,
		},
		{
			name:        "running",
			status:      StatusRunning,
			isCompleted: false,
			isFailed:    false,
			isRunning:   true,
		},
		{
			name:        "completed",
			status:      StatusCompleted,
			isCompleted: true,
			isFailed:    false,
			isRunning:   false,
		},
		{
			name:        "failed",
			status:      StatusFailed,
			isCompleted: false,
			isFailed:    true,
			isRunning:   false,
		},
		{
			name:        "cancelled",
			status:      StatusCancelled,
			isCompleted: false,
			isFailed:    false,
			isRunning:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SimplifiedTask{Status: tt.status}

			if task.IsCompleted() != tt.isCompleted {
				t.Errorf("IsCompleted() = %v, expected %v", task.IsCompleted(), tt.isCompleted)
			}
			if task.IsFailed() != tt.isFailed {
				t.Errorf("IsFailed() = %v, expected %v", task.IsFailed(), tt.isFailed)
			}
			if task.IsRunning() != tt.isRunning {
				t.Errorf("IsRunning() = %v, expected %v", task.IsRunning(), tt.isRunning)
			}
		})
	}
}

func TestSimplifiedTask_GetDuration(t *testing.T) {
	tests := []struct {
		name     string
		result   *TaskResult
		expected time.Duration
	}{
		{
			name:     "no result",
			result:   nil,
			expected: 0,
		},
		{
			name: "with result",
			result: &TaskResult{
				Duration: 5 * time.Minute,
			},
			expected: 5 * time.Minute,
		},
		{
			name: "zero duration",
			result: &TaskResult{
				Duration: 0,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SimplifiedTask{Result: tt.result}
			duration := task.GetDuration()
			if duration != tt.expected {
				t.Errorf("GetDuration() = %v, expected %v", duration, tt.expected)
			}
		})
	}
}

func TestSimplifiedTask_ToLegacyTask(t *testing.T) {
	createdAt := time.Now()
	task := &SimplifiedTask{
		ID:        "test-123",
		Name:      "Test Task",
		Worktree:  "test-branch",
		Priority:  PriorityHigh,
		Status:    StatusPending,
		CreatedAt: createdAt,
		Prompt:    "Do something",
		DependsOn: []string{"dep1", "dep2"},
		Result:    nil,
	}

	legacyTask := task.ToLegacyTask()

	if legacyTask == nil {
		t.Fatal("ToLegacyTask() returned nil")
	}
	if legacyTask.ID != task.ID {
		t.Errorf("Expected ID '%s', got '%s'", task.ID, legacyTask.ID)
	}
	if legacyTask.Name != task.Name {
		t.Errorf("Expected Name '%s', got '%s'", task.Name, legacyTask.Name)
	}
	if legacyTask.Worktree != task.Worktree {
		t.Errorf("Expected Worktree '%s', got '%s'", task.Worktree, legacyTask.Worktree)
	}
	if legacyTask.Priority != task.Priority {
		t.Errorf("Expected Priority %v, got %v", task.Priority, legacyTask.Priority)
	}
	if legacyTask.Status != task.Status {
		t.Errorf("Expected Status %v, got %v", task.Status, legacyTask.Status)
	}
	if !legacyTask.CreatedAt.Equal(task.CreatedAt) {
		t.Errorf("Expected CreatedAt %v, got %v", task.CreatedAt, legacyTask.CreatedAt)
	}
	if legacyTask.Prompt != task.Prompt {
		t.Errorf("Expected Prompt '%s', got '%s'", task.Prompt, legacyTask.Prompt)
	}
	if len(legacyTask.DependsOn) != len(task.DependsOn) {
		t.Errorf("Expected DependsOn length %d, got %d", len(task.DependsOn), len(legacyTask.DependsOn))
	}

	// Check default values
	if legacyTask.AgentType != "claude" {
		t.Errorf("Expected AgentType 'claude', got '%s'", legacyTask.AgentType)
	}
	if legacyTask.DependencyPolicy != DependencyPolicyWait {
		t.Errorf("Expected DependencyPolicy %v, got %v", DependencyPolicyWait, legacyTask.DependencyPolicy)
	}
	if !legacyTask.Config.SkipPermissions {
		t.Error("Expected SkipPermissions to be true")
	}
	if legacyTask.Config.AutoCommit {
		t.Error("Expected AutoCommit to be false")
	}
	if legacyTask.Config.BackupFiles {
		t.Error("Expected BackupFiles to be false")
	}
}

func TestSimplifiedTask_ToLegacyTask_WithResult(t *testing.T) {
	createdAt := time.Now()
	duration := 5 * time.Minute
	task := &SimplifiedTask{
		ID:        "test-123",
		Name:      "Test Task",
		Worktree:  "test-branch",
		Priority:  PriorityHigh,
		Status:    StatusCompleted,
		CreatedAt: createdAt,
		Prompt:    "Do something",
		DependsOn: []string{},
		Result: &TaskResult{
			Duration: duration,
			ExitCode: 0,
		},
	}

	legacyTask := task.ToLegacyTask()

	if legacyTask.CompletedAt == nil {
		t.Error("CompletedAt should be set for completed task with result")
	} else {
		expectedCompletedAt := createdAt.Add(duration)
		if !legacyTask.CompletedAt.Equal(expectedCompletedAt) {
			t.Errorf("Expected CompletedAt %v, got %v", expectedCompletedAt, *legacyTask.CompletedAt)
		}
	}
}

func TestFromLegacyTask(t *testing.T) {
	createdAt := time.Now()
	startedAt := createdAt.Add(1 * time.Minute)
	completedAt := createdAt.Add(6 * time.Minute)

	legacyTask := &Task{
		ID:          "test-123",
		Name:        "Test Task",
		Worktree:    "test-branch",
		Priority:    PriorityHigh,
		Status:      StatusCompleted,
		CreatedAt:   createdAt,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		Prompt:      "Do something",
		DependsOn:   []string{"dep1"},
		Result: &TaskResult{
			Duration: 3 * time.Minute, // This will be recalculated
			ExitCode: 0,
		},
	}

	task := FromLegacyTask(legacyTask)

	if task == nil {
		t.Fatal("FromLegacyTask() returned nil")
	}
	if task.ID != legacyTask.ID {
		t.Errorf("Expected ID '%s', got '%s'", legacyTask.ID, task.ID)
	}
	if task.Name != legacyTask.Name {
		t.Errorf("Expected Name '%s', got '%s'", legacyTask.Name, task.Name)
	}
	if task.Worktree != legacyTask.Worktree {
		t.Errorf("Expected Worktree '%s', got '%s'", legacyTask.Worktree, task.Worktree)
	}
	if task.Priority != legacyTask.Priority {
		t.Errorf("Expected Priority %v, got %v", legacyTask.Priority, task.Priority)
	}
	if task.Status != legacyTask.Status {
		t.Errorf("Expected Status %v, got %v", legacyTask.Status, task.Status)
	}
	if !task.CreatedAt.Equal(legacyTask.CreatedAt) {
		t.Errorf("Expected CreatedAt %v, got %v", legacyTask.CreatedAt, task.CreatedAt)
	}
	if task.Prompt != legacyTask.Prompt {
		t.Errorf("Expected Prompt '%s', got '%s'", legacyTask.Prompt, task.Prompt)
	}
	if len(task.DependsOn) != len(legacyTask.DependsOn) {
		t.Errorf("Expected DependsOn length %d, got %d", len(legacyTask.DependsOn), len(task.DependsOn))
	}

	// Check that duration was recalculated from timing fields
	if task.Result == nil {
		t.Fatal("Result should not be nil")
	}
	expectedDuration := completedAt.Sub(startedAt)
	if task.Result.Duration != expectedDuration {
		t.Errorf("Expected duration %v, got %v", expectedDuration, task.Result.Duration)
	}
}

func TestFromLegacyTask_NoTiming(t *testing.T) {
	legacyTask := &Task{
		ID:        "test-123",
		Name:      "Test Task",
		Worktree:  "test-branch",
		Priority:  PriorityHigh,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		Prompt:    "Do something",
		DependsOn: []string{},
		Result:    nil,
	}

	task := FromLegacyTask(legacyTask)

	if task == nil {
		t.Fatal("FromLegacyTask() returned nil")
	}
	if task.Result != nil {
		t.Error("Result should be nil when legacy task has no result")
	}
}

// Test round-trip conversion
func TestSimplifiedTask_RoundTrip(t *testing.T) {
	original := NewSimplifiedTask("test-123", "Test Task", "test-branch", "Do something", PriorityHigh)
	original.DependsOn = []string{"dep1", "dep2"}
	original.Status = StatusRunning

	// Convert to legacy and back
	legacyTask := original.ToLegacyTask()
	roundTrip := FromLegacyTask(legacyTask)

	// Compare essential fields
	if roundTrip.ID != original.ID {
		t.Errorf("ID mismatch: %s != %s", roundTrip.ID, original.ID)
	}
	if roundTrip.Name != original.Name {
		t.Errorf("Name mismatch: %s != %s", roundTrip.Name, original.Name)
	}
	if roundTrip.Worktree != original.Worktree {
		t.Errorf("Worktree mismatch: %s != %s", roundTrip.Worktree, original.Worktree)
	}
	if roundTrip.Priority != original.Priority {
		t.Errorf("Priority mismatch: %v != %v", roundTrip.Priority, original.Priority)
	}
	if roundTrip.Status != original.Status {
		t.Errorf("Status mismatch: %v != %v", roundTrip.Status, original.Status)
	}
	if roundTrip.Prompt != original.Prompt {
		t.Errorf("Prompt mismatch: %s != %s", roundTrip.Prompt, original.Prompt)
	}
	if len(roundTrip.DependsOn) != len(original.DependsOn) {
		t.Errorf("DependsOn length mismatch: %d != %d", len(roundTrip.DependsOn), len(original.DependsOn))
	}
}

// Benchmark tests
func BenchmarkNewSimplifiedTask(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewSimplifiedTask("test-123", "Test Task", "test-branch", "Do something", PriorityHigh)
	}
}

func BenchmarkSimplifiedTask_ToLegacyTask(b *testing.B) {
	task := NewSimplifiedTask("test-123", "Test Task", "test-branch", "Do something", PriorityHigh)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task.ToLegacyTask()
	}
}

func BenchmarkFromLegacyTask(b *testing.B) {
	legacyTask := &Task{
		ID:        "test-123",
		Name:      "Test Task",
		Worktree:  "test-branch",
		Priority:  PriorityHigh,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		Prompt:    "Do something",
		DependsOn: []string{},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FromLegacyTask(legacyTask)
	}
}
