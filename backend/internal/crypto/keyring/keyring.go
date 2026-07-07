// Package keyring provides small envelope encryption helpers for durable secrets.
package keyring

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/Laisky/errors/v2"
)

const (
	// CiphertextPrefix identifies version-one keyring ciphertexts.
	CiphertextPrefix = "enc:v1:"

	// MinKeyLength is the fewest characters an operator-supplied secret may
	// contain. A secret is any passphrase, not raw key bytes: it is hashed into
	// an AES-256 key, so operators never encode key material by hand.
	MinKeyLength = 16

	keyLength   = 32
	nonceLength = 12
	wrappedDEK  = keyLength + 16
	keyIDLength = 16

	// Distinct labels keep the public key id independent from the secret key
	// material even though both derive from the same passphrase.
	kekValueLabel = "accounting-keyring:v1:value:"
	kekIDLabel    = "accounting-keyring:v1:ident:"
)

// Ring encrypts with one active key and decrypts with active or retired keys.
type Ring struct {
	active  Key
	retired map[string]Key
}

// Key contains one AES-256-GCM key-encryption key.
type Key struct {
	ID    string
	Value []byte
}

// New receives an active secret and optional retired secrets and returns a Ring.
// Each secret is any operator-chosen passphrase of at least MinKeyLength
// characters; it is hashed into a stable key id and an AES-256 key.
func New(active string, retired []string) (*Ring, error) {
	activeKey, err := deriveKey(active)
	if err != nil {
		return nil, errors.Wrap(err, "derive active key")
	}
	ring := &Ring{
		active:  activeKey,
		retired: map[string]Key{},
	}
	for _, spec := range retired {
		if strings.TrimSpace(spec) == "" {
			continue
		}
		key, err := deriveKey(spec)
		if err != nil {
			return nil, errors.Wrap(err, "derive retired key")
		}
		if key.ID == activeKey.ID {
			return nil, errors.Errorf("retired secret %q duplicates the active secret", key.ID)
		}
		if _, ok := ring.retired[key.ID]; ok {
			return nil, errors.Errorf("retired secret %q is duplicated", key.ID)
		}
		ring.retired[key.ID] = key
	}

	return ring, nil
}

// IsEncrypted reports whether value has the version-one ciphertext prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), CiphertextPrefix)
}

// Encrypt receives plaintext and AAD and returns an envelope ciphertext string.
func (r *Ring) Encrypt(plaintext string, aad string) (string, error) {
	if r == nil {
		return "", errors.WithStack(errors.New("keyring is nil"))
	}
	dek := make([]byte, keyLength)
	if _, err := rand.Read(dek); err != nil {
		return "", errors.Wrap(err, "read data key")
	}
	defer zeroBytes(dek)

	kekAEAD, err := newAEAD(r.active.Value)
	if err != nil {
		return "", err
	}
	dekAEAD, err := newAEAD(dek)
	if err != nil {
		return "", err
	}

	dekNonce := make([]byte, nonceLength)
	payloadNonce := make([]byte, nonceLength)
	if _, err := rand.Read(dekNonce); err != nil {
		return "", errors.Wrap(err, "read data key nonce")
	}
	if _, err := rand.Read(payloadNonce); err != nil {
		return "", errors.Wrap(err, "read payload nonce")
	}
	aadBytes := []byte(aad)
	wrapped := kekAEAD.Seal(nil, dekNonce, dek, aadBytes)
	payload := dekAEAD.Seal(nil, payloadNonce, []byte(plaintext), aadBytes)

	blob := make([]byte, 0, len(dekNonce)+len(wrapped)+len(payloadNonce)+len(payload))
	blob = append(blob, dekNonce...)
	blob = append(blob, wrapped...)
	blob = append(blob, payloadNonce...)
	blob = append(blob, payload...)

	return CiphertextPrefix + r.active.ID + ":" + base64.RawURLEncoding.EncodeToString(blob), nil
}

// Decrypt receives an envelope ciphertext string and AAD and returns plaintext.
func (r *Ring) Decrypt(ciphertext string, aad string) (string, error) {
	if r == nil {
		return "", errors.WithStack(errors.New("keyring is nil"))
	}
	keyID, encoded, err := splitCiphertext(ciphertext)
	if err != nil {
		return "", err
	}
	key, ok := r.keyByID(keyID)
	if !ok {
		return "", errors.Errorf("key %q is not available", keyID)
	}
	blob, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.Wrap(err, "decode ciphertext")
	}
	if len(blob) <= nonceLength+wrappedDEK+nonceLength {
		return "", errors.WithStack(errors.New("ciphertext is too short"))
	}

	dekNonce := blob[:nonceLength]
	wrapped := blob[nonceLength : nonceLength+wrappedDEK]
	payloadNonce := blob[nonceLength+wrappedDEK : nonceLength+wrappedDEK+nonceLength]
	payload := blob[nonceLength+wrappedDEK+nonceLength:]

	kekAEAD, err := newAEAD(key.Value)
	if err != nil {
		return "", err
	}
	dek, err := kekAEAD.Open(nil, dekNonce, wrapped, []byte(aad))
	if err != nil {
		return "", errors.Wrap(err, "unwrap data key")
	}
	defer zeroBytes(dek)
	dekAEAD, err := newAEAD(dek)
	if err != nil {
		return "", err
	}
	plaintext, err := dekAEAD.Open(nil, payloadNonce, payload, []byte(aad))
	if err != nil {
		return "", errors.Wrap(err, "decrypt payload")
	}

	return string(plaintext), nil
}

func (r *Ring) keyByID(keyID string) (Key, bool) {
	if r.active.ID == keyID {
		return r.active, true
	}
	key, ok := r.retired[keyID]
	return key, ok
}

// deriveKey hashes an operator-supplied secret into a stable key id and an
// AES-256 key. The secret is trimmed so trailing newlines in environment
// variables never change the derived key.
func deriveKey(secret string) (Key, error) {
	trimmed := strings.TrimSpace(secret)
	if utf8.RuneCountInString(trimmed) < MinKeyLength {
		return Key{}, errors.Errorf("secret must contain at least %d characters", MinKeyLength)
	}
	value := deriveBytes(kekValueLabel, trimmed)
	id := hex.EncodeToString(deriveBytes(kekIDLabel, trimmed))[:keyIDLength]
	return Key{ID: id, Value: value[:keyLength]}, nil
}

// deriveBytes returns the SHA-256 digest of a domain-separation label prefixed
// to the secret.
func deriveBytes(label string, secret string) []byte {
	sum := sha256.Sum256([]byte(label + secret))
	return sum[:]
}

func splitCiphertext(value string) (string, string, error) {
	rest := strings.TrimPrefix(strings.TrimSpace(value), CiphertextPrefix)
	if rest == strings.TrimSpace(value) {
		return "", "", errors.WithStack(errors.New("ciphertext prefix is missing"))
	}
	keyID, encoded, ok := strings.Cut(rest, ":")
	if !ok || strings.TrimSpace(keyID) == "" || strings.TrimSpace(encoded) == "" {
		return "", "", errors.WithStack(errors.New("ciphertext key id or payload is missing"))
	}
	return strings.TrimSpace(keyID), strings.TrimSpace(encoded), nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "create aes cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "create gcm")
	}
	return aead, nil
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
