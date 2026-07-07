package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

const (
	authUsersNS             = "auth.users"
	authSessionsNS          = "auth.sessions"
	authEmailCodesNS        = "auth.email_codes"
	authPendingTOTPNS       = "auth.pending_totp"
	authTOTPReplaysNS       = "auth.totp_replays"
	authFailedTOTPsNS       = "auth.failed_totps"
	authFailedLoginsNS      = "auth.failed_logins"
	authPasskeysNS          = "auth.passkeys"           //nolint:gosec // Persistence namespace names are not credentials.
	authPasskeyCeremoniesNS = "auth.passkey_ceremonies" //nolint:gosec // Persistence namespace names are not credentials.
)

// SQLStore persists authentication records directly in SQL rows.
type SQLStore struct {
	records *persistence.RecordStore
}

// NewSQLStore receives a record store and returns a direct SQL authentication Store implementation.
func NewSQLStore(records *persistence.RecordStore) *SQLStore {
	return &SQLStore{records: records}
}

// NewPostgresStore receives a database handle and returns a direct SQL authentication store.
func NewPostgresStore(db *sql.DB) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectPostgres)), nil
}

// NewSQLiteStore receives a database handle and returns a direct SQL authentication store.
func NewSQLiteStore(db *sql.DB) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite)), nil
}

// CreateUser receives a user record and stores it when its email and id are unique.
func (s *SQLStore) CreateUser(ctx context.Context, user UserRecord) (UserRecord, error) {
	user = cloneUserRecord(user)
	if err := s.records.Insert(ctx, record(authUsersNS, user.ID, "", user.Email, user)); err != nil {
		return UserRecord{}, errors.Wrap(err, "insert user")
	}
	return cloneUserRecord(user), nil
}

// UserByEmail receives a normalized email and returns the matching user record.
func (s *SQLStore) UserByEmail(ctx context.Context, email string) (UserRecord, error) {
	var user UserRecord
	if err := s.records.GetBySecondary(ctx, authUsersNS, email, &user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserRecord{}, errors.WithStack(errors.New("user not found"))
		}
		return UserRecord{}, errors.Wrap(err, "load user by email")
	}
	return cloneUserRecord(user), nil
}

// UserByID receives a user id and returns the matching user record.
func (s *SQLStore) UserByID(ctx context.Context, userID string) (UserRecord, error) {
	var user UserRecord
	if err := s.records.Get(ctx, authUsersNS, userID, &user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserRecord{}, errors.WithStack(errors.New("user not found"))
		}
		return UserRecord{}, errors.Wrap(err, "load user by id")
	}
	return cloneUserRecord(user), nil
}

// UpdateUser receives a user record and replaces the stored record for the same id and email.
func (s *SQLStore) UpdateUser(ctx context.Context, user UserRecord) (UserRecord, error) {
	existing, err := s.UserByID(ctx, user.ID)
	if err != nil {
		return UserRecord{}, err
	}
	if existing.Email != user.Email {
		return UserRecord{}, errors.WithStack(errors.New("user email cannot change"))
	}
	user = cloneUserRecord(user)
	if err := s.records.Update(ctx, record(authUsersNS, user.ID, "", user.Email, user)); err != nil {
		return UserRecord{}, errors.Wrap(err, "update user")
	}
	return cloneUserRecord(user), nil
}

// StoreSession receives a hashed session token and stores the associated session metadata.
func (s *SQLStore) StoreSession(ctx context.Context, tokenHash string, session Session) error {
	return s.records.Upsert(ctx, record(authSessionsNS, tokenHash, session.UserID, "", session))
}

// SessionByTokenHash receives a hashed session token and returns the matching active session.
func (s *SQLStore) SessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	var session Session
	if err := s.records.Get(ctx, authSessionsNS, tokenHash, &session); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, errors.WithStack(errors.New("session not found"))
		}
		return Session{}, errors.Wrap(err, "load session")
	}
	return session, nil
}

// DeleteSession receives a hashed session token and removes the associated session when present.
func (s *SQLStore) DeleteSession(ctx context.Context, tokenHash string) error {
	return s.records.Delete(ctx, authSessionsNS, tokenHash)
}

// DeleteSessionsByUser receives a user id and removes all sessions owned by that user.
func (s *SQLStore) DeleteSessionsByUser(ctx context.Context, userID string) error {
	return s.records.DeleteByOwner(ctx, authSessionsNS, userID)
}

// MigrateTOTPSecrets receives a secret transform and rewrites stored TOTP secrets in place.
func (s *SQLStore) MigrateTOTPSecrets(ctx context.Context, encrypt func(userID string, secret string) (string, error)) error {
	var users []UserRecord
	if err := s.records.List(ctx, authUsersNS, nil, nil, &users); err != nil {
		return errors.Wrap(err, "list users for totp migration")
	}
	for _, user := range users {
		if strings.TrimSpace(user.TOTPSecret) == "" {
			continue
		}
		secret, err := encrypt(user.ID, user.TOTPSecret)
		if err != nil {
			return err
		}
		if secret == user.TOTPSecret {
			continue
		}
		user.TOTPSecret = secret
		if _, err := s.UpdateUser(ctx, user); err != nil {
			return errors.Wrap(err, "update migrated totp user")
		}
	}

	records, err := s.records.ListRecords(ctx, authPendingTOTPNS)
	if err != nil {
		return errors.Wrap(err, "list pending totp records for migration")
	}
	for _, record := range records {
		var setup PendingTOTPSetup
		if err := json.Unmarshal(record.Data, &setup); err != nil {
			return errors.Wrap(err, "decode pending totp setup")
		}
		if strings.TrimSpace(setup.Secret) == "" {
			continue
		}
		secret, err := encrypt(setup.UserID, setup.Secret)
		if err != nil {
			return err
		}
		if secret == setup.Secret {
			continue
		}
		setup.Secret = secret
		record.Data = mustJSON(setup)
		if err := s.records.Upsert(ctx, record); err != nil {
			return errors.Wrap(err, "update migrated pending totp setup")
		}
	}

	return nil
}

// StoreEmailCode receives a one-time email code record and stores it by email and purpose.
func (s *SQLStore) StoreEmailCode(ctx context.Context, code EmailCodeRecord) error {
	code = cloneEmailCodeRecord(code)
	return s.records.Upsert(ctx, record(authEmailCodesNS, emailCodeRecordKey(code.Email, code.Purpose), "", "", code))
}

// EmailCode receives email and purpose and returns the matching one-time code record.
func (s *SQLStore) EmailCode(ctx context.Context, email string, purpose EmailCodePurpose) (EmailCodeRecord, error) {
	var code EmailCodeRecord
	if err := s.records.Get(ctx, authEmailCodesNS, emailCodeRecordKey(email, purpose), &code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailCodeRecord{}, errors.WithStack(errors.New("email code not found"))
		}
		return EmailCodeRecord{}, errors.Wrap(err, "load email code")
	}
	return cloneEmailCodeRecord(code), nil
}

// DeleteEmailCode receives email and purpose and removes the matching one-time code record.
func (s *SQLStore) DeleteEmailCode(ctx context.Context, email string, purpose EmailCodePurpose) error {
	return s.records.Delete(ctx, authEmailCodesNS, emailCodeRecordKey(email, purpose))
}

// StorePendingTOTPSetup receives a session id and stores its pending unconfirmed TOTP setup.
func (s *SQLStore) StorePendingTOTPSetup(ctx context.Context, sessionID string, setup PendingTOTPSetup) error {
	return s.records.Upsert(ctx, record(authPendingTOTPNS, sessionID, setup.UserID, "", setup))
}

// PendingTOTPSetup receives a session id and returns its pending unconfirmed TOTP setup.
func (s *SQLStore) PendingTOTPSetup(ctx context.Context, sessionID string) (PendingTOTPSetup, error) {
	var setup PendingTOTPSetup
	if err := s.records.Get(ctx, authPendingTOTPNS, sessionID, &setup); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PendingTOTPSetup{}, errors.WithStack(errors.New("pending totp setup not found"))
		}
		return PendingTOTPSetup{}, errors.Wrap(err, "load pending totp setup")
	}
	return setup, nil
}

// DeletePendingTOTPSetup receives a session id and removes its pending TOTP setup.
func (s *SQLStore) DeletePendingTOTPSetup(ctx context.Context, sessionID string) error {
	return s.records.Delete(ctx, authPendingTOTPNS, sessionID)
}

// StoreTOTPReplay receives a user id, code hash, and expiry and stores a replay marker.
func (s *SQLStore) StoreTOTPReplay(ctx context.Context, userID string, codeHash string, expiresAt time.Time) error {
	replay := TOTPReplaySnapshot{UserID: userID, CodeHash: codeHash, ExpiresAt: expiresAt.UTC()}
	return s.records.Upsert(ctx, record(authTOTPReplaysNS, replayKey(userID, codeHash), userID, "", replay))
}

// TOTPReplayExists receives a user id, code hash, and current time and reports whether the code was reused.
func (s *SQLStore) TOTPReplayExists(ctx context.Context, userID string, codeHash string, now time.Time) (bool, error) {
	var replay TOTPReplaySnapshot
	key := replayKey(userID, codeHash)
	if err := s.records.Get(ctx, authTOTPReplaysNS, key, &replay); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, errors.Wrap(err, "load totp replay")
	}
	if !replay.ExpiresAt.After(now.UTC()) {
		if err := s.records.Delete(ctx, authTOTPReplaysNS, key); err != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// IncrementFailedTOTP receives a user id and increments its failed TOTP counter.
func (s *SQLStore) IncrementFailedTOTP(ctx context.Context, userID string) (int, error) {
	return s.records.IncrementCounter(ctx, authFailedTOTPsNS, userID)
}

// ResetFailedTOTP receives a user id and clears its failed TOTP counter.
func (s *SQLStore) ResetFailedTOTP(ctx context.Context, userID string) error {
	return s.records.Delete(ctx, authFailedTOTPsNS, userID)
}

// FailedTOTPCount receives a user id and returns its failed TOTP counter.
func (s *SQLStore) FailedTOTPCount(ctx context.Context, userID string) (int, error) {
	return s.records.Counter(ctx, authFailedTOTPsNS, userID)
}

// LoginThrottle receives a normalized email and returns its password-login throttle state.
func (s *SQLStore) LoginThrottle(ctx context.Context, email string) (LoginThrottle, error) {
	var throttle LoginThrottle
	if err := s.records.Get(ctx, authFailedLoginsNS, email, &throttle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginThrottle{}, nil
		}
		var legacyCount int
		if legacyErr := s.records.Get(ctx, authFailedLoginsNS, email, &legacyCount); legacyErr == nil {
			return LoginThrottle{Email: email, FailedCount: legacyCount}, nil
		}
		return LoginThrottle{}, errors.Wrap(err, "load login throttle")
	}
	return throttle, nil
}

// StoreLoginThrottle receives password-login throttle state and stores it by normalized email.
func (s *SQLStore) StoreLoginThrottle(ctx context.Context, throttle LoginThrottle) error {
	if strings.TrimSpace(throttle.Email) == "" {
		return errors.WithStack(errors.New("login throttle email is required"))
	}
	throttle.LockedUntil = throttle.LockedUntil.UTC()
	throttle.UpdatedAt = throttle.UpdatedAt.UTC()
	return s.records.Upsert(ctx, record(authFailedLoginsNS, throttle.Email, "", "", throttle))
}

// ResetLoginThrottle receives a normalized email and clears its password-login throttle state.
func (s *SQLStore) ResetLoginThrottle(ctx context.Context, email string) error {
	return s.records.Delete(ctx, authFailedLoginsNS, email)
}

// CreatePasskey receives a passkey credential and stores it when its ids are unique.
func (s *SQLStore) CreatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
	passkey = clonePasskeyCredential(passkey)
	if err := s.records.Insert(ctx, passkeyRecord(passkey)); err != nil {
		return PasskeyCredential{}, errors.Wrap(err, "insert passkey")
	}
	return clonePasskeyCredential(passkey), nil
}

// UpdatePasskey receives a passkey credential and replaces the stored credential for the same id.
func (s *SQLStore) UpdatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
	existing, err := s.PasskeyByID(ctx, passkey.UserID, passkey.ID)
	if err != nil {
		return PasskeyCredential{}, err
	}
	if existing.UserID != passkey.UserID {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey owner cannot change"))
	}
	if string(existing.CredentialID) != string(passkey.CredentialID) {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey credential id cannot change"))
	}
	passkey = clonePasskeyCredential(passkey)
	if err := s.records.Update(ctx, passkeyRecord(passkey)); err != nil {
		return PasskeyCredential{}, errors.Wrap(err, "update passkey")
	}
	return clonePasskeyCredential(passkey), nil
}

// DeletePasskey receives a user id and passkey id and removes the matching credential.
func (s *SQLStore) DeletePasskey(ctx context.Context, userID string, passkeyID string) error {
	if _, err := s.PasskeyByID(ctx, userID, passkeyID); err != nil {
		return err
	}
	return s.records.Delete(ctx, authPasskeysNS, passkeyID)
}

// PasskeyByID receives a user id and passkey id and returns the owned credential.
func (s *SQLStore) PasskeyByID(ctx context.Context, userID string, passkeyID string) (PasskeyCredential, error) {
	var passkey PasskeyCredential
	if err := s.records.Get(ctx, authPasskeysNS, passkeyID, &passkey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasskeyCredential{}, errors.WithStack(errors.New("passkey not found"))
		}
		return PasskeyCredential{}, errors.Wrap(err, "load passkey")
	}
	if passkey.UserID != userID {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey not found"))
	}
	return clonePasskeyCredential(passkey), nil
}

// PasskeyByCredentialID receives a raw WebAuthn credential id and returns the matching passkey.
func (s *SQLStore) PasskeyByCredentialID(ctx context.Context, credentialID []byte) (PasskeyCredential, error) {
	var passkey PasskeyCredential
	if err := s.records.GetBySecondary(ctx, authPasskeysNS, credentialKey(credentialID), &passkey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasskeyCredential{}, errors.WithStack(errors.New("passkey not found"))
		}
		return PasskeyCredential{}, errors.Wrap(err, "load passkey by credential")
	}
	return clonePasskeyCredential(passkey), nil
}

// ListPasskeys receives a user id and returns all passkeys owned by the user.
func (s *SQLStore) ListPasskeys(ctx context.Context, userID string) ([]PasskeyCredential, error) {
	var passkeys []PasskeyCredential
	if err := s.records.List(ctx, authPasskeysNS, nil, &userID, &passkeys); err != nil {
		return nil, errors.Wrap(err, "list passkeys")
	}
	slices.SortFunc(passkeys, func(left PasskeyCredential, right PasskeyCredential) int {
		return stringsCompare(left.ID, right.ID)
	})
	return clonePasskeyCredentials(passkeys), nil
}

// StorePasskeyCeremony receives WebAuthn challenge state and stores it by ceremony id.
func (s *SQLStore) StorePasskeyCeremony(ctx context.Context, ceremony PasskeyCeremony) error {
	return s.records.Upsert(ctx, record(authPasskeyCeremoniesNS, ceremony.ID, ceremony.UserID, "", ceremony))
}

// PasskeyCeremony receives a ceremony id and returns the stored WebAuthn challenge state.
func (s *SQLStore) PasskeyCeremony(ctx context.Context, ceremonyID string) (PasskeyCeremony, error) {
	var ceremony PasskeyCeremony
	if err := s.records.Get(ctx, authPasskeyCeremoniesNS, ceremonyID, &ceremony); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasskeyCeremony{}, errors.WithStack(errors.New("passkey ceremony not found"))
		}
		return PasskeyCeremony{}, errors.Wrap(err, "load passkey ceremony")
	}
	return ceremony, nil
}

// DeletePasskeyCeremony receives a ceremony id and removes the matching WebAuthn challenge state.
func (s *SQLStore) DeletePasskeyCeremony(ctx context.Context, ceremonyID string) error {
	return s.records.Delete(ctx, authPasskeyCeremoniesNS, ceremonyID)
}

func record(namespace string, key string, ownerKey string, secondaryKey string, value any) persistence.Record {
	return persistence.Record{Namespace: namespace, Key: key, OwnerKey: ownerKey, SecondaryKey: secondaryKey, Data: mustJSON(value)}
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func emailCodeRecordKey(email string, purpose EmailCodePurpose) string {
	return persistence.JoinKey(email, string(purpose))
}

func replayKey(userID string, codeHash string) string {
	return persistence.JoinKey(userID, codeHash)
}

func passkeyRecord(passkey PasskeyCredential) persistence.Record {
	return record(authPasskeysNS, passkey.ID, passkey.UserID, credentialKey(passkey.CredentialID), passkey)
}

func credentialKey(credentialID []byte) string {
	return base64.RawURLEncoding.EncodeToString(credentialID)
}

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

func clonePasskeyCredentials(passkeys []PasskeyCredential) []PasskeyCredential {
	cloned := make([]PasskeyCredential, 0, len(passkeys))
	for _, passkey := range passkeys {
		cloned = append(cloned, clonePasskeyCredential(passkey))
	}
	return cloned
}
