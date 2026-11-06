import type { TFunction } from 'i18next';
import { ApiError } from '@/lib/apiClient';

// Keyed by the backend's governed ProblemDetail `code` enum (RFC 9457). Codes without a
// dedicated entry (e.g. totp_required, conflict, payload_too_large, exchange_rates_unavailable,
// internal_error) fall through to the generic copy.
const apiErrorKeys: Record<string, string> = {
  authentication_required: 'apiError.authenticationRequired',
  invalid_credentials: 'apiError.invalidCredentials',
  invalid_ledger_input: 'apiError.invalidLedgerInput',
  invalid_request_body: 'apiError.invalidRequest',
  validation_failed: 'apiError.invalidRequest',
  access_denied: 'apiError.accessDenied',
  ledger_access_denied: 'apiError.accessDenied',
  not_found: 'apiError.notFound',
  ledger_not_found: 'apiError.notFound',
  rate_limited: 'apiError.rateLimited',
  import_failed: 'apiError.invalidImportInput',
};

// apiErrorMessage receives an unknown caught error and returns localized API copy when the server sent a stable code.
export function apiErrorMessage(t: TFunction, error: unknown, fallbackKey: string): string {
  if (!(error instanceof ApiError)) {
    return t(fallbackKey);
  }

  const key = apiErrorKeys[error.code] ?? 'apiError.generic';
  return t(key);
}
