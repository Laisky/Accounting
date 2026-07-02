// Package auth contains authentication, password, Turnstile, and session use cases.
package auth

import (
	"time"

	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
)

// UserStatus identifies whether a user may authenticate into the application.
type UserStatus string

const (
	// UserStatusPendingVerification identifies a registered user awaiting email verification.
	UserStatusPendingVerification UserStatus = "pending_verification"
	// UserStatusActive identifies a user who can authenticate.
	UserStatusActive UserStatus = "active"
)

// User represents an authenticated identity without exposing password hash material.
type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Status        UserStatus `json:"status"`
	EmailVerified bool       `json:"emailVerified"`
	TOTPEnabled   bool       `json:"totpEnabled"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// UserRecord represents a stored user with password hash material kept server-side only.
type UserRecord struct {
	User
	PasswordHash string
	TOTPSecret   string
}

// Session represents a browser session with only stable identity and status metadata.
type Session struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	UserEmail string     `json:"userEmail"`
	Status    UserStatus `json:"status"`
	ExpiresAt time.Time  `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

// RegisterRequest contains input for email/password registration.
type RegisterRequest struct {
	Email          string
	Password       string
	TurnstileToken string
	RemoteIP       string
}

// LoginRequest contains input for email/password login.
type LoginRequest struct {
	Email          string
	Password       string
	TOTPCode       string
	TurnstileToken string
	RemoteIP       string
}

// ExternalSSOLoginRequest contains the opaque credential returned by the configured SSO provider.
type ExternalSSOLoginRequest struct {
	Token string
}

// ExternalSSOIdentity contains validated identity metadata from the configured SSO provider.
type ExternalSSOIdentity struct {
	Subject     string
	Username    string
	DisplayName string
}

// AuthResult contains authenticated user and session data returned by login.
type AuthResult struct {
	User         User
	Session      Session
	SessionToken string
	// TOTPRequired reports that the password verified but a TOTP code is still
	// needed. When true, User carries the resolved identity for auditing while
	// Session and SessionToken stay empty because no session is created.
	TOTPRequired bool
}

// PasskeyCredential represents a stored WebAuthn passkey without private key material.
type PasskeyCredential struct {
	ID                      string
	UserID                  string
	Label                   string
	CredentialID            []byte
	PublicKey               []byte
	SignCount               uint32
	BackupEligible          bool
	BackupState             bool
	Transports              []string
	AAGUID                  []byte
	AttestationType         string
	AttestationFormat       string
	AuthenticatorAttachment string
	Credential              webauthnlib.Credential
	CreatedAt               time.Time
	UpdatedAt               time.Time
	LastUsedAt              *time.Time
}

// PasskeyCeremonyType identifies the passkey ceremony currently in progress.
type PasskeyCeremonyType string

const (
	// PasskeyCeremonyRegistration identifies an authenticated passkey registration ceremony.
	PasskeyCeremonyRegistration PasskeyCeremonyType = "registration"
	// PasskeyCeremonyLogin identifies a discoverable passkey login ceremony.
	PasskeyCeremonyLogin PasskeyCeremonyType = "login"
)

// PasskeyCeremony stores server-side WebAuthn challenge data between begin and finish.
type PasskeyCeremony struct {
	ID        string
	UserID    string
	Type      PasskeyCeremonyType
	Session   webauthnlib.SessionData
	CreatedAt time.Time
	ExpiresAt time.Time
}

// PasskeyListItem contains public passkey metadata returned to the owning user.
type PasskeyListItem struct {
	ID             string     `json:"id"`
	Label          string     `json:"label"`
	Transports     []string   `json:"transports,omitempty"`
	BackupEligible bool       `json:"backupEligible"`
	BackupState    bool       `json:"backupState"`
	SignCount      uint32     `json:"signCount"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	LastUsedAt     *time.Time `json:"lastUsedAt,omitempty"`
}

// PasskeyListRequest contains actor identity and pagination settings for listing passkeys.
type PasskeyListRequest struct {
	Actor    Actor
	Page     int
	PageSize int
}

// PasskeyRegistrationStart contains WebAuthn registration options and ceremony id.
type PasskeyRegistrationStart struct {
	FlowID  string `json:"flowId"`
	Options any    `json:"options"`
}

// PasskeyRegistrationFinishRequest contains actor and ceremony data for passkey registration confirmation.
type PasskeyRegistrationFinishRequest struct {
	Actor  Actor
	FlowID string
	Label  string
}

// PasskeyLoginStart contains WebAuthn discoverable login options and ceremony id.
type PasskeyLoginStart struct {
	FlowID  string `json:"flowId"`
	Options any    `json:"options"`
}

// PasskeyLoginFinishRequest contains ceremony data for completing a passkey login.
type PasskeyLoginFinishRequest struct {
	FlowID string
}

// PasskeyUpdateRequest contains actor and label data for renaming a passkey.
type PasskeyUpdateRequest struct {
	Actor     Actor
	PasskeyID string
	Label     string
}

// EmailCodePurpose identifies the auth flow that owns an email code.
type EmailCodePurpose string

const (
	// EmailCodePurposeVerification identifies a registration email verification code.
	EmailCodePurposeVerification EmailCodePurpose = "email_verification"
	// EmailCodePurposePasswordReset identifies an account recovery password reset code.
	EmailCodePurposePasswordReset EmailCodePurpose = "password_reset"
)

// EmailCodeRecord represents a stored one-time email code without plaintext code material.
type EmailCodeRecord struct {
	Email     string
	Purpose   EmailCodePurpose
	CodeHash  string
	ExpiresAt time.Time
	CreatedAt time.Time
	UsedAt    *time.Time
}

// EmailCodeDelivery contains one-time code data intended only for the email delivery boundary.
type EmailCodeDelivery struct {
	Email     string    `json:"email"`
	Code      string    `json:"-"`
	User      *User     `json:"-"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// EmailCodeRequest contains input for sending an email verification or reset code.
type EmailCodeRequest struct {
	Email string
}

// ConfirmEmailRequest contains input for confirming an email verification code.
type ConfirmEmailRequest struct {
	Email string
	Code  string
}

// ConfirmPasswordResetRequest contains input for confirming a password reset code.
type ConfirmPasswordResetRequest struct {
	Email       string
	Code        string
	NewPassword string
}

// TOTPSetup represents a pending authenticator app setup challenge.
type TOTPSetup struct {
	Secret    string    `json:"-"`
	Otpauth   string    `json:"otpauth"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// TOTPStatus represents whether the authenticated user has TOTP enabled.
type TOTPStatus struct {
	Enabled bool `json:"enabled"`
}

// TOTPConfirmRequest contains input for confirming a pending TOTP setup.
type TOTPConfirmRequest struct {
	Actor   Actor
	Session Session
	Code    string
}

// TOTPSetupRequest contains input for starting a pending TOTP setup.
type TOTPSetupRequest struct {
	Actor   Actor
	Session Session
}

// TOTPDisableRequest contains input for disabling TOTP on the authenticated user.
type TOTPDisableRequest struct {
	Actor Actor
	Code  string
}

// PendingTOTPSetup stores an unconfirmed TOTP secret for a browser session.
type PendingTOTPSetup struct {
	UserID    string
	Secret    string
	Otpauth   string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CookieSettings contains secure browser cookie attributes for session tokens.
type CookieSettings struct {
	Name     string
	Path     string
	MaxAge   int
	Secure   bool
	HTTPOnly bool
	SameSite string
}
