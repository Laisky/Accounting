import type { TFunction } from 'i18next';
import { ApiError } from '@/lib/apiClient';

const apiErrorKeys: Record<string, string> = {
  authentication_required: 'apiError.authenticationRequired',
  invalid_email_or_password: 'apiError.invalidCredentials',
  invalid_import_input: 'apiError.invalidImportInput',
  invalid_ledger_input: 'apiError.invalidLedgerInput',
  invalid_request_body: 'apiError.invalidRequest',
  ledger_access_denied: 'apiError.accessDenied',
  ledger_resource_not_found: 'apiError.notFound',
  rate_limit_exceeded: 'apiError.rateLimited',
};

// apiErrorMessage receives an unknown caught error and returns localized API copy when the server sent a stable code.
export function apiErrorMessage(t: TFunction, error: unknown, fallbackKey: string): string {
  if (!(error instanceof ApiError)) {
    return t(fallbackKey);
  }

  const key = apiErrorKeys[error.code] ?? 'apiError.generic';
  return t(key);
}
