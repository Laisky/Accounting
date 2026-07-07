package auth

import "time"

// Snapshot contains the durable authentication store state.
type Snapshot struct {
	Users             []UserRecord                `json:"users"`
	Sessions          map[string]Session          `json:"sessions"`
	EmailCodes        []EmailCodeRecord           `json:"emailCodes"`
	PendingTOTP       map[string]PendingTOTPSetup `json:"pendingTotp"`
	TOTPReplays       []TOTPReplaySnapshot        `json:"totpReplays"`
	FailedTOTPs       map[string]int              `json:"failedTotps"`
	LoginThrottles    map[string]LoginThrottle    `json:"loginThrottles"`
	FailedLogins      map[string]int              `json:"failedLogins,omitempty"`
	Passkeys          []PasskeyCredential         `json:"passkeys"`
	PasskeyCeremonies []PasskeyCeremony           `json:"passkeyCeremonies"`
}

// TOTPReplaySnapshot contains one durable TOTP replay marker.
type TOTPReplaySnapshot struct {
	UserID    string    `json:"userId"`
	CodeHash  string    `json:"codeHash"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// NewMemoryStoreFromSnapshot receives durable state and returns an in-memory authentication store.
func NewMemoryStoreFromSnapshot(snapshot Snapshot) *MemoryStore {
	store := NewMemoryStore()
	for _, user := range snapshot.Users {
		user = cloneUserRecord(user)
		store.usersByID[user.ID] = user
		store.userIDsByEmail[user.Email] = user.ID
	}
	for tokenHash, session := range snapshot.Sessions {
		store.sessions[tokenHash] = session
	}
	for _, code := range snapshot.EmailCodes {
		store.emailCodes[emailCodeKey{email: code.Email, purpose: code.Purpose}] = cloneEmailCodeRecord(code)
	}
	for sessionID, setup := range snapshot.PendingTOTP {
		store.pendingTOTP[sessionID] = setup
	}
	for _, replay := range snapshot.TOTPReplays {
		store.totpReplays[totpReplayKey{userID: replay.UserID, codeHash: replay.CodeHash}] = replay.ExpiresAt
	}
	for userID, count := range snapshot.FailedTOTPs {
		store.failedTOTPs[userID] = count
	}
	for email, throttle := range snapshot.LoginThrottles {
		if throttle.Email == "" {
			throttle.Email = email
		}
		store.loginThrottles[email] = throttle
	}
	for email, count := range snapshot.FailedLogins {
		if _, ok := store.loginThrottles[email]; ok {
			continue
		}
		store.loginThrottles[email] = LoginThrottle{
			Email:       email,
			FailedCount: count,
		}
	}
	for _, passkey := range snapshot.Passkeys {
		passkey = clonePasskeyCredential(passkey)
		store.passkeysByID[passkey.ID] = passkey
		store.passkeyIDsByCredentialID[string(passkey.CredentialID)] = passkey.ID
	}
	for _, ceremony := range snapshot.PasskeyCeremonies {
		store.passkeyCeremonies[ceremony.ID] = ceremony
	}

	return store
}

// Snapshot returns a detached durable representation of the store.
func (s *MemoryStore) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := Snapshot{
		Users:             make([]UserRecord, 0, len(s.usersByID)),
		Sessions:          make(map[string]Session, len(s.sessions)),
		EmailCodes:        make([]EmailCodeRecord, 0, len(s.emailCodes)),
		PendingTOTP:       make(map[string]PendingTOTPSetup, len(s.pendingTOTP)),
		TOTPReplays:       make([]TOTPReplaySnapshot, 0, len(s.totpReplays)),
		FailedTOTPs:       make(map[string]int, len(s.failedTOTPs)),
		LoginThrottles:    make(map[string]LoginThrottle, len(s.loginThrottles)),
		Passkeys:          make([]PasskeyCredential, 0, len(s.passkeysByID)),
		PasskeyCeremonies: make([]PasskeyCeremony, 0, len(s.passkeyCeremonies)),
	}
	for _, user := range s.usersByID {
		snapshot.Users = append(snapshot.Users, cloneUserRecord(user))
	}
	for tokenHash, session := range s.sessions {
		snapshot.Sessions[tokenHash] = session
	}
	for _, code := range s.emailCodes {
		snapshot.EmailCodes = append(snapshot.EmailCodes, cloneEmailCodeRecord(code))
	}
	for sessionID, setup := range s.pendingTOTP {
		snapshot.PendingTOTP[sessionID] = setup
	}
	for replay, expiresAt := range s.totpReplays {
		snapshot.TOTPReplays = append(snapshot.TOTPReplays, TOTPReplaySnapshot{
			UserID:    replay.userID,
			CodeHash:  replay.codeHash,
			ExpiresAt: expiresAt,
		})
	}
	for userID, count := range s.failedTOTPs {
		snapshot.FailedTOTPs[userID] = count
	}
	for email, throttle := range s.loginThrottles {
		snapshot.LoginThrottles[email] = throttle
	}
	for _, passkey := range s.passkeysByID {
		snapshot.Passkeys = append(snapshot.Passkeys, clonePasskeyCredential(passkey))
	}
	for _, ceremony := range s.passkeyCeremonies {
		snapshot.PasskeyCeremonies = append(snapshot.PasskeyCeremonies, ceremony)
	}

	return snapshot
}
