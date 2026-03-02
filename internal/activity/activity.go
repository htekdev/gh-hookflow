// Package activity provides tracking for multi-phase operations like git-push.
// Activities are persisted as JSON files in ~/.hookflow/activities/{id}/ to allow
// async status checking across separate CLI invocations.
package activity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Phase represents a phase in the activity lifecycle
type Phase string

const (
	PhasePrePush  Phase = "pre_push"
	PhasePush     Phase = "push"
	PhasePostPush Phase = "post_push"
)

// Status represents the status of an activity or phase
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// WorkflowStatus tracks the status of an individual workflow within a phase
type WorkflowStatus struct {
	Name        string `json:"name"`
	Status      Status `json:"status"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// PhaseStatus tracks the status of a phase
type PhaseStatus struct {
	Status      Status           `json:"status"`
	Workflows   []WorkflowStatus `json:"workflows,omitempty"`
	Output      string           `json:"output,omitempty"`
	Error       string           `json:"error,omitempty"`
	StartedAt   string           `json:"started_at,omitempty"`
	CompletedAt string           `json:"completed_at,omitempty"`
}

// Activity represents a tracked multi-phase operation
type Activity struct {
	ID        string                 `json:"id"`
	Status    Status                 `json:"status"`
	GitArgs   []string               `json:"git_args"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
	Phases    map[Phase]*PhaseStatus `json:"phases"`
	Summary   string                 `json:"summary,omitempty"`

	mu      sync.Mutex `json:"-"`
	baseDir string     `json:"-"`
}

// activitiesDir returns the base activities directory
func activitiesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".hookflow", "activities"), nil
}

// generateID creates a short random hex ID
func generateID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewActivity creates a new activity with a unique ID and persists it
func NewActivity(gitArgs []string) (*Activity, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}

	base, err := activitiesDir()
	if err != nil {
		return nil, err
	}

	actDir := filepath.Join(base, id)
	if err := os.MkdirAll(actDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create activity directory: %w", err)
	}

	// Create logs subdirectory
	if err := os.MkdirAll(filepath.Join(actDir, "logs"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	a := &Activity{
		ID:        id,
		Status:    StatusRunning,
		GitArgs:   gitArgs,
		CreatedAt: now,
		UpdatedAt: now,
		Phases: map[Phase]*PhaseStatus{
			PhasePrePush:  {Status: StatusPending},
			PhasePush:     {Status: StatusPending},
			PhasePostPush: {Status: StatusPending},
		},
		baseDir: actDir,
	}

	if err := a.save(); err != nil {
		return nil, err
	}

	return a, nil
}

// LoadActivity reads an activity from disk by ID
func LoadActivity(id string) (*Activity, error) {
	base, err := activitiesDir()
	if err != nil {
		return nil, err
	}

	actDir := filepath.Join(base, id)
	stateFile := filepath.Join(actDir, "state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("activity %q not found", id)
		}
		return nil, fmt.Errorf("failed to read activity state: %w", err)
	}

	var a Activity
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("failed to parse activity state: %w", err)
	}
	a.baseDir = actDir

	return &a, nil
}

// StartPhase marks a phase as running
func (a *Activity) StartPhase(phase Phase) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if ps, ok := a.Phases[phase]; ok {
		ps.Status = StatusRunning
		ps.StartedAt = now
	}
	a.UpdatedAt = now
	_ = a.save()
}

// CompletePhase marks a phase as completed
func (a *Activity) CompletePhase(phase Phase, success bool, output string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if ps, ok := a.Phases[phase]; ok {
		if success {
			ps.Status = StatusCompleted
		} else {
			ps.Status = StatusFailed
		}
		ps.Output = output
		ps.CompletedAt = now
	}
	a.UpdatedAt = now
	_ = a.save()
}

// FailPhase marks a phase as failed with an error
func (a *Activity) FailPhase(phase Phase, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if ps, ok := a.Phases[phase]; ok {
		ps.Status = StatusFailed
		ps.Error = errMsg
		ps.CompletedAt = now
	}
	a.UpdatedAt = now
	_ = a.save()
}

// AddWorkflowResult adds a workflow result to a phase
func (a *Activity) AddWorkflowResult(phase Phase, name string, success bool, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if ps, ok := a.Phases[phase]; ok {
		ps.Workflows = append(ps.Workflows, WorkflowStatus{
			Name:        name,
			Status:      statusFromBool(success),
			Success:     success,
			Error:       errMsg,
			CompletedAt: now,
		})
	}
	a.UpdatedAt = now
	_ = a.save()
}

// Complete marks the entire activity as completed or failed
func (a *Activity) Complete(status Status, summary string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Status = status
	a.Summary = summary
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_ = a.save()
}

// WriteLog writes log content for a specific phase and workflow
func (a *Activity) WriteLog(phase Phase, workflowName, content string) error {
	logsDir := filepath.Join(a.baseDir, "logs")
	filename := fmt.Sprintf("%s-%s.log", phase, sanitizeFilename(workflowName))
	logPath := filepath.Join(logsDir, filename)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(content)
	return err
}

// ReadLogs reads all log files for the activity
func (a *Activity) ReadLogs() (map[string]string, error) {
	logsDir := filepath.Join(a.baseDir, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	logs := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(logsDir, entry.Name()))
		if err != nil {
			continue
		}
		logs[entry.Name()] = string(content)
	}
	return logs, nil
}

// GetDir returns the activity directory path
func (a *Activity) GetDir() string {
	return a.baseDir
}

// save persists the activity state to disk
func (a *Activity) save() error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal activity state: %w", err)
	}

	stateFile := filepath.Join(a.baseDir, "state.json")
	return os.WriteFile(stateFile, data, 0644)
}

// statusFromBool converts a boolean success to a Status
func statusFromBool(success bool) Status {
	if success {
		return StatusCompleted
	}
	return StatusFailed
}

// sanitizeFilename replaces characters not safe for filenames
func sanitizeFilename(name string) string {
	replacer := []string{"/", "-", "\\", "-", " ", "_", ":", "-"}
	r := name
	for i := 0; i < len(replacer); i += 2 {
		r = replaceAll(r, replacer[i], replacer[i+1])
	}
	return r
}

func replaceAll(s, old, new string) string {
	result := s
	for {
		idx := indexOf(result, old)
		if idx == -1 {
			return result
		}
		result = result[:idx] + new + result[idx+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// CleanupOldActivities removes activities older than the given duration
func CleanupOldActivities(maxAge time.Duration) error {
	base, err := activitiesDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		actDir := filepath.Join(base, entry.Name())
		stateFile := filepath.Join(actDir, "state.json")

		info, err := os.Stat(stateFile)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(actDir)
		}
	}

	return nil
}
