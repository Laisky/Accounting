// Package audit owns user-visible security and data-change audit events.
package audit

import (
	"time"

	"github.com/Laisky/errors/v2"
)

// ErrInvalidInput is returned when audit input is malformed.
var ErrInvalidInput = errors.New("invalid audit input")

// Action identifies the kind of user-visible event that occurred.
type Action string

const (
	// ActionAuthRegister records a successful registration.
	ActionAuthRegister Action = "auth.register"
	// ActionAuthLogin records a successful login.
	ActionAuthLogin Action = "auth.login"
	// ActionAuthLoginFailed records a failed login attempt.
	ActionAuthLoginFailed Action = "auth.login_failed"
	// ActionAuthLoginTOTPChallenge records a password that verified but still awaits a TOTP code.
	ActionAuthLoginTOTPChallenge Action = "auth.login_totp_challenge"
	// ActionAuthLogout records a logout.
	ActionAuthLogout Action = "auth.logout"
	// ActionEmailVerificationRequested records a verification code request.
	ActionEmailVerificationRequested Action = "auth.email_verification_requested"
	// ActionEmailVerified records a successful email verification.
	ActionEmailVerified Action = "auth.email_verified"
	// ActionPasswordResetRequested records a password reset code request.
	ActionPasswordResetRequested Action = "auth.password_reset_requested"
	// ActionPasswordResetConfirmed records a successful password reset.
	ActionPasswordResetConfirmed Action = "auth.password_reset_confirmed"
	// ActionTOTPSetupRequested records a TOTP setup start.
	ActionTOTPSetupRequested Action = "auth.totp_setup_requested"
	// ActionTOTPEnabled records a successful TOTP confirmation.
	ActionTOTPEnabled Action = "auth.totp_enabled"
	// ActionTOTPDisabled records a successful TOTP disable.
	ActionTOTPDisabled Action = "auth.totp_disabled"
	// ActionPasskeyRegistrationStarted records a passkey registration ceremony start.
	ActionPasskeyRegistrationStarted Action = "auth.passkey_registration_started"
	// ActionPasskeyRegistered records a successful passkey registration.
	ActionPasskeyRegistered Action = "auth.passkey_registered"
	// ActionPasskeyLogin records a successful passkey login.
	ActionPasskeyLogin Action = "auth.passkey_login"
	// ActionPasskeyRenamed records a passkey label update.
	ActionPasskeyRenamed Action = "auth.passkey_renamed"
	// ActionPasskeyDeleted records a passkey deletion.
	ActionPasskeyDeleted Action = "auth.passkey_deleted"
	// ActionBookCreated records a book creation.
	ActionBookCreated Action = "book.created"
	// ActionBookUpdated records a book settings update.
	ActionBookUpdated Action = "book.updated"
	// ActionAccountGroupCreated records a personal account group creation.
	ActionAccountGroupCreated Action = "account_group.created"
	// ActionAccountGroupUpdated records a personal account group update.
	ActionAccountGroupUpdated Action = "account_group.updated"
	// ActionAccountCreated records a personal account creation.
	ActionAccountCreated Action = "account.created"
	// ActionCategoryCreated records a book category creation.
	ActionCategoryCreated Action = "category.created"
	// ActionCategoryUpdated records a book category update.
	ActionCategoryUpdated Action = "category.updated"
	// ActionEntryCreated records a book entry creation.
	ActionEntryCreated Action = "entry.created"
	// ActionEntryUpdated records a book entry update.
	ActionEntryUpdated Action = "entry.updated"
	// ActionEntryDeleted records a book entry deletion.
	ActionEntryDeleted Action = "entry.deleted"
	// ActionImportPreviewCreated records an import preview batch creation or reuse.
	ActionImportPreviewCreated Action = "import.preview_created"
	// ActionImportCommitted records a staged import batch being committed into ledger entries.
	ActionImportCommitted Action = "import.committed"
)

// Event represents a sanitized audit event visible to the affected user.
type Event struct {
	ID         string            `json:"id"`
	ActorID    string            `json:"actorId,omitempty"`
	ActorEmail string            `json:"actorEmail,omitempty"`
	Action     Action            `json:"action"`
	TargetType string            `json:"targetType"`
	TargetID   string            `json:"targetId,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
}

// RecordRequest contains input for creating an audit event.
type RecordRequest struct {
	ActorID    string
	ActorEmail string
	Action     Action
	TargetType string
	TargetID   string
	Metadata   map[string]string
}

// ListRequest contains filters for reading audit events.
type ListRequest struct {
	ActorID  string
	Page     int
	PageSize int
}

// ListResult contains paginated audit events.
type ListResult struct {
	Items    []Event `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Total    int     `json:"total"`
}
