package a2a

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
)

// NewTaskID generates a UUID v4 string using crypto/rand.
func NewTaskID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	// Set version 4 (bits 12-15 of time_hi_and_version).
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant bits (bits 6-7 of clock_seq_hi_and_reserved).
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// TaskStore is a concurrency-safe in-memory store for agent-side task tracking.
// Tasks are stored in a map keyed by ID with a separate slice maintaining
// insertion order for deterministic pagination.
type TaskStore struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	orderIDs []string // insertion-order task IDs
}

// NewTaskStore returns an initialized TaskStore ready for use.
func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks:    make(map[string]*Task),
		orderIDs: make([]string, 0),
	}
}

// Create stores a new task. It returns an error if a task with the same ID
// already exists.
func (s *TaskStore) Create(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task %q already exists", task.ID)
	}
	s.tasks[task.ID] = &task
	s.orderIDs = append(s.orderIDs, task.ID)
	return nil
}

// Get returns a deep copy of the task with the given ID. It returns an error
// if no task with that ID is found. The returned copy is safe to mutate without
// affecting the store.
func (s *TaskStore) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return deepCopyTask(t), nil
}

// Update applies the mutation function fn to the task identified by id under
// a write lock. The function receives the actual stored task pointer, so all
// mutations are applied in-place. It returns an error if the task is not found.
func (s *TaskStore) Update(id string, fn func(*Task)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	fn(t)
	return nil
}

// List returns tasks matching the filter criteria with pagination support.
//
// Filtering:
//   - If ContextID is non-empty, only tasks with that context ID are included.
//   - If Status is non-empty, only tasks whose current state matches are included.
//
// Pagination:
//   - PageToken is the ID of the last task from the previous page; results
//     start after that task in insertion order.
//   - PageSize <= 0 means return all matching tasks (no pagination).
func (s *TaskStore) List(filter ListTasksRequest) (*ListTasksResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Determine where to start based on page token.
	startIdx := 0
	if filter.PageToken != "" {
		found := false
		for i, id := range s.orderIDs {
			if id == filter.PageToken {
				startIdx = i + 1
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid page token %q", filter.PageToken)
		}
	}

	// Collect all matching tasks (for total count) and the page slice.
	var matched []Task
	for i := startIdx; i < len(s.orderIDs); i++ {
		t := s.tasks[s.orderIDs[i]]
		if !matchesFilter(t, filter) {
			continue
		}
		matched = append(matched, *deepCopyTask(t))
	}

	// Also count matches before startIdx for the total size.
	totalBefore := 0
	for i := 0; i < startIdx; i++ {
		t := s.tasks[s.orderIDs[i]]
		if matchesFilter(t, filter) {
			totalBefore++
		}
	}

	totalSize := totalBefore + len(matched)

	// Apply page size.
	var nextPageToken string
	if filter.PageSize > 0 && len(matched) > filter.PageSize {
		nextPageToken = matched[filter.PageSize-1].ID
		matched = matched[:filter.PageSize]
	}

	if matched == nil {
		matched = []Task{}
	}

	return &ListTasksResponse{
		Tasks:         matched,
		TotalSize:     totalSize,
		NextPageToken: nextPageToken,
	}, nil
}

// matchesFilter returns true if the task passes the context ID and status
// filters specified in the request.
func matchesFilter(t *Task, filter ListTasksRequest) bool {
	if filter.ContextID != "" && t.ContextID != filter.ContextID {
		return false
	}
	if filter.Status != "" && string(t.Status.State) != filter.Status {
		return false
	}
	return true
}

// deepCopyTask returns a new Task that is a deep copy of src. Slice fields
// (Artifacts, History) and the Metadata byte slice are independently copied.
func deepCopyTask(src *Task) *Task {
	dst := *src

	if src.Artifacts != nil {
		dst.Artifacts = make([]Artifact, len(src.Artifacts))
		for i, a := range src.Artifacts {
			dst.Artifacts[i] = deepCopyArtifact(a)
		}
	}

	if src.History != nil {
		dst.History = make([]Message, len(src.History))
		for i, m := range src.History {
			dst.History[i] = deepCopyMessage(m)
		}
	}

	if src.Metadata != nil {
		dst.Metadata = make(json.RawMessage, len(src.Metadata))
		copy(dst.Metadata, src.Metadata)
	}

	// Deep copy the status message if present.
	if src.Status.Message != nil {
		msgCopy := deepCopyMessage(*src.Status.Message)
		dst.Status.Message = &msgCopy
	}

	return &dst
}

// deepCopyMessage returns a deep copy of a Message.
func deepCopyMessage(src Message) Message {
	dst := src

	if src.Parts != nil {
		dst.Parts = make([]Part, len(src.Parts))
		for i, p := range src.Parts {
			dst.Parts[i] = deepCopyPart(p)
		}
	}

	if src.Metadata != nil {
		dst.Metadata = make(json.RawMessage, len(src.Metadata))
		copy(dst.Metadata, src.Metadata)
	}

	if src.Extensions != nil {
		dst.Extensions = make([]string, len(src.Extensions))
		copy(dst.Extensions, src.Extensions)
	}

	if src.ReferenceTaskIDs != nil {
		dst.ReferenceTaskIDs = make([]string, len(src.ReferenceTaskIDs))
		copy(dst.ReferenceTaskIDs, src.ReferenceTaskIDs)
	}

	return dst
}

// deepCopyPart returns a deep copy of a Part.
func deepCopyPart(src Part) Part {
	dst := src

	if src.Raw != nil {
		dst.Raw = make([]byte, len(src.Raw))
		copy(dst.Raw, src.Raw)
	}

	if src.Data != nil {
		dst.Data = make(json.RawMessage, len(src.Data))
		copy(dst.Data, src.Data)
	}

	if src.Metadata != nil {
		dst.Metadata = make(json.RawMessage, len(src.Metadata))
		copy(dst.Metadata, src.Metadata)
	}

	return dst
}

// deepCopyArtifact returns a deep copy of an Artifact.
func deepCopyArtifact(src Artifact) Artifact {
	dst := src

	if src.Parts != nil {
		dst.Parts = make([]Part, len(src.Parts))
		for i, p := range src.Parts {
			dst.Parts[i] = deepCopyPart(p)
		}
	}

	if src.Metadata != nil {
		dst.Metadata = make(json.RawMessage, len(src.Metadata))
		copy(dst.Metadata, src.Metadata)
	}

	if src.Extensions != nil {
		dst.Extensions = make([]string, len(src.Extensions))
		copy(dst.Extensions, src.Extensions)
	}

	return dst
}
