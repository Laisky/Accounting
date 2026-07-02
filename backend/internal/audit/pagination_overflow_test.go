package audit

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestListRejectsPaginationOverflowWithoutPanic guards the fix for an integer
// overflow: a near-max page makes (page-1)*pageSize wrap negative, which would
// produce a negative slice bound and panic (HTTP 500). The service must instead
// treat such a page as past the end and return an empty page.
func TestListRejectsPaginationOverflowWithoutPanic(t *testing.T) {
	service := NewService(NewMemoryStore())
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := service.Record(ctx, RecordRequest{
			ActorID:    "actor-1",
			Action:     ActionAuthLogin,
			TargetType: "user",
			TargetID:   "actor-1",
		})
		require.NoError(t, err)
	}

	result, err := service.List(ctx, ListRequest{ActorID: "actor-1", Page: math.MaxInt64, PageSize: maxPageSize})
	require.NoError(t, err)
	require.Empty(t, result.Items)
	require.Equal(t, 3, result.Total)
}
