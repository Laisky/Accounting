package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/pbkdf2"
)

// TestHashPasswordUsesArgon2id verifies password hashes use the configured Argon2id format.
func TestHashPasswordUsesArgon2id(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$"))

	matched, err := VerifyPassword("correct horse battery staple", hash)
	require.NoError(t, err)
	require.True(t, matched)

	matched, err = VerifyPassword("wrong horse battery staple", hash)
	require.NoError(t, err)
	require.False(t, matched)
}

// TestVerifyPasswordAcceptsLegacyPBKDF2 verifies legacy hashes remain valid during migration.
func TestVerifyPasswordAcceptsLegacyPBKDF2(t *testing.T) {
	hash := legacyPBKDF2Hash("correct horse battery staple", 600000)

	matched, err := VerifyPassword("correct horse battery staple", hash)
	require.NoError(t, err)
	require.True(t, matched)

	needsRehash, err := NeedsPasswordRehash(hash)
	require.NoError(t, err)
	require.True(t, needsRehash)
}

// TestVerifyPasswordRejectsWeakHashParameters verifies stored hashes must keep safe cost settings.
func TestVerifyPasswordRejectsWeakHashParameters(t *testing.T) {
	_, err := VerifyPassword("password", "$pbkdf2-sha256$i=9999,l=32$c2FsdA$aGFzaA")

	require.Error(t, err)
	require.Contains(t, err.Error(), "iteration count is too low")
}

func legacyPBKDF2Hash(password string, iterations int) string {
	salt := []byte("0123456789abcdef")
	hash := pbkdf2.Key([]byte(password), salt, iterations, passwordKeyBytes, sha256.New)
	return fmt.Sprintf(
		"$pbkdf2-sha256$i=%d,l=32$%s$%s",
		iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}
