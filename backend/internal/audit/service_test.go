package audit

import (
	"context"
	"testing"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/stretchr/testify/require"
)

// TestServiceRecordSanitizesMetadata verifies audit events are stored without secret metadata.
func TestServiceRecordSanitizesMetadata(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(NewMemoryStore()).WithClock(func() time.Time {
		return now
	})

	event, err := service.Record(context.Background(), RecordRequest{
		ActorID:    "user-owner",
		ActorEmail: "person@example.test",
		Action:     ActionAuthLogin,
		TargetType: "user",
		TargetID:   "user-owner",
		Metadata: map[string]string{
			"method":   "password",
			"password": "secret",
			"token":    "secret",
			"code":     "123456",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "password", event.Metadata["method"])
	require.NotContains(t, event.Metadata, "password")
	require.NotContains(t, event.Metadata, "token")
	require.NotContains(t, event.Metadata, "code")
	require.True(t, event.CreatedAt.Equal(now))
}

// TestServiceListScopesByActor verifies audit event listing is actor scoped and paginated.
func TestServiceListScopesByActor(t *testing.T) {
	service := NewService(NewMemoryStore())
	_, err := service.Record(context.Background(), RecordRequest{
		ActorID:    "user-owner",
		Action:     ActionAuthLogin,
		TargetType: "user",
		TargetID:   "user-owner",
	})
	require.NoError(t, err)
	_, err = service.Record(context.Background(), RecordRequest{
		ActorID:    "user-other",
		Action:     ActionAuthLogin,
		TargetType: "user",
		TargetID:   "user-other",
	})
	require.NoError(t, err)

	result, err := service.List(context.Background(), ListRequest{ActorID: "user-owner", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, "user-owner", result.Items[0].ActorID)

	_, err = service.List(context.Background(), ListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}
