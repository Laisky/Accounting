package ledger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServiceSummary verifies that the scaffold service returns a deterministic opening summary.
func TestServiceSummary(t *testing.T) {
	service := NewService()

	summary := service.Summary(context.Background())

	require.Equal(t, int64(0), summary.BalanceCents)
	require.Equal(t, "USD", summary.Currency)
	require.Equal(t, 1, summary.EntryCount)
}
