package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// SQLRepository persists auth records in relational users/sessions tables plus auth_kv.
type SQLRepository struct {
	db      *storage.DB
	dialect string
}

type authKVRecord struct {
	Namespace    string
	Key          string
	OwnerKey     string
	SecondaryKey string
	Data         []byte
	ExpiresAt    *time.Time
}

// NewSQLRepository receives a migrated storage database and returns an authentication Store.
func NewSQLRepository(db *storage.DB) (*SQLRepository, error) {
	if db == nil || db.SQLDB() == nil {
		return nil, errors.WithStack(errors.New("storage database is nil"))
	}
	return &SQLRepository{db: db, dialect: db.Dialect()}, nil
}

// CreateUser receives a user record and stores it when its email and id are unique.
func (s *SQLRepository) CreateUser(ctx context.Context, user UserRecord) (UserRecord, error) {
	user = cloneUserRecord(user)
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO users (
			id, email, status, email_verified, totp_enabled, base_currency, password_hash,
			totp_secret, external_sso_subject, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		user.ID, user.Email, user.Status, user.EmailVerified, user.TOTPEnabled, user.BaseCurrency,
		user.PasswordHash, user.TOTPSecret, user.ExternalSSOSubject, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return UserRecord{}, errors.Wrap(err, "insert user")
	}
	return cloneUserRecord(user), nil
}

// UserByEmail receives an email and returns the matching user record using case-insensitive lookup.
func (s *SQLRepository) UserByEmail(ctx context.Context, email string) (UserRecord, error) {
	row := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT id, email, status, email_verified, totp_enabled, base_currency, password_hash,
			totp_secret, external_sso_subject, created_at, updated_at
		FROM users WHERE lower(email) = lower(?)`), email)
	return scanUserRecord(row)
}

// UserByID receives a user id and returns the matching user record.
func (s *SQLRepository) UserByID(ctx context.Context, userID string) (UserRecord, error) {
	row := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT id, email, status, email_verified, totp_enabled, base_currency, password_hash,
			totp_secret, external_sso_subject, created_at, updated_at
		FROM users WHERE id = ?`), userID)
	return scanUserRecord(row)
}

// UpdateUser receives a user record and replaces the stored record for the same id and email.
func (s *SQLRepository) UpdateUser(ctx context.Context, user UserRecord) (UserRecord, error) {
	existing, err := s.UserByID(ctx, user.ID)
	if err != nil {
		return UserRecord{}, err
	}
	if existing.Email != user.Email {
		return UserRecord{}, errors.WithStack(errors.New("user email cannot change"))
	}
	user = cloneUserRecord(user)
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		UPDATE users SET status = ?, email_verified = ?, totp_enabled = ?, base_currency = ?,
			password_hash = ?, totp_secret = ?, external_sso_subject = ?, updated_at = ?
		WHERE id = ?`),
		user.Status, user.EmailVerified, user.TOTPEnabled, user.BaseCurrency, user.PasswordHash,
		user.TOTPSecret, user.ExternalSSOSubject, user.UpdatedAt, user.ID)
	if err != nil {
		return UserRecord{}, errors.Wrap(err, "update user")
	}
	if err := authRowsAffected(result, "user not found"); err != nil {
		return UserRecord{}, err
	}
	return cloneUserRecord(user), nil
}

// StoreSession receives a hashed session token and stores the associated session metadata.
func (s *SQLRepository) StoreSession(ctx context.Context, tokenHash string, session Session) error {
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO sessions (token_hash, id, user_id, user_email, status, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(token_hash) DO UPDATE SET
			id = excluded.id,
			user_id = excluded.user_id,
			user_email = excluded.user_email,
			status = excluded.status,
			expires_at = excluded.expires_at,
			created_at = excluded.created_at`),
		tokenHash, session.ID, session.UserID, session.UserEmail, session.Status, session.ExpiresAt.UTC(), session.CreatedAt.UTC())
	if err != nil {
		return errors.Wrap(err, "store session")
	}
	return nil
}

// SessionByTokenHash receives a hashed session token and returns the matching active session.
func (s *SQLRepository) SessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	var session Session
	err := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT id, user_id, user_email, status, expires_at, created_at FROM sessions WHERE token_hash = ?`), tokenHash).
		Scan(&session.ID, &session.UserID, &session.UserEmail, &session.Status, (*authSQLTime)(&session.ExpiresAt), (*authSQLTime)(&session.CreatedAt))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, errors.WithStack(errors.New("session not found"))
	}
	if err != nil {
		return Session{}, errors.Wrap(err, "load session")
	}
	return session, nil
}

// DeleteSession receives a hashed session token and removes the associated session when present.
func (s *SQLRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `DELETE FROM sessions WHERE token_hash = ?`), tokenHash)
	if err != nil {
		return errors.Wrap(err, "delete session")
	}
	return nil
}

// DeleteSessionsByUser receives a user id and removes all sessions owned by that user.
func (s *SQLRepository) DeleteSessionsByUser(ctx context.Context, userID string) error {
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `DELETE FROM sessions WHERE user_id = ?`), userID)
	if err != nil {
		return errors.Wrap(err, "delete sessions by user")
	}
	return nil
}

// MigrateTOTPSecrets receives a secret transform and rewrites stored TOTP secrets in place.
func (s *SQLRepository) MigrateTOTPSecrets(ctx context.Context, encrypt func(userID string, secret string) (string, error)) error {
	users, err := s.listUsers(ctx)
	if err != nil {
		return err
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

	records, err := s.kvList(ctx, authPendingTOTPNS)
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
		record.Data = authMustJSON(setup)
		if err := s.kvUpsert(ctx, record); err != nil {
			return errors.Wrap(err, "update migrated pending totp setup")
		}
	}
	return nil
}

// StoreEmailCode receives a one-time email code record and stores it by email and purpose.
func (s *SQLRepository) StoreEmailCode(ctx context.Context, code EmailCodeRecord) error {
	code = cloneEmailCodeRecord(code)
	return s.kvUpsert(ctx, authKVRecord{Namespace: authEmailCodesNS, Key: emailCodeRecordKey(code.Email, code.Purpose), Data: authMustJSON(code), ExpiresAt: &code.ExpiresAt})
}

// EmailCode receives email and purpose and returns the matching one-time code record.
func (s *SQLRepository) EmailCode(ctx context.Context, email string, purpose EmailCodePurpose) (EmailCodeRecord, error) {
	var code EmailCodeRecord
	if err := s.kvGet(ctx, authEmailCodesNS, emailCodeRecordKey(email, purpose), &code); err != nil {
		return EmailCodeRecord{}, authNotFound(err, "email code not found", "load email code")
	}
	return cloneEmailCodeRecord(code), nil
}

// DeleteEmailCode receives email and purpose and removes the matching one-time code record.
func (s *SQLRepository) DeleteEmailCode(ctx context.Context, email string, purpose EmailCodePurpose) error {
	return s.kvDelete(ctx, authEmailCodesNS, emailCodeRecordKey(email, purpose))
}

// StorePendingTOTPSetup receives a session id and stores its pending unconfirmed TOTP setup.
func (s *SQLRepository) StorePendingTOTPSetup(ctx context.Context, sessionID string, setup PendingTOTPSetup) error {
	return s.kvUpsert(ctx, authKVRecord{Namespace: authPendingTOTPNS, Key: sessionID, OwnerKey: setup.UserID, Data: authMustJSON(setup), ExpiresAt: &setup.ExpiresAt})
}

// PendingTOTPSetup receives a session id and returns its pending unconfirmed TOTP setup.
func (s *SQLRepository) PendingTOTPSetup(ctx context.Context, sessionID string) (PendingTOTPSetup, error) {
	var setup PendingTOTPSetup
	if err := s.kvGet(ctx, authPendingTOTPNS, sessionID, &setup); err != nil {
		return PendingTOTPSetup{}, authNotFound(err, "pending totp setup not found", "load pending totp setup")
	}
	return setup, nil
}

// DeletePendingTOTPSetup receives a session id and removes its pending TOTP setup.
func (s *SQLRepository) DeletePendingTOTPSetup(ctx context.Context, sessionID string) error {
	return s.kvDelete(ctx, authPendingTOTPNS, sessionID)
}

// StoreTOTPReplay receives a user id, code hash, and expiry and stores a replay marker.
func (s *SQLRepository) StoreTOTPReplay(ctx context.Context, userID string, codeHash string, expiresAt time.Time) error {
	replay := TOTPReplaySnapshot{UserID: userID, CodeHash: codeHash, ExpiresAt: expiresAt.UTC()}
	return s.kvUpsert(ctx, authKVRecord{Namespace: authTOTPReplaysNS, Key: replayKey(userID, codeHash), OwnerKey: userID, Data: authMustJSON(replay), ExpiresAt: &replay.ExpiresAt})
}

// TOTPReplayExists receives a user id, code hash, and current time and reports whether the code was reused.
func (s *SQLRepository) TOTPReplayExists(ctx context.Context, userID string, codeHash string, now time.Time) (bool, error) {
	var replay TOTPReplaySnapshot
	key := replayKey(userID, codeHash)
	if err := s.kvGet(ctx, authTOTPReplaysNS, key, &replay); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, errors.Wrap(err, "load totp replay")
	}
	if !replay.ExpiresAt.After(now.UTC()) {
		if err := s.kvDelete(ctx, authTOTPReplaysNS, key); err != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// IncrementFailedTOTP receives a user id and increments its failed TOTP counter.
func (s *SQLRepository) IncrementFailedTOTP(ctx context.Context, userID string) (int, error) {
	return s.incrementCounter(ctx, authFailedTOTPsNS, userID)
}

// ResetFailedTOTP receives a user id and clears its failed TOTP counter.
func (s *SQLRepository) ResetFailedTOTP(ctx context.Context, userID string) error {
	return s.kvDelete(ctx, authFailedTOTPsNS, userID)
}

// FailedTOTPCount receives a user id and returns its failed TOTP counter.
func (s *SQLRepository) FailedTOTPCount(ctx context.Context, userID string) (int, error) {
	return s.counter(ctx, authFailedTOTPsNS, userID)
}

// LoginThrottle receives a normalized email and returns its password-login throttle state.
func (s *SQLRepository) LoginThrottle(ctx context.Context, email string) (LoginThrottle, error) {
	var throttle LoginThrottle
	if err := s.kvGet(ctx, authFailedLoginsNS, email, &throttle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginThrottle{}, nil
		}
		var legacyCount int
		if legacyErr := s.kvGet(ctx, authFailedLoginsNS, email, &legacyCount); legacyErr == nil {
			return LoginThrottle{Email: email, FailedCount: legacyCount}, nil
		}
		return LoginThrottle{}, errors.Wrap(err, "load login throttle")
	}
	return throttle, nil
}

// StoreLoginThrottle receives password-login throttle state and stores it by normalized email.
func (s *SQLRepository) StoreLoginThrottle(ctx context.Context, throttle LoginThrottle) error {
	if strings.TrimSpace(throttle.Email) == "" {
		return errors.WithStack(errors.New("login throttle email is required"))
	}
	throttle.LockedUntil = throttle.LockedUntil.UTC()
	throttle.UpdatedAt = throttle.UpdatedAt.UTC()
	return s.kvUpsert(ctx, authKVRecord{Namespace: authFailedLoginsNS, Key: throttle.Email, Data: authMustJSON(throttle), ExpiresAt: &throttle.LockedUntil})
}

// ResetLoginThrottle receives a normalized email and clears its password-login throttle state.
func (s *SQLRepository) ResetLoginThrottle(ctx context.Context, email string) error {
	return s.kvDelete(ctx, authFailedLoginsNS, email)
}

// CreatePasskey receives a passkey credential and stores it when its ids are unique.
func (s *SQLRepository) CreatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
	passkey = clonePasskeyCredential(passkey)
	if err := s.kvInsert(ctx, passkeyKV(passkey)); err != nil {
		return PasskeyCredential{}, errors.Wrap(err, "insert passkey")
	}
	return clonePasskeyCredential(passkey), nil
}

// UpdatePasskey receives a passkey credential and replaces the stored credential for the same id.
func (s *SQLRepository) UpdatePasskey(ctx context.Context, passkey PasskeyCredential) (PasskeyCredential, error) {
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
	if err := s.kvUpdate(ctx, passkeyKV(passkey)); err != nil {
		return PasskeyCredential{}, errors.Wrap(err, "update passkey")
	}
	return clonePasskeyCredential(passkey), nil
}

// DeletePasskey receives a user id and passkey id and removes the matching credential.
func (s *SQLRepository) DeletePasskey(ctx context.Context, userID string, passkeyID string) error {
	if _, err := s.PasskeyByID(ctx, userID, passkeyID); err != nil {
		return err
	}
	return s.kvDelete(ctx, authPasskeysNS, passkeyID)
}

// PasskeyByID receives a user id and passkey id and returns the owned credential.
func (s *SQLRepository) PasskeyByID(ctx context.Context, userID string, passkeyID string) (PasskeyCredential, error) {
	var passkey PasskeyCredential
	if err := s.kvGet(ctx, authPasskeysNS, passkeyID, &passkey); err != nil {
		return PasskeyCredential{}, authNotFound(err, "passkey not found", "load passkey")
	}
	if passkey.UserID != userID {
		return PasskeyCredential{}, errors.WithStack(errors.New("passkey not found"))
	}
	return clonePasskeyCredential(passkey), nil
}

// PasskeyByCredentialID receives a raw WebAuthn credential id and returns the matching passkey.
func (s *SQLRepository) PasskeyByCredentialID(ctx context.Context, credentialID []byte) (PasskeyCredential, error) {
	var passkey PasskeyCredential
	if err := s.kvGetBySecondary(ctx, authPasskeysNS, credentialKey(credentialID), &passkey); err != nil {
		return PasskeyCredential{}, authNotFound(err, "passkey not found", "load passkey by credential")
	}
	return clonePasskeyCredential(passkey), nil
}

// ListPasskeys receives a user id and returns all passkeys owned by the user.
func (s *SQLRepository) ListPasskeys(ctx context.Context, userID string) ([]PasskeyCredential, error) {
	records, err := s.kvListByOwner(ctx, authPasskeysNS, userID)
	if err != nil {
		return nil, errors.Wrap(err, "list passkeys")
	}
	passkeys := make([]PasskeyCredential, 0, len(records))
	for _, record := range records {
		var passkey PasskeyCredential
		if err := json.Unmarshal(record.Data, &passkey); err != nil {
			return nil, errors.Wrap(err, "decode passkey")
		}
		passkeys = append(passkeys, passkey)
	}
	slices.SortFunc(passkeys, func(left PasskeyCredential, right PasskeyCredential) int {
		return stringsCompare(left.ID, right.ID)
	})
	return clonePasskeyCredentials(passkeys), nil
}

// StorePasskeyCeremony receives WebAuthn challenge state and stores it by ceremony id.
func (s *SQLRepository) StorePasskeyCeremony(ctx context.Context, ceremony PasskeyCeremony) error {
	return s.kvUpsert(ctx, authKVRecord{Namespace: authPasskeyCeremoniesNS, Key: ceremony.ID, OwnerKey: ceremony.UserID, Data: authMustJSON(ceremony)})
}

// PasskeyCeremony receives a ceremony id and returns the stored WebAuthn challenge state.
func (s *SQLRepository) PasskeyCeremony(ctx context.Context, ceremonyID string) (PasskeyCeremony, error) {
	var ceremony PasskeyCeremony
	if err := s.kvGet(ctx, authPasskeyCeremoniesNS, ceremonyID, &ceremony); err != nil {
		return PasskeyCeremony{}, authNotFound(err, "passkey ceremony not found", "load passkey ceremony")
	}
	return ceremony, nil
}

// DeletePasskeyCeremony receives a ceremony id and removes the matching WebAuthn challenge state.
func (s *SQLRepository) DeletePasskeyCeremony(ctx context.Context, ceremonyID string) error {
	return s.kvDelete(ctx, authPasskeyCeremoniesNS, ceremonyID)
}

func (s *SQLRepository) listUsers(ctx context.Context) ([]UserRecord, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, `
		SELECT id, email, status, email_verified, totp_enabled, base_currency, password_hash,
			totp_secret, external_sso_subject, created_at, updated_at
		FROM users ORDER BY id`)
	if err != nil {
		return nil, errors.Wrap(err, "list users")
	}
	defer func() { _ = rows.Close() }()

	users := []UserRecord{}
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate users")
	}
	return users, nil
}

func scanUserRecord(row *sql.Row) (UserRecord, error) {
	var user UserRecord
	if err := row.Scan(&user.ID, &user.Email, &user.Status, (*authSQLBool)(&user.EmailVerified), (*authSQLBool)(&user.TOTPEnabled), &user.BaseCurrency, &user.PasswordHash, &user.TOTPSecret, &user.ExternalSSOSubject, (*authSQLTime)(&user.CreatedAt), (*authSQLTime)(&user.UpdatedAt)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserRecord{}, errors.WithStack(errors.New("user not found"))
		}
		return UserRecord{}, errors.Wrap(err, "scan user")
	}
	return cloneUserRecord(user), nil
}

func scanUserRows(rows *sql.Rows) (UserRecord, error) {
	var user UserRecord
	if err := rows.Scan(&user.ID, &user.Email, &user.Status, (*authSQLBool)(&user.EmailVerified), (*authSQLBool)(&user.TOTPEnabled), &user.BaseCurrency, &user.PasswordHash, &user.TOTPSecret, &user.ExternalSSOSubject, (*authSQLTime)(&user.CreatedAt), (*authSQLTime)(&user.UpdatedAt)); err != nil {
		return UserRecord{}, errors.Wrap(err, "scan user")
	}
	return cloneUserRecord(user), nil
}

func passkeyKV(passkey PasskeyCredential) authKVRecord {
	return authKVRecord{
		Namespace:    authPasskeysNS,
		Key:          passkey.ID,
		OwnerKey:     passkey.UserID,
		SecondaryKey: credentialKey(passkey.CredentialID),
		Data:         authMustJSON(passkey),
	}
}

func authMustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func (s *SQLRepository) kvInsert(ctx context.Context, record authKVRecord) error {
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO auth_kv (namespace, record_key, owner_key, secondary_key, data, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`),
		record.Namespace, record.Key, record.OwnerKey, record.SecondaryKey, record.Data, record.ExpiresAt)
	if err != nil {
		return errors.Wrap(err, "insert auth kv record")
	}
	return nil
}

func (s *SQLRepository) kvUpsert(ctx context.Context, record authKVRecord) error {
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO auth_kv (namespace, record_key, owner_key, secondary_key, data, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(namespace, record_key) DO UPDATE SET
			owner_key = excluded.owner_key,
			secondary_key = excluded.secondary_key,
			data = excluded.data,
			expires_at = excluded.expires_at,
			updated_at = CURRENT_TIMESTAMP`),
		record.Namespace, record.Key, record.OwnerKey, record.SecondaryKey, record.Data, record.ExpiresAt)
	if err != nil {
		return errors.Wrap(err, "upsert auth kv record")
	}
	return nil
}

func (s *SQLRepository) kvUpdate(ctx context.Context, record authKVRecord) error {
	result, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		UPDATE auth_kv
		SET owner_key = ?, secondary_key = ?, data = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE namespace = ? AND record_key = ?`),
		record.OwnerKey, record.SecondaryKey, record.Data, record.ExpiresAt, record.Namespace, record.Key)
	if err != nil {
		return errors.Wrap(err, "update auth kv record")
	}
	return authRowsAffected(result, "auth kv record not found")
}

func (s *SQLRepository) kvGet(ctx context.Context, namespace string, key string, value any) error {
	var data []byte
	err := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT data FROM auth_kv WHERE namespace = ? AND record_key = ?`), namespace, key).Scan(&data)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return errors.Wrap(err, "decode auth kv record")
	}
	return nil
}

func (s *SQLRepository) kvGetBySecondary(ctx context.Context, namespace string, secondaryKey string, value any) error {
	var data []byte
	err := s.db.SQLDB().QueryRowContext(ctx, storage.Rebind(s.dialect, `
		SELECT data FROM auth_kv WHERE namespace = ? AND secondary_key = ?`), namespace, secondaryKey).Scan(&data)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return errors.Wrap(err, "decode auth kv record by secondary key")
	}
	return nil
}

func (s *SQLRepository) kvList(ctx context.Context, namespace string) ([]authKVRecord, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT namespace, record_key, owner_key, secondary_key, data, expires_at
		FROM auth_kv WHERE namespace = ? ORDER BY record_key`), namespace)
	if err != nil {
		return nil, errors.Wrap(err, "list auth kv records")
	}
	defer func() { _ = rows.Close() }()
	return scanAuthKVRows(rows)
}

func (s *SQLRepository) kvListByOwner(ctx context.Context, namespace string, ownerKey string) ([]authKVRecord, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, storage.Rebind(s.dialect, `
		SELECT namespace, record_key, owner_key, secondary_key, data, expires_at
		FROM auth_kv WHERE namespace = ? AND owner_key = ? ORDER BY record_key`), namespace, ownerKey)
	if err != nil {
		return nil, errors.Wrap(err, "list auth kv records by owner")
	}
	defer func() { _ = rows.Close() }()
	return scanAuthKVRows(rows)
}

func scanAuthKVRows(rows *sql.Rows) ([]authKVRecord, error) {
	records := []authKVRecord{}
	for rows.Next() {
		var record authKVRecord
		var expiresAt authNullableTime
		if err := rows.Scan(&record.Namespace, &record.Key, &record.OwnerKey, &record.SecondaryKey, &record.Data, &expiresAt); err != nil {
			return nil, errors.Wrap(err, "scan auth kv record")
		}
		if expiresAt.Valid {
			record.ExpiresAt = &expiresAt.Time
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate auth kv records")
	}
	return records, nil
}

func (s *SQLRepository) kvDelete(ctx context.Context, namespace string, key string) error {
	_, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, `
		DELETE FROM auth_kv WHERE namespace = ? AND record_key = ?`), namespace, key)
	if err != nil {
		return errors.Wrap(err, "delete auth kv record")
	}
	return nil
}

func (s *SQLRepository) counter(ctx context.Context, namespace string, key string) (int, error) {
	var count int
	if err := s.kvGet(ctx, namespace, key, &count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, errors.Wrap(err, "load auth counter")
	}
	return count, nil
}

func (s *SQLRepository) incrementCounter(ctx context.Context, namespace string, key string) (int, error) {
	var next int
	err := s.db.WithTx(ctx, func(tx storage.DBTX) error {
		var count int
		err := tx.QueryRowContext(ctx, storage.Rebind(s.dialect, `
			SELECT data FROM auth_kv WHERE namespace = ? AND record_key = ?`), namespace, key).Scan((*authJSONInt)(&count))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return errors.Wrap(err, "load auth counter for increment")
		}
		next = count + 1
		_, err = tx.ExecContext(ctx, storage.Rebind(s.dialect, `
			INSERT INTO auth_kv (namespace, record_key, data)
			VALUES (?, ?, ?)
			ON CONFLICT(namespace, record_key) DO UPDATE SET
				data = excluded.data,
				updated_at = CURRENT_TIMESTAMP`),
			namespace, key, authMustJSON(next))
		if err != nil {
			return errors.Wrap(err, "increment auth counter")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return next, nil
}

func authRowsAffected(result sql.Result, notFoundMsg string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "read rows affected")
	}
	if affected == 0 {
		return errors.WithStack(errors.New(notFoundMsg))
	}
	return nil
}

func authNotFound(err error, notFoundMsg string, wrapMsg string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errors.WithStack(errors.New(notFoundMsg))
	}
	return errors.Wrap(err, wrapMsg)
}

type authJSONInt int

func (value *authJSONInt) Scan(src any) error {
	var data []byte
	switch src := src.(type) {
	case string:
		data = []byte(src)
	case []byte:
		data = src
	default:
		return errors.Errorf("unsupported json int source %T", src)
	}
	var count int
	if err := json.Unmarshal(data, &count); err != nil {
		return errors.Wrap(err, "decode auth json int")
	}
	*value = authJSONInt(count)
	return nil
}

type authSQLBool bool

func (value *authSQLBool) Scan(src any) error {
	switch src := src.(type) {
	case bool:
		*value = authSQLBool(src)
		return nil
	case int64:
		*value = authSQLBool(src != 0)
		return nil
	case int:
		*value = authSQLBool(src != 0)
		return nil
	case []byte:
		return value.scanString(string(src))
	case string:
		return value.scanString(src)
	default:
		return errors.Errorf("unsupported bool source %T", src)
	}
}

func (value *authSQLBool) scanString(src string) error {
	switch strings.ToLower(strings.TrimSpace(src)) {
	case "1", "t", "true", "y", "yes":
		*value = true
		return nil
	case "0", "f", "false", "n", "no":
		*value = false
		return nil
	default:
		return errors.Errorf("unsupported bool value %q", src)
	}
}

type authSQLTime time.Time

func (value *authSQLTime) Scan(src any) error {
	parsed, err := parseAuthSQLTime(src)
	if err != nil {
		return err
	}
	*value = authSQLTime(parsed)
	return nil
}

type authNullableTime struct {
	Time  time.Time
	Valid bool
}

func (value *authNullableTime) Scan(src any) error {
	if src == nil {
		value.Time = time.Time{}
		value.Valid = false
		return nil
	}
	parsed, err := parseAuthSQLTime(src)
	if err != nil {
		return err
	}
	value.Time = parsed
	value.Valid = true
	return nil
}

func parseAuthSQLTime(src any) (time.Time, error) {
	switch src := src.(type) {
	case time.Time:
		return src.UTC(), nil
	case string:
		return parseAuthSQLTimeString(src)
	case []byte:
		return parseAuthSQLTimeString(string(src))
	default:
		return time.Time{}, errors.Errorf("unsupported time source %T", src)
	}
}

func parseAuthSQLTimeString(src string) (time.Time, error) {
	value := strings.TrimSpace(src)
	if value == "" {
		return time.Time{}, errors.WithStack(errors.New("empty time value"))
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	if unixNano, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(0, unixNano).UTC(), nil
	}
	return time.Time{}, errors.Errorf("parse time value %q", src)
}

var _ Store = (*SQLRepository)(nil)
