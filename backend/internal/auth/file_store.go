package auth

import (
	"context"
	"sync"
	"time"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// FileStore persists authentication data to an atomic JSON snapshot file.
type FileStore struct {
	mu     sync.Mutex
	path   string
	memory *MemoryStore
}

// NewFileStore receives a JSON path, loads existing authentication state, and returns a durable store.
func NewFileStore(path string) (*FileStore, error) {
	var snapshot Snapshot
	if err := persistence.LoadJSON(path, &snapshot); err != nil {
		return nil, errors.Wrap(err, "load auth file store")
	}

	return &FileStore{
		path:   path,
		memory: NewMemoryStoreFromSnapshot(snapshot),
	}, nil
}

// CreateUser receives a user record, stores it, and persists the snapshot.
func (s *FileStore) CreateUser(ctx context.Context, user UserRecord) (UserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, err := s.memory.CreateUser(ctx, user)
	if err != nil {
		return UserRecord{}, err
	}
	if err := s.persist(); err != nil {
		return UserRecord{}, err
	}

	return created, nil
}

// UserByEmail receives a normalized email and returns the matching user record.
func (s *FileStore) UserByEmail(ctx context.Context, email string) (UserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.UserByEmail(ctx, email)
}

// UpdateUser receives a user record, replaces it, and persists the snapshot.
func (s *FileStore) UpdateUser(ctx context.Context, user UserRecord) (UserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.memory.UpdateUser(ctx, user)
	if err != nil {
		return UserRecord{}, err
	}
	if err := s.persist(); err != nil {
		return UserRecord{}, err
	}

	return updated, nil
}

// UserByID receives a user id and returns the matching user record.
func (s *FileStore) UserByID(ctx context.Context, userID string) (UserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.UserByID(ctx, userID)
}

// StoreSession receives a hashed session token, stores it, and persists the snapshot.
func (s *FileStore) StoreSession(ctx context.Context, tokenHash string, session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.StoreSession(ctx, tokenHash, session); err != nil {
		return err
	}
	return s.persist()
}

// SessionByTokenHash receives a hashed session token and returns the matching session.
func (s *FileStore) SessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.SessionByTokenHash(ctx, tokenHash)
}

// DeleteSession receives a hashed session token, deletes it, and persists the snapshot.
func (s *FileStore) DeleteSession(ctx context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.DeleteSession(ctx, tokenHash); err != nil {
		return err
	}
	return s.persist()
}

// StoreEmailCode receives a one-time email code record, stores it, and persists the snapshot.
func (s *FileStore) StoreEmailCode(ctx context.Context, code EmailCodeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.StoreEmailCode(ctx, code); err != nil {
		return err
	}
	return s.persist()
}

// EmailCode receives email-code identity and returns the stored code record.
func (s *FileStore) EmailCode(ctx context.Context, email string, purpose EmailCodePurpose) (EmailCodeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.EmailCode(ctx, email, purpose)
}

// DeleteEmailCode receives email-code identity, deletes it, and persists the snapshot.
func (s *FileStore) DeleteEmailCode(ctx context.Context, email string, purpose EmailCodePurpose) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.DeleteEmailCode(ctx, email, purpose); err != nil {
		return err
	}
	return s.persist()
}

// StorePendingTOTPSetup receives a pending setup, stores it, and persists the snapshot.
func (s *FileStore) StorePendingTOTPSetup(ctx context.Context, sessionID string, setup PendingTOTPSetup) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.StorePendingTOTPSetup(ctx, sessionID, setup); err != nil {
		return err
	}
	return s.persist()
}

// PendingTOTPSetup receives a session id and returns the pending setup.
func (s *FileStore) PendingTOTPSetup(ctx context.Context, sessionID string) (PendingTOTPSetup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.PendingTOTPSetup(ctx, sessionID)
}

// DeletePendingTOTPSetup receives a session id, deletes setup state, and persists the snapshot.
func (s *FileStore) DeletePendingTOTPSetup(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.DeletePendingTOTPSetup(ctx, sessionID); err != nil {
		return err
	}
	return s.persist()
}

// StoreTOTPReplay receives a replay marker, stores it, and persists the snapshot.
func (s *FileStore) StoreTOTPReplay(ctx context.Context, userID string, codeHash string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.StoreTOTPReplay(ctx, userID, codeHash, expiresAt); err != nil {
		return err
	}
	return s.persist()
}

// TOTPReplayExists receives replay identity and reports whether the marker is active.
func (s *FileStore) TOTPReplayExists(ctx context.Context, userID string, codeHash string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.TOTPReplayExists(ctx, userID, codeHash, now)
}

// IncrementFailedTOTP increments a user's failed TOTP counter and persists the snapshot.
func (s *FileStore) IncrementFailedTOTP(ctx context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count, err := s.memory.IncrementFailedTOTP(ctx, userID)
	if err != nil {
		return 0, err
	}
	if err := s.persist(); err != nil {
		return 0, err
	}

	return count, nil
}

// ResetFailedTOTP clears a user's failed TOTP counter and persists the snapshot.
func (s *FileStore) ResetFailedTOTP(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.ResetFailedTOTP(ctx, userID); err != nil {
		return err
	}
	return s.persist()
}

// FailedTOTPCount receives a user id and returns the failed TOTP count.
func (s *FileStore) FailedTOTPCount(ctx context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.FailedTOTPCount(ctx, userID)
}

// IncrementFailedLogin increments an email failed-login counter and persists the snapshot.
func (s *FileStore) IncrementFailedLogin(ctx context.Context, email string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count, err := s.memory.IncrementFailedLogin(ctx, email)
	if err != nil {
		return 0, err
	}
	if err := s.persist(); err != nil {
		return 0, err
	}

	return count, nil
}

// ResetFailedLogin clears an email failed-login counter and persists the snapshot.
func (s *FileStore) ResetFailedLogin(ctx context.Context, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.ResetFailedLogin(ctx, email); err != nil {
		return err
	}
	return s.persist()
}

// FailedLoginCount receives an email and returns the failed login count.
func (s *FileStore) FailedLoginCount(ctx context.Context, email string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.FailedLoginCount(ctx, email)
}

// CreatePasskey receives a passkey credential, stores it, and persists the snapshot.
func (s *FileStore) CreatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, err := s.memory.CreatePasskey(ctx, passkey)
	if err != nil {
		return PasskeyCredential{}, err
	}
	if err := s.persist(); err != nil {
		return PasskeyCredential{}, err
	}

	return created, nil
}

// UpdatePasskey receives a passkey credential, updates it, and persists the snapshot.
func (s *FileStore) UpdatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.memory.UpdatePasskey(ctx, passkey)
	if err != nil {
		return PasskeyCredential{}, err
	}
	if err := s.persist(); err != nil {
		return PasskeyCredential{}, err
	}

	return updated, nil
}

// DeletePasskey receives passkey identity, deletes it, and persists the snapshot.
func (s *FileStore) DeletePasskey(ctx context.Context, userID string, passkeyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.DeletePasskey(ctx, userID, passkeyID); err != nil {
		return err
	}
	return s.persist()
}

// PasskeyByID receives passkey identity and returns the owned credential.
func (s *FileStore) PasskeyByID(ctx context.Context, userID string, passkeyID string) (PasskeyCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.PasskeyByID(ctx, userID, passkeyID)
}

// PasskeyByCredentialID receives a raw WebAuthn credential id and returns the matching passkey.
func (s *FileStore) PasskeyByCredentialID(ctx context.Context, credentialID []byte) (PasskeyCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.PasskeyByCredentialID(ctx, credentialID)
}

// ListPasskeys receives a user id and returns all passkeys owned by the user.
func (s *FileStore) ListPasskeys(ctx context.Context, userID string) ([]PasskeyCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.ListPasskeys(ctx, userID)
}

// StorePasskeyCeremony receives WebAuthn challenge state, stores it, and persists the snapshot.
func (s *FileStore) StorePasskeyCeremony(ctx context.Context, ceremony PasskeyCeremony) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.StorePasskeyCeremony(ctx, ceremony); err != nil {
		return err
	}
	return s.persist()
}

// PasskeyCeremony receives a ceremony id and returns stored WebAuthn challenge state.
func (s *FileStore) PasskeyCeremony(ctx context.Context, ceremonyID string) (PasskeyCeremony, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.PasskeyCeremony(ctx, ceremonyID)
}

// DeletePasskeyCeremony receives a ceremony id, deletes it, and persists the snapshot.
func (s *FileStore) DeletePasskeyCeremony(ctx context.Context, ceremonyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.memory.DeletePasskeyCeremony(ctx, ceremonyID); err != nil {
		return err
	}
	return s.persist()
}

// persist writes the current memory snapshot to the configured JSON path.
func (s *FileStore) persist() error {
	if err := persistence.SaveJSONAtomic(s.path, s.memory.Snapshot()); err != nil {
		return errors.Wrap(err, "persist auth file store")
	}

	return nil
}
