package a2a

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// T-04.07  TaskStore tests
// ---------------------------------------------------------------------------

func TestTaskStore_CreateGetRoundTrip(t *testing.T) {
	store := NewTaskStore()

	task := Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status:    TaskStatus{State: TaskStateSubmitted},
		Artifacts: []Artifact{
			{ArtifactID: "art-1", Name: "output", Parts: []Part{TextPart("hello")}},
		},
		History: []Message{
			{MessageID: "msg-1", Role: RoleUser, Parts: []Part{TextPart("do something")}},
		},
	}

	require.NoError(t, store.Create(task))

	got, err := store.Get("task-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, task.ID, got.ID)
	assert.Equal(t, task.ContextID, got.ContextID)
	assert.Equal(t, task.Status.State, got.Status.State)
	require.Len(t, got.Artifacts, 1)
	assert.Equal(t, "art-1", got.Artifacts[0].ArtifactID)
	require.Len(t, got.History, 1)
	assert.Equal(t, "msg-1", got.History[0].MessageID)
}

func TestTaskStore_DuplicateCreateReturnsError(t *testing.T) {
	store := NewTaskStore()

	task := Task{ID: "dup-1", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateSubmitted}}
	require.NoError(t, store.Create(task))

	err := store.Create(task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestTaskStore_GetNonExistentReturnsError(t *testing.T) {
	store := NewTaskStore()

	got, err := store.Get("does-not-exist")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "not found")
}

func TestTaskStore_GetReturnsDeepCopy(t *testing.T) {
	store := NewTaskStore()

	task := Task{
		ID:        "deep-1",
		ContextID: "ctx-1",
		Status:    TaskStatus{State: TaskStateWorking},
		Artifacts: []Artifact{
			{ArtifactID: "art-1", Name: "original", Parts: []Part{TextPart("original text")}},
		},
		History: []Message{
			{MessageID: "msg-1", Role: RoleUser, Parts: []Part{TextPart("original msg")}},
		},
	}
	require.NoError(t, store.Create(task))

	// Get a copy and mutate it.
	copy1, err := store.Get("deep-1")
	require.NoError(t, err)
	copy1.ContextID = "mutated-ctx"
	copy1.Status.State = TaskStateFailed
	copy1.Artifacts[0].Name = "mutated"
	copy1.Artifacts = append(copy1.Artifacts, Artifact{ArtifactID: "art-extra"})
	copy1.History[0].MessageID = "mutated-msg"

	// Verify the store is unchanged.
	original, err := store.Get("deep-1")
	require.NoError(t, err)
	assert.Equal(t, "ctx-1", original.ContextID, "ContextID must not be mutated in store")
	assert.Equal(t, TaskStateWorking, original.Status.State, "Status must not be mutated in store")
	require.Len(t, original.Artifacts, 1, "Artifacts slice must not grow in store")
	assert.Equal(t, "original", original.Artifacts[0].Name, "Artifact name must not be mutated in store")
	assert.Equal(t, "msg-1", original.History[0].MessageID, "History must not be mutated in store")
}

func TestTaskStore_UpdateMutatesInPlace(t *testing.T) {
	store := NewTaskStore()

	task := Task{ID: "upd-1", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateSubmitted}}
	require.NoError(t, store.Create(task))

	err := store.Update("upd-1", func(t *Task) {
		t.Status.State = TaskStateWorking
		t.Artifacts = append(t.Artifacts, Artifact{ArtifactID: "art-new", Name: "added"})
	})
	require.NoError(t, err)

	got, err := store.Get("upd-1")
	require.NoError(t, err)
	assert.Equal(t, TaskStateWorking, got.Status.State)
	require.Len(t, got.Artifacts, 1)
	assert.Equal(t, "art-new", got.Artifacts[0].ArtifactID)
}

func TestTaskStore_UpdateNonExistentReturnsError(t *testing.T) {
	store := NewTaskStore()

	err := store.Update("ghost", func(t *Task) {
		t.Status.State = TaskStateFailed
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTaskStore_ListFiltersByContextID(t *testing.T) {
	store := NewTaskStore()

	require.NoError(t, store.Create(Task{ID: "lc-1", ContextID: "ctx-a", Status: TaskStatus{State: TaskStateSubmitted}}))
	require.NoError(t, store.Create(Task{ID: "lc-2", ContextID: "ctx-b", Status: TaskStatus{State: TaskStateSubmitted}}))
	require.NoError(t, store.Create(Task{ID: "lc-3", ContextID: "ctx-a", Status: TaskStatus{State: TaskStateWorking}}))
	require.NoError(t, store.Create(Task{ID: "lc-4", ContextID: "ctx-c", Status: TaskStatus{State: TaskStateCompleted}}))

	resp, err := store.List(ListTasksRequest{ContextID: "ctx-a"})
	require.NoError(t, err)
	require.Len(t, resp.Tasks, 2)
	assert.Equal(t, "lc-1", resp.Tasks[0].ID)
	assert.Equal(t, "lc-3", resp.Tasks[1].ID)
	assert.Equal(t, 2, resp.TotalSize)
}

func TestTaskStore_ListFiltersByStatus(t *testing.T) {
	store := NewTaskStore()

	require.NoError(t, store.Create(Task{ID: "ls-1", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateSubmitted}}))
	require.NoError(t, store.Create(Task{ID: "ls-2", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateWorking}}))
	require.NoError(t, store.Create(Task{ID: "ls-3", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateCompleted}}))
	require.NoError(t, store.Create(Task{ID: "ls-4", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateWorking}}))

	resp, err := store.List(ListTasksRequest{Status: "working"})
	require.NoError(t, err)
	require.Len(t, resp.Tasks, 2)
	assert.Equal(t, "ls-2", resp.Tasks[0].ID)
	assert.Equal(t, "ls-4", resp.Tasks[1].ID)
	assert.Equal(t, 2, resp.TotalSize)
}

func TestTaskStore_ListPagination(t *testing.T) {
	store := NewTaskStore()

	// Create 5 tasks.
	for i := 1; i <= 5; i++ {
		require.NoError(t, store.Create(Task{
			ID:        fmt.Sprintf("pg-%d", i),
			ContextID: "ctx-pg",
			Status:    TaskStatus{State: TaskStateSubmitted},
		}))
	}

	// Page 1: first 2.
	resp1, err := store.List(ListTasksRequest{PageSize: 2})
	require.NoError(t, err)
	require.Len(t, resp1.Tasks, 2)
	assert.Equal(t, "pg-1", resp1.Tasks[0].ID)
	assert.Equal(t, "pg-2", resp1.Tasks[1].ID)
	assert.Equal(t, 5, resp1.TotalSize)
	assert.NotEmpty(t, resp1.NextPageToken, "should have a next page token")

	// Page 2: next 2.
	resp2, err := store.List(ListTasksRequest{PageSize: 2, PageToken: resp1.NextPageToken})
	require.NoError(t, err)
	require.Len(t, resp2.Tasks, 2)
	assert.Equal(t, "pg-3", resp2.Tasks[0].ID)
	assert.Equal(t, "pg-4", resp2.Tasks[1].ID)
	assert.Equal(t, 5, resp2.TotalSize)
	assert.NotEmpty(t, resp2.NextPageToken)

	// Page 3: last 1.
	resp3, err := store.List(ListTasksRequest{PageSize: 2, PageToken: resp2.NextPageToken})
	require.NoError(t, err)
	require.Len(t, resp3.Tasks, 1)
	assert.Equal(t, "pg-5", resp3.Tasks[0].ID)
	assert.Equal(t, 5, resp3.TotalSize)
	assert.Empty(t, resp3.NextPageToken, "no more pages")
}

func TestTaskStore_ListInvalidPageToken(t *testing.T) {
	store := NewTaskStore()

	require.NoError(t, store.Create(Task{ID: "pt-1", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateSubmitted}}))

	_, err := store.List(ListTasksRequest{PageToken: "bogus-token"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid page token")
}

func TestTaskStore_ConcurrentAccess(t *testing.T) {
	store := NewTaskStore()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines create tasks.
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("conc-%d", idx)
			_ = store.Create(Task{
				ID:        id,
				ContextID: "ctx-conc",
				Status:    TaskStatus{State: TaskStateSubmitted},
			})
		}(i)
	}

	// The other half read/list tasks concurrently.
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("conc-%d", idx)
			// Get may fail if the task hasn't been created yet; that's fine.
			_, _ = store.Get(id)
			_, _ = store.List(ListTasksRequest{ContextID: "ctx-conc"})
		}(i)
	}

	wg.Wait()

	// Verify all tasks were eventually created.
	resp, err := store.List(ListTasksRequest{ContextID: "ctx-conc"})
	require.NoError(t, err)
	assert.Equal(t, goroutines, len(resp.Tasks), "all goroutine tasks should be present")
}

func TestNewTaskID_Uniqueness(t *testing.T) {
	const count = 1000
	ids := make(map[string]struct{}, count)

	for i := 0; i < count; i++ {
		id := NewTaskID()
		assert.NotEmpty(t, id, "generated ID must not be empty")
		_, exists := ids[id]
		assert.False(t, exists, "duplicate ID detected: %s", id)
		ids[id] = struct{}{}
	}

	assert.Len(t, ids, count, "all 1000 IDs must be unique")
}

func TestTaskStore_ListEmptyStore(t *testing.T) {
	store := NewTaskStore()

	resp, err := store.List(ListTasksRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Tasks)
	assert.Equal(t, 0, resp.TotalSize)
	assert.Empty(t, resp.NextPageToken)
}

func TestTaskStore_ListCombinedFilters(t *testing.T) {
	store := NewTaskStore()

	require.NoError(t, store.Create(Task{ID: "cf-1", ContextID: "ctx-x", Status: TaskStatus{State: TaskStateWorking}}))
	require.NoError(t, store.Create(Task{ID: "cf-2", ContextID: "ctx-x", Status: TaskStatus{State: TaskStateCompleted}}))
	require.NoError(t, store.Create(Task{ID: "cf-3", ContextID: "ctx-y", Status: TaskStatus{State: TaskStateWorking}}))
	require.NoError(t, store.Create(Task{ID: "cf-4", ContextID: "ctx-x", Status: TaskStatus{State: TaskStateWorking}}))

	// Filter by both contextID and status.
	resp, err := store.List(ListTasksRequest{ContextID: "ctx-x", Status: "working"})
	require.NoError(t, err)
	require.Len(t, resp.Tasks, 2)
	assert.Equal(t, "cf-1", resp.Tasks[0].ID)
	assert.Equal(t, "cf-4", resp.Tasks[1].ID)
}

func TestTaskStore_UpdateMultipleTimes(t *testing.T) {
	store := NewTaskStore()

	require.NoError(t, store.Create(Task{ID: "um-1", ContextID: "ctx-1", Status: TaskStatus{State: TaskStateSubmitted}}))

	// First update.
	require.NoError(t, store.Update("um-1", func(t *Task) {
		t.Status.State = TaskStateWorking
	}))

	// Second update.
	require.NoError(t, store.Update("um-1", func(t *Task) {
		t.Status.State = TaskStateCompleted
		t.Artifacts = []Artifact{{ArtifactID: "final", Name: "result"}}
	}))

	got, err := store.Get("um-1")
	require.NoError(t, err)
	assert.Equal(t, TaskStateCompleted, got.Status.State)
	require.Len(t, got.Artifacts, 1)
	assert.Equal(t, "final", got.Artifacts[0].ArtifactID)
}
