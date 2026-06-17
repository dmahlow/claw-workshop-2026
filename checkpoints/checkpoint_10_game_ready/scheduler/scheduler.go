package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Task represents a scheduled task.
type Task struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	ExecuteAt   time.Time     `json:"execute_at"`
	Recurring   bool          `json:"recurring"`
	Interval    time.Duration `json:"interval"`
}

// MarshalJSON provides custom JSON marshaling for the Interval field.
func (t Task) MarshalJSON() ([]byte, error) {
	type Alias Task
	return json.Marshal(&struct {
		Alias
		Interval string `json:"interval"`
	}{
		Alias:    (Alias)(t),
		Interval: t.Interval.String(),
	})
}

// UnmarshalJSON provides custom JSON unmarshaling for the Interval field.
func (t *Task) UnmarshalJSON(data []byte) error {
	type Alias Task
	aux := &struct {
		*Alias
		Interval string `json:"interval"`
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Interval != "" {
		d, err := time.ParseDuration(aux.Interval)
		if err != nil {
			return fmt.Errorf("parsing interval: %w", err)
		}
		t.Interval = d
	}
	return nil
}

// Scheduler manages scheduled tasks and fires them when due.
type Scheduler struct {
	mu       sync.Mutex
	tasks    []Task
	filePath string
	counter  int
	callback func(description string)
}

// New creates a new scheduler that persists tasks to the given file path.
// The callback is invoked (in the scheduler goroutine) when a task fires.
func New(filePath string, callback func(description string)) (*Scheduler, error) {
	s := &Scheduler{
		filePath: filePath,
		callback: callback,
	}

	// Create parent directory if needed.
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating scheduler directory: %w", err)
	}

	// Load existing tasks.
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading tasks: %w", err)
	}

	return s, nil
}

// Add schedules a new task. Returns the task ID.
func (s *Scheduler) Add(description string, delay time.Duration, recurring bool) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	task := Task{
		ID:          fmt.Sprintf("task_%d", s.counter),
		Description: description,
		ExecuteAt:   time.Now().Add(delay),
		Recurring:   recurring,
		Interval:    delay,
	}
	s.tasks = append(s.tasks, task)

	if err := s.save(); err != nil {
		return Task{}, fmt.Errorf("saving tasks: %w", err)
	}

	return task, nil
}

// List returns all scheduled tasks.
func (s *Scheduler) List() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Task, len(s.tasks))
	copy(result, s.tasks)
	return result
}

// Run starts the scheduler loop. It checks for due tasks on a regular tick
// and fires the callback for each one. It blocks until the context is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndFire()
		}
	}
}

// Save persists current tasks to disk. Called externally during shutdown.
func (s *Scheduler) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *Scheduler) checkAndFire() {
	s.mu.Lock()
	now := time.Now()

	var due []Task
	var remaining []Task

	for _, t := range s.tasks {
		if now.After(t.ExecuteAt) || now.Equal(t.ExecuteAt) {
			due = append(due, t)
			if t.Recurring {
				// Reschedule.
				next := t
				next.ExecuteAt = now.Add(t.Interval)
				remaining = append(remaining, next)
			}
		} else {
			remaining = append(remaining, t)
		}
	}

	s.tasks = remaining
	// Best-effort save after firing.
	_ = s.save()
	s.mu.Unlock()

	// Fire callbacks outside the lock.
	for _, t := range due {
		s.callback(t.Description)
	}
}

func (s *Scheduler) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var state struct {
		Tasks   []Task `json:"tasks"`
		Counter int    `json:"counter"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parsing tasks file: %w", err)
	}
	s.tasks = state.Tasks
	s.counter = state.Counter
	return nil
}

func (s *Scheduler) save() error {
	state := struct {
		Tasks   []Task `json:"tasks"`
		Counter int    `json:"counter"`
	}{
		Tasks:   s.tasks,
		Counter: s.counter,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tasks: %w", err)
	}
	return os.WriteFile(s.filePath, data, 0o644)
}
