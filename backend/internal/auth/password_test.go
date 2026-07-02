package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHashPasswordUsesPBKDF2SHA256 verifies password hashes use the configured PBKDF2-SHA256 format.
func TestHashPasswordUsesPBKDF2SHA256(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(hash, "$pbkdf2-sha256$i=600000,l=32$"))

	matched, err := VerifyPassword("correct horse battery staple", hash)
	require.NoError(t, err)
	require.True(t, matched)

	matched, err = VerifyPassword("wrong horse battery staple", hash)
	require.NoError(t, err)
	require.False(t, matched)
}

// TestVerifyPasswordRejectsWeakHashParameters verifies stored hashes must keep OWASP-grade cost settings.
func TestVerifyPasswordRejectsWeakHashParameters(t *testing.T) {
	_, err := VerifyPassword("password", "$pbkdf2-sha256$i=9999,l=32$c2FsdA$aGFzaA")

	require.Error(t, err)
	require.Contains(t, err.Error(), "iteration count is too low")
}
