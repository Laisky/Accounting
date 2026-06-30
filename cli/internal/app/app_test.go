package app

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRunVersion verifies that the version command writes the current CLI version.
func TestRunVersion(t *testing.T) {
	var out bytes.Buffer

	err := Run(context.Background(), []string{"version"}, &out)

	require.NoError(t, err)
	require.Equal(t, "accounting 0.1.0\n", out.String())
}

// TestRunUnknownCommand verifies that unsupported commands fail closed.
func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer

	err := Run(context.Background(), []string{"missing"}, &out)

	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown command "missing"`)
}
