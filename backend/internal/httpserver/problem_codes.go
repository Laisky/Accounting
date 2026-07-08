package httpserver

import "net/http"

// ProblemCode is a governed, stable machine-readable error identifier. Each code is bound
// one-to-one to a canonical HTTP status and a stable English title in problemRegistry and
// never changes with message wording; the dynamic text travels in ProblemDetail.detail.
type ProblemCode string

const (
	// CodeInvalidRequestBody indicates a malformed or undecodable request body.
	CodeInvalidRequestBody ProblemCode = "invalid_request_body"
	// CodeValidationFailed is the generic 400 fallback for failed input validation.
	CodeValidationFailed ProblemCode = "validation_failed"
	// CodeInvalidLedgerInput indicates ledger-domain input that failed validation.
	CodeInvalidLedgerInput ProblemCode = "invalid_ledger_input"
	// CodeAuthenticationRequired indicates a missing or invalid session.
	CodeAuthenticationRequired ProblemCode = "authentication_required"
	// CodeInvalidCredentials indicates a failed authentication attempt.
	CodeInvalidCredentials ProblemCode = "invalid_credentials" //nolint:gosec // governed error-code identifier, not a credential
	// CodeTOTPRequired indicates login must be completed with a second factor.
	CodeTOTPRequired ProblemCode = "totp_required"
	// CodeAccessDenied is the generic 403 fallback.
	CodeAccessDenied ProblemCode = "access_denied"
	// CodeLedgerAccessDenied indicates the actor cannot access the target book.
	CodeLedgerAccessDenied ProblemCode = "ledger_access_denied"
	// CodeNotFound is the generic 404 fallback.
	CodeNotFound ProblemCode = "not_found"
	// CodeLedgerNotFound indicates a missing ledger resource.
	CodeLedgerNotFound ProblemCode = "ledger_not_found"
	// CodeConflict indicates an idempotency, uniqueness, or concurrency conflict.
	CodeConflict ProblemCode = "conflict"
	// CodePayloadTooLarge indicates the request body exceeded the size limit.
	CodePayloadTooLarge ProblemCode = "payload_too_large"
	// CodeRateLimited indicates the limiter or account lockout rejected the request.
	CodeRateLimited ProblemCode = "rate_limited"
	// CodeImportFailed indicates an import apply business failure.
	CodeImportFailed ProblemCode = "import_failed"
	// CodeExchangeRatesUnavailable indicates the exchange-rate dependency is unavailable.
	CodeExchangeRatesUnavailable ProblemCode = "exchange_rates_unavailable"
	// CodeInternalError is the generic 5xx fallback.
	CodeInternalError ProblemCode = "internal_error"
)

// problemDefinition binds a governed code to its canonical status and stable title.
type problemDefinition struct {
	status int
	title  string
}

// problemRegistry is the single source of truth for the governed error codes.
var problemRegistry = map[ProblemCode]problemDefinition{
	CodeInvalidRequestBody:       {http.StatusBadRequest, "Invalid request body"},
	CodeValidationFailed:         {http.StatusBadRequest, "Validation failed"},
	CodeInvalidLedgerInput:       {http.StatusBadRequest, "Invalid ledger input"},
	CodeAuthenticationRequired:   {http.StatusUnauthorized, "Authentication required"},
	CodeInvalidCredentials:       {http.StatusUnauthorized, "Invalid credentials"},
	CodeTOTPRequired:             {http.StatusUnauthorized, "Two-factor verification required"},
	CodeAccessDenied:             {http.StatusForbidden, "Access denied"},
	CodeLedgerAccessDenied:       {http.StatusForbidden, "No access to this book"},
	CodeNotFound:                 {http.StatusNotFound, "Resource not found"},
	CodeLedgerNotFound:           {http.StatusNotFound, "Ledger resource not found"},
	CodeConflict:                 {http.StatusConflict, "Resource conflict"},
	CodePayloadTooLarge:          {http.StatusRequestEntityTooLarge, "Request body too large"},
	CodeRateLimited:              {http.StatusTooManyRequests, "Too many requests"},
	CodeImportFailed:             {http.StatusUnprocessableEntity, "Import processing failed"},
	CodeExchangeRatesUnavailable: {http.StatusServiceUnavailable, "Exchange-rate service unavailable"},
	CodeInternalError:            {http.StatusInternalServerError, "Internal server error"},
}

// defaultCodeForStatus maps an HTTP status to the generic governed code for that status class.
func defaultCodeForStatus(status int) ProblemCode {
	switch status {
	case http.StatusBadRequest:
		return CodeValidationFailed
	case http.StatusUnauthorized:
		return CodeAuthenticationRequired
	case http.StatusForbidden:
		return CodeAccessDenied
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusRequestEntityTooLarge:
		return CodePayloadTooLarge
	case http.StatusTooManyRequests:
		return CodeRateLimited
	case http.StatusUnprocessableEntity:
		return CodeImportFailed
	case http.StatusServiceUnavailable:
		return CodeExchangeRatesUnavailable
	}
	if status >= 500 {
		return CodeInternalError
	}
	return CodeValidationFailed
}

// messageCodeIndex refines the status-derived code for specific known messages where the
// generic status default is not specific enough (e.g. a failed login is CodeInvalidCredentials,
// not the generic CodeAuthenticationRequired). Unknown messages fall back to the status default.
var messageCodeIndex = map[string]ProblemCode{
	"invalid request body":              CodeInvalidRequestBody,
	"invalid ledger input":              CodeInvalidLedgerInput,
	"invalid email or password":         CodeInvalidCredentials,
	"passkey login failed":              CodeInvalidCredentials,
	"ledger access denied":              CodeLedgerAccessDenied,
	"book access denied":                CodeLedgerAccessDenied,
	"ledger resource not found":         CodeLedgerNotFound,
	"book summary not found":            CodeLedgerNotFound,
	"request body too large":            CodePayloadTooLarge,
	"rate limit exceeded":               CodeRateLimited,
	"login temporarily locked":          CodeRateLimited,
	"import batch source hash mismatch": CodeConflict,
}

// codeForMessage resolves the governed code for a human message and status, preferring an
// explicit index entry and otherwise falling back to the status default.
func codeForMessage(message string, status int) ProblemCode {
	if code, ok := messageCodeIndex[message]; ok {
		return code
	}
	return defaultCodeForStatus(status)
}
