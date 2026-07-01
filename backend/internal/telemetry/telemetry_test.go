package telemetry

import (
	"context"
	"testing"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/stretchr/testify/require"
)

// TestInitDisabled verifies disabled telemetry returns no provider and no error.
func TestInitDisabled(t *testing.T) {
	t.Parallel()

	provider, err := Init(context.Background(), config.TelemetryConfig{Enabled: false})

	require.NoError(t, err)
	require.Nil(t, provider)
}

// TestInitRequiresEndpoint verifies enabled telemetry fails closed without an OTLP endpoint.
func TestInitRequiresEndpoint(t *testing.T) {
	t.Parallel()

	provider, err := Init(context.Background(), config.TelemetryConfig{Enabled: true})

	require.Error(t, err)
	require.Nil(t, provider)
}
