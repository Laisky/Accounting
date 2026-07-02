package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestContextWithSessionStoresActorAndSession verifies auth context helpers round-trip identity data.
func TestContextWithSessionStoresActorAndSession(t *testing.T) {
	session := Session{
		ID:        "session-id",
		UserID:    "user-id",
		UserEmail: "person@example.test",
		Status:    UserStatusActive,
		CreatedAt: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
	}

	ctx := ContextWithSession(context.Background(), session)

	actor, ok := ActorFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "user-id", actor.UserID)
	require.Equal(t, "person@example.test", actor.Email)
	require.Equal(t, UserStatusActive, actor.Status)

	storedSession, ok := SessionFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, session, storedSession)
}

// TestActorFromContextMissing verifies missing auth context data fails closed.
func TestActorFromContextMissing(t *testing.T) {
	_, ok := ActorFromContext(context.Background())
	require.False(t, ok)

	_, ok = SessionFromContext(context.Background())
	require.False(t, ok)
}

// TestActorFromContextWrongType verifies unrelated context values do not authenticate a request.
func TestActorFromContextWrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), actorContextKey, "not an actor")
	ctx = context.WithValue(ctx, sessionContextKey, "not a session")

	_, ok := ActorFromContext(ctx)
	require.False(t, ok)

	_, ok = SessionFromContext(ctx)
	require.False(t, ok)
}
