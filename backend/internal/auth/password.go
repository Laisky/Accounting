package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/Laisky/errors/v2"
	"golang.org/x/crypto/pbkdf2"
)

const (
	passwordAlgorithm     = "pbkdf2-sha256"
	passwordIterations    = 600000
	minPasswordIterations = 600000
	passwordSaltBytes     = 16
	passwordKeyBytes      = 32
	minPasswordLength     = 8
	maxPasswordLength     = 1024
)

// HashPassword receives a plaintext password and returns a PBKDF2-SHA256 encoded password hash.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.Wrap(err, "read password salt randomness")
	}

	hash := pbkdf2.Key([]byte(password), salt, passwordIterations, passwordKeyBytes, sha256.New)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$%s$i=%d,l=%d$%s$%s",
		passwordAlgorithm,
		passwordIterations,
		passwordKeyBytes,
		encodedSalt,
		encodedHash,
	), nil
}

// VerifyPassword receives a plaintext password and stored hash and reports whether they match.
func VerifyPassword(password string, encodedHash string) (bool, error) {
	params, salt, expectedHash, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, err
	}

	actualHash := pbkdf2.Key([]byte(password), salt, params.iterations, params.keyLength, sha256.New)
	if subtle.ConstantTimeCompare(actualHash, expectedHash) != 1 {
		return false, nil
	}

	return true, nil
}

// ValidatePassword receives a plaintext password and returns an error when it is unsafe to hash.
func ValidatePassword(password string) error {
	passwordLength := len([]rune(password))
	if passwordLength < minPasswordLength {
		return errors.WithStack(errors.New("password must be at least 8 characters"))
	}
	if passwordLength > maxPasswordLength {
		return errors.WithStack(errors.New("password is too long"))
	}

	return nil
}

type passwordParams struct {
	iterations int
	keyLength  int
}

// parsePasswordHash receives an encoded hash and returns the parsed parameters, salt, and hash bytes.
func parsePasswordHash(encodedHash string) (passwordParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 || parts[1] != passwordAlgorithm {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("unsupported password hash format"))
	}

	params, err := parsePasswordParams(parts[2])
	if err != nil {
		return passwordParams{}, nil, nil, err
	}
	if params.iterations < minPasswordIterations {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash iteration count is too low"))
	}
	if params.keyLength < passwordKeyBytes {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash key length is too low"))
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return passwordParams{}, nil, nil, errors.Wrap(err, "decode password salt")
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return passwordParams{}, nil, nil, errors.Wrap(err, "decode password hash")
	}

	return params, salt, hash, nil
}

// parsePasswordParams receives encoded Argon2id parameters and returns structured password parameters.
func parsePasswordParams(encodedParams string) (passwordParams, error) {
	params := passwordParams{}
	for _, part := range strings.Split(encodedParams, ",") {
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			return passwordParams{}, errors.Errorf("invalid password hash parameter %q", part)
		}

		value, err := strconv.Atoi(keyValue[1])
		if err != nil {
			return passwordParams{}, errors.Wrapf(err, "parse password hash parameter %q", keyValue[0])
		}

		switch keyValue[0] {
		case "i":
			params.iterations = value
		case "l":
			params.keyLength = value
		default:
			return passwordParams{}, errors.Errorf("unknown password hash parameter %q", keyValue[0])
		}
	}

	if params.iterations == 0 || params.keyLength == 0 {
		return passwordParams{}, errors.WithStack(errors.New("password hash parameters are incomplete"))
	}

	return params, nil
}
