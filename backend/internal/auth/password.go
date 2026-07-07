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
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

const (
	passwordAlgorithmArgon2id = "argon2id"
	passwordAlgorithmPBKDF2   = "pbkdf2-sha256" //nolint:gosec // Algorithm identifiers are not credentials.
	passwordArgon2Version     = argon2.Version
	passwordArgon2MemoryKiB   = 19456
	passwordArgon2Iterations  = 2
	passwordArgon2Parallelism = 1
	passwordIterations        = 600000
	minPasswordIterations     = 600000
	passwordSaltBytes         = 16
	passwordKeyBytes          = 32
	minPasswordLength         = 8
	maxPasswordLength         = 1024
	maxPasswordUint8          = 255
	maxPasswordUint32         = 1<<32 - 1
)

// HashPassword receives a plaintext password and returns an Argon2id encoded password hash.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.Wrap(err, "read password salt randomness")
	}

	hash := argon2.IDKey([]byte(password), salt, passwordArgon2Iterations, passwordArgon2MemoryKiB, passwordArgon2Parallelism, passwordKeyBytes)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		passwordAlgorithmArgon2id,
		passwordArgon2Version,
		passwordArgon2MemoryKiB,
		passwordArgon2Iterations,
		passwordArgon2Parallelism,
		encodedSalt,
		encodedHash,
	), nil
}

// VerifyPassword receives a plaintext password and stored hash and reports whether they match.
func VerifyPassword(password string, encodedHash string) (bool, error) {
	algorithm, err := passwordHashAlgorithm(encodedHash)
	if err != nil {
		return false, err
	}

	switch algorithm {
	case passwordAlgorithmArgon2id:
		params, salt, expectedHash, err := parseArgon2idHash(encodedHash)
		if err != nil {
			return false, err
		}
		actualHash := argon2.IDKey(
			[]byte(password),
			salt,
			uint32(params.iterations), //nolint:gosec // parseArgon2idHash bounds parameters before conversion.
			uint32(params.memoryKiB),  //nolint:gosec // parseArgon2idHash bounds parameters before conversion.
			uint8(params.parallelism), //nolint:gosec // parseArgon2idHash bounds parameters before conversion.
			uint32(params.keyLength),  //nolint:gosec // parseArgon2idHash bounds parameters before conversion.
		)
		return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
	case passwordAlgorithmPBKDF2:
		params, salt, expectedHash, err := parsePBKDF2Hash(encodedHash)
		if err != nil {
			return false, err
		}
		actualHash := pbkdf2.Key([]byte(password), salt, params.iterations, params.keyLength, sha256.New)
		return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
	default:
		return false, errors.WithStack(errors.New("unsupported password hash format"))
	}
}

// NeedsPasswordRehash receives a stored password hash and reports whether it should be upgraded.
func NeedsPasswordRehash(encodedHash string) (bool, error) {
	algorithm, err := passwordHashAlgorithm(encodedHash)
	if err != nil {
		return false, err
	}
	if algorithm != passwordAlgorithmArgon2id {
		return true, nil
	}
	params, _, _, err := parseArgon2idHash(encodedHash)
	if err != nil {
		return false, err
	}

	return params.version != passwordArgon2Version ||
		params.memoryKiB < passwordArgon2MemoryKiB ||
		params.iterations < passwordArgon2Iterations ||
		params.parallelism < passwordArgon2Parallelism ||
		params.keyLength < passwordKeyBytes, nil
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
	version     int
	memoryKiB   int
	iterations  int
	parallelism int
	keyLength   int
}

// passwordHashAlgorithm receives an encoded password hash and returns its algorithm marker.
func passwordHashAlgorithm(encodedHash string) (string, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) < 2 || parts[0] != "" || strings.TrimSpace(parts[1]) == "" {
		return "", errors.WithStack(errors.New("unsupported password hash format"))
	}

	return parts[1], nil
}

// parseArgon2idHash receives an encoded Argon2id hash and returns parsed parameters, salt, and hash bytes.
func parseArgon2idHash(encodedHash string) (passwordParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != passwordAlgorithmArgon2id {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("unsupported password hash format"))
	}
	params, err := parseArgon2idParams(parts[2], parts[3])
	if err != nil {
		return passwordParams{}, nil, nil, err
	}
	if params.version != passwordArgon2Version {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash argon2 version is unsupported"))
	}
	if params.memoryKiB < passwordArgon2MemoryKiB {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash memory cost is too low"))
	}
	if params.iterations < passwordArgon2Iterations {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash iteration count is too low"))
	}
	if params.parallelism < passwordArgon2Parallelism {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash parallelism is too low"))
	}
	if params.memoryKiB > maxPasswordUint32 ||
		params.iterations > maxPasswordUint32 ||
		params.parallelism > maxPasswordUint8 {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash parameters are too large"))
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return passwordParams{}, nil, nil, errors.Wrap(err, "decode password salt")
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return passwordParams{}, nil, nil, errors.Wrap(err, "decode password hash")
	}
	params.keyLength = len(hash)
	if params.keyLength < passwordKeyBytes {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash key length is too low"))
	}
	if params.keyLength > maxPasswordUint32 {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("password hash key length is too large"))
	}

	return params, salt, hash, nil
}

// parsePBKDF2Hash receives an encoded legacy PBKDF2 hash and returns parsed parameters, salt, and hash bytes.
func parsePBKDF2Hash(encodedHash string) (passwordParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 || parts[1] != passwordAlgorithmPBKDF2 {
		return passwordParams{}, nil, nil, errors.WithStack(errors.New("unsupported password hash format"))
	}

	params, err := parsePBKDF2Params(parts[2])
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

// parseArgon2idParams receives encoded Argon2id parameters and returns structured password parameters.
func parseArgon2idParams(versionParam string, encodedParams string) (passwordParams, error) {
	params := passwordParams{}
	versionParts := strings.SplitN(versionParam, "=", 2)
	if len(versionParts) != 2 || versionParts[0] != "v" {
		return passwordParams{}, errors.Errorf("invalid argon2 version parameter %q", versionParam)
	}
	version, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return passwordParams{}, errors.Wrap(err, "parse argon2 version")
	}
	params.version = version

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
		case "m":
			params.memoryKiB = value
		case "t":
			params.iterations = value
		case "p":
			params.parallelism = value
		default:
			return passwordParams{}, errors.Errorf("unknown password hash parameter %q", keyValue[0])
		}
	}

	if params.version == 0 || params.memoryKiB == 0 || params.iterations == 0 || params.parallelism == 0 {
		return passwordParams{}, errors.WithStack(errors.New("password hash parameters are incomplete"))
	}

	return params, nil
}

// parsePBKDF2Params receives encoded PBKDF2 parameters and returns structured password parameters.
func parsePBKDF2Params(encodedParams string) (passwordParams, error) {
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
