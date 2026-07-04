package auth

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/go-webauthn/webauthn/protocol"
)

// Store defines authentication persistence operations required by the service layer.
type Store interface {
	CreateUser(ctx context.Context, user UserRecord) (UserRecord, error)
	UserByEmail(ctx context.Context, email string) (UserRecord, error)
	UserByID(ctx context.Context, userID string) (UserRecord, error)
	UpdateUser(ctx context.Context, user UserRecord) (UserRecord, error)
	StoreSession(ctx context.Context, tokenHash string, session Session) error
	SessionByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	StoreEmailCode(ctx context.Context, code EmailCodeRecord) error
	EmailCode(ctx context.Context, email string, purpose EmailCodePurpose) (EmailCodeRecord, error)
	DeleteEmailCode(ctx context.Context, email string, purpose EmailCodePurpose) error
	StorePendingTOTPSetup(ctx context.Context, sessionID string, setup PendingTOTPSetup) error
	PendingTOTPSetup(ctx context.Context, sessionID string) (PendingTOTPSetup, error)
	DeletePendingTOTPSetup(ctx context.Context, sessionID string) error
	StoreTOTPReplay(ctx context.Context, userID string, codeHash string, expiresAt time.Time) error
	TOTPReplayExists(ctx context.Context, userID string, codeHash string, now time.Time) (bool, error)
	IncrementFailedTOTP(ctx context.Context, userID string) (int, error)
	ResetFailedTOTP(ctx context.Context, userID string) error
	FailedTOTPCount(ctx context.Context, userID string) (int, error)
	IncrementFailedLogin(ctx context.Context, email string) (int, error)
	ResetFailedLogin(ctx context.Context, email string) error
	FailedLoginCount(ctx context.Context, email string) (int, error)
	CreatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error)
	UpdatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error)
	DeletePasskey(ctx context.Context, userID string, passkeyID string) error
	PasskeyByID(ctx context.Context, userID string, passkeyID string) (PasskeyCredential, error)
	PasskeyByCredentialID(ctx context.Context, credentialID []byte) (PasskeyCredential, error)
	ListPasskeys(ctx context.Context, userID string) ([]PasskeyCredential, error)
	StorePasskeyCeremony(ctx context.Context, ceremony PasskeyCeremony) error
	PasskeyCeremony(ctx context.Context, ceremonyID string) (PasskeyCeremony, error)
	DeletePasskeyCeremony(ctx context.Context, ceremonyID string) error
}

// MemoryStore keeps users, sessions, and failure counters in process for local development.
type MemoryStore struct {
	mu                       sync.RWMutex
	usersByID                map[string]UserRecord
	userIDsByEmail           map[string]string
	sessions                 map[string]Session
	emailCodes               map[emailCodeKey]EmailCodeRecord
	pendingTOTP              map[string]PendingTOTPSetup
	totpReplays              map[totpReplayKey]time.Time
	failedTOTPs              map[string]int
	failedLogins             map[string]int
	passkeysByID             map[string]PasskeyCredential
	passkeyIDsByCredentialID map[string]string
	passkeyCeremonies        map[string]PasskeyCeremony
}

type emailCodeKey struct {
	email   string
	purpose EmailCodePurpose
}

type totpReplayKey struct {
	userID   string
	codeHash string
}

// NewMemoryStore returns an empty in-memory authentication Store implementation.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usersByID:                map[string]UserRecord{},
		userIDsByEmail:           map[string]string{},
		sessions:                 map[string]Session{},
		emailCodes:               map[emailCodeKey]EmailCodeRecord{},
		pendingTOTP:              map[string]PendingTOTPSetup{},
		totpReplays:              map[totpReplayKey]time.Time{},
		failedTOTPs:              map[string]int{},
		failedLogins:             map[string]int{},
		passkeysByID:             map[string]PasskeyCredential{},
		passkeyIDsByCredentialID: map[string]string{},
		passkeyCeremonies:        map[string]PasskeyCeremony{},
	}
}

// CreateUser receives a user record and stores it when its email and id are unique.
func (s *MemoryStore) CreateUser(_ context.Context, user UserRecord) (UserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.usersByID[user.ID]; ok {
		return UserRecord{}, errors.WithStack(errors.New("user id already exists"))
	}
	if _, ok := s.userIDsByEmail[user.Email]; ok {
		return UserRecord{}, errors.WithStack(errors.New("user email already exists"))
	}

	s.usersByID[user.ID] = cloneUserRecord(user)
	s.userIDsByEmail[user.Email] = user.ID

	return cloneUserRecord(user), nil
}

// UserByEmail receives a normalized email and returns the matching user record.
func (s *MemoryStore) UserByEmail(_ context.Context, email string) (UserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.userIDsByEmail[email]
	if !ok {
		return UserRecord{}, errors.WithStack(errors.New("user not found"))
	}

	return cloneUserRecord(s.usersByID[userID]), nil
}

// UpdateUser receives a user record and replaces the stored record for the same id and email.
func (s *MemoryStore) UpdateUser(_ context.Context, user UserRecord) (UserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.usersByID[user.ID]
	if !ok {
		return UserRecord{}, errors.WithStack(errors.New("user not found"))
	}
	if existing.Email != user.Email {
		return UserRecord{}, errors.WithStack(errors.New("user email cannot change"))
	}

	s.usersByID[user.ID] = cloneUserRecord(user)
	return cloneUserRecord(user), nil
}

// UserByID receives a user id and returns the matching user record.
func (s *MemoryStore) UserByID(_ context.Context, userID string) (UserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.usersByID[userID]
	if !ok {
		return UserRecord{}, errors.WithStack(errors.New("user not found"))
	}

	return cloneUserRecord(user), nil
}

// StoreSession receives a hashed session token and stores the associated session metadata.
func (s *MemoryStore) StoreSession(_ context.Context, tokenHash string, session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[tokenHash] = session
	return nil
}

// SessionByTokenHash receives a hashed session token and returns the matching active session.
func (s *MemoryStore) SessionByTokenHash(_ context.Context, tokenHash string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[tokenHash]
	if !ok {
		return Session{}, errors.WithStack(errors.New("session not found"))
	}
	return session, nil
}

// DeleteSession receives a hashed session token and removes the associated session when present.
func (s *MemoryStore) DeleteSession(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, tokenHash)
	return nil
}

// StoreEmailCode receives a one-time email code record and stores it by email and purpose.
func (s *MemoryStore) StoreEmailCode(_ context.Context, code EmailCodeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.emailCodes[emailCodeKey{email: code.Email, purpose: code.Purpose}] = cloneEmailCodeRecord(code)
	return nil
}

// EmailCode receives email and purpose and returns the matching one-time code record.
func (s *MemoryStore) EmailCode(_ context.Context, email string, purpose EmailCodePurpose) (EmailCodeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	code, ok := s.emailCodes[emailCodeKey{email: email, purpose: purpose}]
	if !ok {
		return EmailCodeRecord{}, errors.WithStack(errors.New("email code not found"))
	}

	return cloneEmailCodeRecord(code), nil
}

// DeleteEmailCode receives email and purpose and removes the matching one-time code record.
func (s *MemoryStore) DeleteEmailCode(_ context.Context, email string, purpose EmailCodePurpose) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.emailCodes, emailCodeKey{email: email, purpose: purpose})
	return nil
}

// StorePendingTOTPSetup receives a session id and stores its pending unconfirmed TOTP setup.
func (s *MemoryStore) StorePendingTOTPSetup(_ context.Context, sessionID string, setup PendingTOTPSetup) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pendingTOTP[sessionID] = setup
	return nil
}

// PendingTOTPSetup receives a session id and returns its pending unconfirmed TOTP setup.
func (s *MemoryStore) PendingTOTPSetup(_ context.Context, sessionID string) (PendingTOTPSetup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	setup, ok := s.pendingTOTP[sessionID]
	if !ok {
		return PendingTOTPSetup{}, errors.WithStack(errors.New("pending totp setup not found"))
	}

	return setup, nil
}

// DeletePendingTOTPSetup receives a session id and removes its pending TOTP setup.
func (s *MemoryStore) DeletePendingTOTPSetup(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingTOTP, sessionID)
	return nil
}

// StoreTOTPReplay receives a user id, code hash, and expiry and stores a replay marker.
func (s *MemoryStore) StoreTOTPReplay(_ context.Context, userID string, codeHash string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.totpReplays[totpReplayKey{userID: userID, codeHash: codeHash}] = expiresAt
	return nil
}

// TOTPReplayExists receives a user id, code hash, and current time and reports whether the code was reused.
func (s *MemoryStore) TOTPReplayExists(_ context.Context, userID string, codeHash string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := totpReplayKey{userID: userID, codeHash: codeHash}
	expiresAt, ok := s.totpReplays[key]
	if !ok {
		return false, nil
	}
	if !expiresAt.After(now) {
		delete(s.totpReplays, key)
		return false, nil
	}

	return true, nil
}

// IncrementFailedTOTP receives a user id and increments its failed TOTP counter.
func (s *MemoryStore) IncrementFailedTOTP(_ context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failedTOTPs[userID]++
	return s.failedTOTPs[userID], nil
}

// ResetFailedTOTP receives a user id and clears its failed TOTP counter.
func (s *MemoryStore) ResetFailedTOTP(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.failedTOTPs, userID)
	return nil
}

// FailedTOTPCount receives a user id and returns its failed TOTP counter.
func (s *MemoryStore) FailedTOTPCount(_ context.Context, userID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.failedTOTPs[userID], nil
}

// IncrementFailedLogin receives a normalized email and increments its failed login counter.
func (s *MemoryStore) IncrementFailedLogin(_ context.Context, email string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failedLogins[email]++
	return s.failedLogins[email], nil
}

// ResetFailedLogin receives a normalized email and clears its failed login counter.
func (s *MemoryStore) ResetFailedLogin(_ context.Context, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.failedLogins, email)
	return nil
}

// FailedLoginCount receives a normalized email and returns its current failed login counter.
func (s *MemoryStore) FailedLoginCount(_ context.Context, email string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.failedLogins[email], nil
}

// CreatePasskey receives a passkey credential and stores it when its ids are unique.
func (s *MemoryStore) CreatePasskey(_ context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.passkeysByID[passkey.ID]; ok {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey id already exists"))
	}
	credentialKey := string(passkey.CredentialID)
	if _, ok := s.passkeyIDsByCredentialID[credentialKey]; ok {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey credential already exists"))
	}

	s.passkeysByID[passkey.ID] = clonePasskeyCredential(passkey)
	s.passkeyIDsByCredentialID[credentialKey] = passkey.ID

	return clonePasskeyCredential(passkey), nil
}

// UpdatePasskey receives a passkey credential and replaces the stored credential for the same id.
func (s *MemoryStore) UpdatePasskey(_ context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.passkeysByID[passkey.ID]
	if !ok {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey not found"))
	}
	if existing.UserID != passkey.UserID {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey owner cannot change"))
	}
	if string(existing.CredentialID) != string(passkey.CredentialID) {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey credential id cannot change"))
	}

	s.passkeysByID[passkey.ID] = clonePasskeyCredential(passkey)
	return clonePasskeyCredential(passkey), nil
}

// DeletePasskey receives a user id and passkey id and removes the matching credential.
func (s *MemoryStore) DeletePasskey(_ context.Context, userID string, passkeyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	passkey, ok := s.passkeysByID[passkeyID]
	if !ok || passkey.UserID != userID {
		return errors.WithStack(errors.New("passkey not found"))
	}

	delete(s.passkeysByID, passkeyID)
	delete(s.passkeyIDsByCredentialID, string(passkey.CredentialID))
	return nil
}

// PasskeyByID receives a user id and passkey id and returns the owned credential.
func (s *MemoryStore) PasskeyByID(_ context.Context, userID string, passkeyID string) (PasskeyCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	passkey, ok := s.passkeysByID[passkeyID]
	if !ok || passkey.UserID != userID {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey not found"))
	}

	return clonePasskeyCredential(passkey), nil
}

// PasskeyByCredentialID receives a raw WebAuthn credential id and returns the matching passkey.
func (s *MemoryStore) PasskeyByCredentialID(_ context.Context, credentialID []byte) (PasskeyCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	passkeyID, ok := s.passkeyIDsByCredentialID[string(credentialID)]
	if !ok {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey not found"))
	}

	return clonePasskeyCredential(s.passkeysByID[passkeyID]), nil
}

// ListPasskeys receives a user id and returns all passkeys owned by the user.
func (s *MemoryStore) ListPasskeys(_ context.Context, userID string) ([]PasskeyCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	passkeys := make([]PasskeyCredential, 0)
	for _, passkey := range s.passkeysByID {
		if passkey.UserID == userID {
			passkeys = append(passkeys, clonePasskeyCredential(passkey))
		}
	}

	return passkeys, nil
}

// StorePasskeyCeremony receives WebAuthn challenge state and stores it by ceremony id.
func (s *MemoryStore) StorePasskeyCeremony(_ context.Context, ceremony PasskeyCeremony) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.passkeyCeremonies[ceremony.ID] = ceremony
	return nil
}

// PasskeyCeremony receives a ceremony id and returns the stored WebAuthn challenge state.
func (s *MemoryStore) PasskeyCeremony(_ context.Context, ceremonyID string) (PasskeyCeremony, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ceremony, ok := s.passkeyCeremonies[ceremonyID]
	if !ok {
		return PasskeyCeremony{}, errors.WithStack(errors.New("passkey ceremony not found"))
	}

	return ceremony, nil
}

// DeletePasskeyCeremony receives a ceremony id and removes the matching WebAuthn challenge state.
func (s *MemoryStore) DeletePasskeyCeremony(_ context.Context, ceremonyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.passkeyCeremonies, ceremonyID)
	return nil
}

// cloneUserRecord receives a user record and returns a detached copy.
func cloneUserRecord(user UserRecord) UserRecord {
	if strings.TrimSpace(user.BaseCurrency) == "" {
		user.BaseCurrency = DefaultBaseCurrency
	}
	return user
}

// cloneEmailCodeRecord receives an email code record and returns a detached copy.
func cloneEmailCodeRecord(code EmailCodeRecord) EmailCodeRecord {
	return code
}

// clonePasskeyCredential receives a passkey credential and returns a detached copy.
func clonePasskeyCredential(passkey PasskeyCredential) PasskeyCredential {
	passkey.CredentialID = append([]byte(nil), passkey.CredentialID...)
	passkey.PublicKey = append([]byte(nil), passkey.PublicKey...)
	passkey.Transports = append([]string(nil), passkey.Transports...)
	passkey.AAGUID = append([]byte(nil), passkey.AAGUID...)
	passkey.Credential.ID = append([]byte(nil), passkey.Credential.ID...)
	passkey.Credential.PublicKey = append([]byte(nil), passkey.Credential.PublicKey...)
	passkey.Credential.Transport = append([]protocol.AuthenticatorTransport(nil), passkey.Credential.Transport...)
	passkey.Credential.Authenticator.AAGUID = append([]byte(nil), passkey.Credential.Authenticator.AAGUID...)
	if passkey.LastUsedAt != nil {
		lastUsedAt := passkey.LastUsedAt.UTC()
		passkey.LastUsedAt = &lastUsedAt
	}

	return passkey
}
