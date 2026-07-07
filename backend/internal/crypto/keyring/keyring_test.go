package keyring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRingEncryptDecryptRoundTrip(t *testing.T) {
	ring, err := New(testKey("active"), nil)
	require.NoError(t, err)

	ciphertext, err := ring.Encrypt("JBSWY3DPEHPK3PXP", "auth.totp:user_1")
	require.NoError(t, err)
	require.True(t, IsEncrypted(ciphertext))
	require.NotContains(t, ciphertext, "JBSWY3DPEHPK3PXP")

	plaintext, err := ring.Decrypt(ciphertext, "auth.totp:user_1")
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", plaintext)
}

func TestRingDecryptsRetiredKey(t *testing.T) {
	oldRing, err := New(testKey("old"), nil)
	require.NoError(t, err)
	ciphertext, err := oldRing.Encrypt("JBSWY3DPEHPK3PXP", "auth.totp:user_1")
	require.NoError(t, err)

	newRing, err := New(testKey("new"), []string{testKey("old")})
	require.NoError(t, err)
	plaintext, err := newRing.Decrypt(ciphertext, "auth.totp:user_1")
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", plaintext)
}

func TestRingRejectsAADTamper(t *testing.T) {
	ring, err := New(testKey("active"), nil)
	require.NoError(t, err)
	ciphertext, err := ring.Encrypt("JBSWY3DPEHPK3PXP", "auth.totp:user_1")
	require.NoError(t, err)

	_, err = ring.Decrypt(ciphertext, "auth.totp:user_2")
	require.Error(t, err)
}

func TestNewRejectsShortSecret(t *testing.T) {
	_, err := New("short-secret", nil) // 12 characters
	require.Error(t, err)

	_, err = New("sixteen-char-key", nil) // exactly MinKeyLength characters
	require.NoError(t, err)
}

func TestNewTrimsSecretWhitespace(t *testing.T) {
	padded, err := New("  "+testKey("active")+"  ", nil)
	require.NoError(t, err)
	ciphertext, err := padded.Encrypt("JBSWY3DPEHPK3PXP", "auth.totp:user_1")
	require.NoError(t, err)

	trimmed, err := New(testKey("active"), nil)
	require.NoError(t, err)
	plaintext, err := trimmed.Decrypt(ciphertext, "auth.totp:user_1")
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", plaintext)
}

func testKey(id string) string {
	return "keyring-secret-" + id + "-passphrase"
}
