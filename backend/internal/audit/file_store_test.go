package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFileStorePersistsEvents verifies audit events survive reopening the JSON store.
func TestFileStorePersistsEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.json")
	store, err := NewFileStore(path)
	require.NoError(t, err)

	_, err = store.SaveEvent(context.Background(), Event{
		ID:        "event-1",
		Seq:       1,
		ActorID:   "user-1",
		Action:    ActionAuthLogin,
		CreatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	reopened, err := NewFileStore(path)
	require.NoError(t, err)
	events, err := reopened.EventsByActor(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "event-1", events[0].ID)
	tail, err := reopened.Tail(context.Background())
	require.NoError(t, err)
	require.Equal(t, "event-1", tail.ID)
	all, err := reopened.AllEvents(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"event-1"}, []string{all[0].ID})
}
