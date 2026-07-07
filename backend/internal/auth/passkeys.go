package auth

import (
	"bytes"
	"context"
	"crypto/subtle"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

const maxPasskeyLabelLength = 80

type passkeyUser struct {
	user        User
	credentials []webauthnlib.Credential
}

// BeginPasskeyRegistration receives an authenticated actor and returns WebAuthn registration options.
func (s *Service) BeginPasskeyRegistration(ctx context.Context, actor Actor) (PasskeyRegistrationStart, error) {
	webAuthn, err := s.passkeyWebAuthn()
	if err != nil {
		return PasskeyRegistrationStart{}, err
	}

	record, err := s.store.UserByID(ctx, actor.UserID)
	if err != nil {
		return PasskeyRegistrationStart{}, errors.Wrap(err, "load passkey registration user")
	}
	if record.Status != UserStatusActive {
		return PasskeyRegistrationStart{}, errors.WithStack(ErrInvalidCredentials)
	}

	user, err := s.passkeyUser(ctx, record.User)
	if err != nil {
		return PasskeyRegistrationStart{}, err
	}
	options, session, err := webAuthn.BeginRegistration(user,
		webauthnlib.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthnlib.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthnlib.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return PasskeyRegistrationStart{}, errors.Wrap(err, "begin passkey registration")
	}

	flowID, err := NewSessionToken()
	if err != nil {
		return PasskeyRegistrationStart{}, err
	}
	ceremony := PasskeyCeremony{
		ID:        flowID,
		UserID:    record.ID,
		Type:      PasskeyCeremonyRegistration,
		Session:   *session,
		CreatedAt: s.clock().UTC(),
		ExpiresAt: session.Expires.UTC(),
	}
	if err := s.store.StorePasskeyCeremony(ctx, ceremony); err != nil {
		return PasskeyRegistrationStart{}, errors.Wrap(err, "store passkey registration ceremony")
	}

	return PasskeyRegistrationStart{
		FlowID:  flowID,
		Options: options,
	}, nil
}

// FinishPasskeyRegistration receives a ceremony id and credential response and stores the verified passkey.
func (s *Service) FinishPasskeyRegistration(ctx context.Context, request PasskeyRegistrationFinishRequest, credentialJSON []byte) (PasskeyListItem, error) {
	webAuthn, err := s.passkeyWebAuthn()
	if err != nil {
		return PasskeyListItem{}, err
	}
	if len(credentialJSON) == 0 {
		return PasskeyListItem{}, errors.WithStack(errors.New("passkey credential response is required"))
	}

	ceremony, err := s.store.PasskeyCeremony(ctx, request.FlowID)
	if err != nil {
		return PasskeyListItem{}, errors.Wrap(err, "load passkey registration ceremony")
	}
	defer func() {
		_ = s.store.DeletePasskeyCeremony(ctx, request.FlowID)
	}()
	if ceremony.Type != PasskeyCeremonyRegistration || ceremony.UserID != request.Actor.UserID {
		return PasskeyListItem{}, errors.WithStack(ErrInvalidCredentials)
	}
	if !ceremony.ExpiresAt.After(s.clock().UTC()) {
		return PasskeyListItem{}, errors.WithStack(errors.New("passkey registration ceremony expired"))
	}

	record, err := s.store.UserByID(ctx, request.Actor.UserID)
	if err != nil {
		return PasskeyListItem{}, errors.Wrap(err, "load passkey registration finish user")
	}
	user, err := s.passkeyUser(ctx, record.User)
	if err != nil {
		return PasskeyListItem{}, err
	}
	httpRequest, err := webAuthnCredentialRequest(ctx, credentialJSON)
	if err != nil {
		return PasskeyListItem{}, err
	}
	credential, err := webAuthn.FinishRegistration(user, ceremony.Session, httpRequest)
	if err != nil {
		return PasskeyListItem{}, errors.Wrap(err, "finish passkey registration")
	}

	now := s.clock().UTC()
	passkey := passkeyFromCredential(record.ID, request.Label, *credential, now)
	stored, err := s.store.CreatePasskey(ctx, passkey)
	if err != nil {
		return PasskeyListItem{}, errors.Wrap(err, "store passkey credential")
	}

	return passkeyListItem(stored), nil
}

// BeginPasskeyLogin receives a context and returns WebAuthn discoverable login options.
func (s *Service) BeginPasskeyLogin(ctx context.Context) (PasskeyLoginStart, error) {
	webAuthn, err := s.passkeyWebAuthn()
	if err != nil {
		return PasskeyLoginStart{}, err
	}

	options, session, err := webAuthn.BeginDiscoverableLogin(webauthnlib.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return PasskeyLoginStart{}, errors.Wrap(err, "begin passkey login")
	}

	flowID, err := NewSessionToken()
	if err != nil {
		return PasskeyLoginStart{}, err
	}
	ceremony := PasskeyCeremony{
		ID:        flowID,
		Type:      PasskeyCeremonyLogin,
		Session:   *session,
		CreatedAt: s.clock().UTC(),
		ExpiresAt: session.Expires.UTC(),
	}
	if err := s.store.StorePasskeyCeremony(ctx, ceremony); err != nil {
		return PasskeyLoginStart{}, errors.Wrap(err, "store passkey login ceremony")
	}

	return PasskeyLoginStart{
		FlowID:  flowID,
		Options: options,
	}, nil
}

// FinishPasskeyLogin receives a ceremony id and credential response and creates an authenticated session.
func (s *Service) FinishPasskeyLogin(ctx context.Context, request PasskeyLoginFinishRequest, credentialJSON []byte) (AuthResult, error) {
	webAuthn, err := s.passkeyWebAuthn()
	if err != nil {
		return AuthResult{}, err
	}
	if len(credentialJSON) == 0 {
		return AuthResult{}, errors.WithStack(errors.New("passkey credential response is required"))
	}

	ceremony, err := s.store.PasskeyCeremony(ctx, request.FlowID)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "load passkey login ceremony")
	}
	defer func() {
		_ = s.store.DeletePasskeyCeremony(ctx, request.FlowID)
	}()
	if ceremony.Type != PasskeyCeremonyLogin {
		return AuthResult{}, errors.WithStack(ErrInvalidCredentials)
	}
	if !ceremony.ExpiresAt.After(s.clock().UTC()) {
		return AuthResult{}, errors.WithStack(errors.New("passkey login ceremony expired"))
	}

	var authenticatedUser User
	handler := func(rawID, userHandle []byte) (webauthnlib.User, error) {
		passkey, err := s.store.PasskeyByCredentialID(ctx, rawID)
		if err != nil {
			return nil, errors.WithStack(ErrInvalidCredentials)
		}
		if subtle.ConstantTimeCompare([]byte(passkey.UserID), userHandle) != 1 {
			return nil, errors.WithStack(ErrInvalidCredentials)
		}
		record, err := s.store.UserByID(ctx, passkey.UserID)
		if err != nil || record.Status != UserStatusActive {
			return nil, errors.WithStack(ErrInvalidCredentials)
		}
		authenticatedUser = record.User

		return s.passkeyUser(ctx, record.User)
	}

	httpRequest, err := webAuthnCredentialRequest(ctx, credentialJSON)
	if err != nil {
		return AuthResult{}, err
	}
	_, credential, err := webAuthn.FinishPasskeyLogin(handler, ceremony.Session, httpRequest)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "finish passkey login")
	}

	passkey, err := s.store.PasskeyByCredentialID(ctx, credential.ID)
	if err != nil {
		return AuthResult{}, errors.Wrap(err, "load passkey credential after login")
	}
	now := s.clock().UTC()
	refreshPasskeyFromCredential(&passkey, *credential, now)
	if _, err := s.store.UpdatePasskey(ctx, passkey); err != nil {
		return AuthResult{}, errors.Wrap(err, "update passkey login metadata")
	}

	token, err := NewSessionToken()
	if err != nil {
		return AuthResult{}, err
	}
	session := Session{
		ID:        uuid.NewString(),
		UserID:    authenticatedUser.ID,
		UserEmail: authenticatedUser.Email,
		Status:    authenticatedUser.Status,
		CreatedAt: now,
		ExpiresAt: now.Add(s.cfg.SessionTTL).UTC(),
	}
	if err := s.store.StoreSession(ctx, HashSessionToken(token), session); err != nil {
		return AuthResult{}, errors.Wrap(err, "store passkey login session")
	}
	if err := s.store.ResetLoginThrottle(ctx, authenticatedUser.Email); err != nil {
		return AuthResult{}, errors.Wrap(err, "reset login throttle")
	}

	return AuthResult{
		User:         authenticatedUser,
		Session:      session,
		SessionToken: token,
	}, nil
}

// ListPasskeys receives an authenticated actor and returns a page of public passkey metadata.
func (s *Service) ListPasskeys(ctx context.Context, request PasskeyListRequest) (Page[PasskeyListItem], error) {
	if _, err := s.passkeyWebAuthn(); err != nil {
		return Page[PasskeyListItem]{}, err
	}

	passkeys, err := s.store.ListPasskeys(ctx, request.Actor.UserID)
	if err != nil {
		return Page[PasskeyListItem]{}, errors.Wrap(err, "list passkeys")
	}
	items := make([]PasskeyListItem, 0, len(passkeys))
	for _, passkey := range passkeys {
		items = append(items, passkeyListItem(passkey))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	return paginate(items, request.Page, request.PageSize), nil
}

// UpdatePasskey receives a passkey id and label and returns the updated passkey metadata.
func (s *Service) UpdatePasskey(ctx context.Context, request PasskeyUpdateRequest) (PasskeyListItem, error) {
	if _, err := s.passkeyWebAuthn(); err != nil {
		return PasskeyListItem{}, err
	}

	passkey, err := s.store.PasskeyByID(ctx, request.Actor.UserID, request.PasskeyID)
	if err != nil {
		return PasskeyListItem{}, errors.Wrap(err, "load passkey for update")
	}
	label, err := normalizePasskeyLabel(request.Label)
	if err != nil {
		return PasskeyListItem{}, err
	}
	passkey.Label = label
	passkey.UpdatedAt = s.clock().UTC()

	updated, err := s.store.UpdatePasskey(ctx, passkey)
	if err != nil {
		return PasskeyListItem{}, errors.Wrap(err, "update passkey")
	}

	return passkeyListItem(updated), nil
}

// DeletePasskey receives an authenticated actor and passkey id and deletes the owned passkey.
func (s *Service) DeletePasskey(ctx context.Context, actor Actor, passkeyID string) error {
	if _, err := s.passkeyWebAuthn(); err != nil {
		return err
	}
	if strings.TrimSpace(passkeyID) == "" {
		return errors.WithStack(errors.New("passkey id is required"))
	}
	if err := s.store.DeletePasskey(ctx, actor.UserID, passkeyID); err != nil {
		return errors.Wrap(err, "delete passkey")
	}

	return nil
}

// WebAuthnID returns the stable WebAuthn user handle.
func (u passkeyUser) WebAuthnID() []byte {
	return []byte(u.user.ID)
}

// WebAuthnName returns a human-readable WebAuthn account name.
func (u passkeyUser) WebAuthnName() string {
	return u.user.Email
}

// WebAuthnDisplayName returns a human-readable WebAuthn display name.
func (u passkeyUser) WebAuthnDisplayName() string {
	return u.user.Email
}

// WebAuthnCredentials returns all WebAuthn credentials owned by the user.
func (u passkeyUser) WebAuthnCredentials() []webauthnlib.Credential {
	return append([]webauthnlib.Credential(nil), u.credentials...)
}

// passkeyWebAuthn returns a configured WebAuthn relying-party instance.
func (s *Service) passkeyWebAuthn() (*webauthnlib.WebAuthn, error) {
	if !s.cfg.PasskeyEnabled {
		return nil, errors.WithStack(errors.New("passkeys are disabled"))
	}

	webAuthn, err := webauthnlib.New(&webauthnlib.Config{
		RPID:          strings.TrimSpace(s.cfg.PasskeyRPID),
		RPDisplayName: strings.TrimSpace(s.cfg.PasskeyRPDisplayName),
		RPOrigins:     []string{strings.TrimSpace(s.cfg.PasskeyRPOrigin)},
		Timeouts: webauthnlib.TimeoutsConfig{
			Login:        webauthnlib.TimeoutConfig{Enforce: true},
			Registration: webauthnlib.TimeoutConfig{Enforce: true},
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "configure passkeys")
	}

	return webAuthn, nil
}

// passkeyUser receives a user and returns the WebAuthn user adapter with stored credentials.
func (s *Service) passkeyUser(ctx context.Context, user User) (passkeyUser, error) {
	passkeys, err := s.store.ListPasskeys(ctx, user.ID)
	if err != nil {
		return passkeyUser{}, errors.Wrap(err, "list user passkey credentials")
	}
	credentials := make([]webauthnlib.Credential, 0, len(passkeys))
	for _, passkey := range passkeys {
		credentials = append(credentials, passkey.Credential)
	}

	return passkeyUser{
		user:        user,
		credentials: credentials,
	}, nil
}

// webAuthnCredentialRequest receives WebAuthn credential JSON and returns an HTTP request for library parsing.
func webAuthnCredentialRequest(ctx context.Context, credentialJSON []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(credentialJSON))
	if err != nil {
		return nil, errors.Wrap(err, "create passkey credential request")
	}
	request.Header.Set("Content-Type", "application/json")

	return request, nil
}

// passkeyFromCredential receives a verified WebAuthn credential and returns a stored passkey record.
func passkeyFromCredential(userID string, label string, credential webauthnlib.Credential, now time.Time) PasskeyCredential {
	passkey := PasskeyCredential{
		ID:         uuid.NewString(),
		UserID:     userID,
		Label:      defaultPasskeyLabel(label),
		Credential: credential,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	refreshPasskeyFromCredential(&passkey, credential, now)
	passkey.LastUsedAt = nil

	return passkey
}

// refreshPasskeyFromCredential receives a passkey and WebAuthn credential and updates stored metadata.
func refreshPasskeyFromCredential(passkey *PasskeyCredential, credential webauthnlib.Credential, now time.Time) {
	passkey.Credential = credential
	passkey.CredentialID = append([]byte(nil), credential.ID...)
	passkey.PublicKey = append([]byte(nil), credential.PublicKey...)
	passkey.SignCount = credential.Authenticator.SignCount
	passkey.BackupEligible = credential.Flags.BackupEligible
	passkey.BackupState = credential.Flags.BackupState
	passkey.AAGUID = append([]byte(nil), credential.Authenticator.AAGUID...)
	passkey.AttestationType = credential.AttestationType
	passkey.AttestationFormat = credential.AttestationFormat
	passkey.AuthenticatorAttachment = string(credential.Authenticator.Attachment)
	passkey.Transports = make([]string, 0, len(credential.Transport))
	for _, transport := range credential.Transport {
		passkey.Transports = append(passkey.Transports, string(transport))
	}
	passkey.LastUsedAt = &now
	passkey.UpdatedAt = now
}

// passkeyListItem receives a stored passkey and returns public API metadata.
func passkeyListItem(passkey PasskeyCredential) PasskeyListItem {
	return PasskeyListItem{
		ID:             passkey.ID,
		Label:          passkey.Label,
		Transports:     append([]string(nil), passkey.Transports...),
		BackupEligible: passkey.BackupEligible,
		BackupState:    passkey.BackupState,
		SignCount:      passkey.SignCount,
		CreatedAt:      passkey.CreatedAt,
		UpdatedAt:      passkey.UpdatedAt,
		LastUsedAt:     passkey.LastUsedAt,
	}
}

// normalizePasskeyLabel receives a user-provided label and returns a bounded display label.
func normalizePasskeyLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", errors.WithStack(errors.New("passkey label is required"))
	}
	if len(label) > maxPasskeyLabelLength {
		return "", errors.WithStack(errors.New("passkey label is too long"))
	}

	return label, nil
}

// defaultPasskeyLabel receives an optional label and returns a bounded passkey display label.
func defaultPasskeyLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" || len(label) > maxPasskeyLabelLength {
		return "Passkey"
	}

	return label
}
