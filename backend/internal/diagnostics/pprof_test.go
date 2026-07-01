package diagnostics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsLoopbackListenAddr verifies loopback and non-loopback pprof listener detection.
func TestIsLoopbackListenAddr(t *testing.T) {
	t.Parallel()

	require.True(t, IsLoopbackListenAddr("localhost:6060"))
	require.True(t, IsLoopbackListenAddr("127.0.0.1:6060"))
	require.True(t, IsLoopbackListenAddr("[::1]:6060"))
	require.False(t, IsLoopbackListenAddr(":6060"))
	require.False(t, IsLoopbackListenAddr("0.0.0.0:6060"))
	require.False(t, IsLoopbackListenAddr("10.0.0.1:6060"))
}
