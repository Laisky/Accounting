package audit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// TestSQLStoreEventsByActorPersists verifies audit events are read from SQL and
// returned newest first for the requested actor.
func TestSQLStoreEventsByActorPersists(t *testing.T) {
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	first := Event{ID: "event_1", Seq: 1, ActorID: "user_1", Action: ActionAuthLogin, CreatedAt: time.Now().UTC()}
	second := Event{ID: "event_2", Seq: 2, ActorID: "user_1", Action: ActionAuthLogout, CreatedAt: first.CreatedAt.Add(time.Second)}
	other := Event{ID: "event_3", Seq: 3, ActorID: "user_2", Action: ActionAuthLogin, CreatedAt: second.CreatedAt.Add(time.Second)}
	_, err = store.SaveEvent(context.Background(), first)
	require.NoError(t, err)
	_, err = store.SaveEvent(context.Background(), second)
	require.NoError(t, err)
	_, err = store.SaveEvent(context.Background(), other)
	require.NoError(t, err)

	reopened := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	events, err := reopened.EventsByActor(context.Background(), "user_1")
	require.NoError(t, err)
	require.Equal(t, []string{"event_2", "event_1"}, []string{events[0].ID, events[1].ID})
	all, err := reopened.AllEvents(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"event_3", "event_2", "event_1"}, []string{all[0].ID, all[1].ID, all[2].ID})
	tail, err := reopened.Tail(context.Background())
	require.NoError(t, err)
	require.Equal(t, "event_3", tail.ID)
}
