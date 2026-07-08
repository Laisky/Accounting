package auth

import (
	"encoding/base64"
	"strings"
	"time"
)

// auth_kv namespaces for ephemeral auth records that have no dedicated relational table.
const (
	authEmailCodesNS        = "auth.email_codes"
	authPendingTOTPNS       = "auth.pending_totp"
	authTOTPReplaysNS       = "auth.totp_replays"
	authFailedTOTPsNS       = "auth.failed_totps"
	authFailedLoginsNS      = "auth.failed_logins"
	authPasskeysNS          = "auth.passkeys"           //nolint:gosec // Persistence namespace names are not credentials.
	authPasskeyCeremoniesNS = "auth.passkey_ceremonies" //nolint:gosec // Persistence namespace names are not credentials.
)

// TOTPReplaySnapshot is the auth_kv payload recording a consumed TOTP code until it expires.
type TOTPReplaySnapshot struct {
	UserID    string    `json:"userId"`
	CodeHash  string    `json:"codeHash"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// authKVKey encodes composite key parts into an opaque, collision-free auth_kv key.
// It mirrors the historical persistence.JoinKey encoding so relational auth_kv keys
// stay stable across the storage cutover.
func authKVKey(parts ...string) string {
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, base64.RawURLEncoding.EncodeToString([]byte(part)))
	}
	return strings.Join(encoded, ".")
}

// emailCodeRecordKey derives the auth_kv key for a one-time email code.
func emailCodeRecordKey(email string, purpose EmailCodePurpose) string {
	return authKVKey(email, string(purpose))
}

// replayKey derives the auth_kv key for a consumed TOTP code.
func replayKey(userID string, codeHash string) string {
	return authKVKey(userID, codeHash)
}

// credentialKey encodes a WebAuthn credential id for auth_kv secondary lookups.
func credentialKey(credentialID []byte) string {
	return base64.RawURLEncoding.EncodeToString(credentialID)
}

// stringsCompare returns -1, 0, or 1 ordering two strings, used for deterministic passkey sorting.
func stringsCompare(left string, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// clonePasskeyCredentials returns a deep copy of a passkey credential slice.
func clonePasskeyCredentials(passkeys []PasskeyCredential) []PasskeyCredential {
	cloned := make([]PasskeyCredential, 0, len(passkeys))
	for _, passkey := range passkeys {
		cloned = append(cloned, clonePasskeyCredential(passkey))
	}
	return cloned
}
