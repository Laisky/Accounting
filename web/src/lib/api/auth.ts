import { apiRequest } from '@/lib/apiClient';
import type { components } from '@/lib/api/generated/schema';

type Schemas = components['schemas'];

export type AuthUser = Schemas['AuthUser'];
export type AuthActor = Schemas['AuthActor'];
export type AuthSession = Schemas['AuthSession'];
export type AuthResult = Schemas['AuthResult'] & { user: AuthUser; session: AuthSession };
export type PasswordLoginResult = ({ kind: 'authenticated' } & AuthResult) | { kind: 'totpRequired' };
export type SessionResult = Schemas['SessionResult'];
export type PasskeyStart = Schemas['PasskeyStart'];
export type PasskeyListItem = Schemas['PasskeyListItem'];
export type PasskeyList = Schemas['PasskeyList'];
export type TotpStatus = Schemas['TotpStatus'];
export type TotpSetup = Schemas['TotpSetup'];

// fetchUserProfile receives an AbortSignal, loads the current user's public profile, and returns user preferences.
export async function fetchUserProfile(signal?: AbortSignal): Promise<AuthUser> {
  const payload = await apiRequest<Schemas['UserEnvelope']>('/api/v1/users/me', { signal });
  return payload.user;
}

// updateUserProfile receives mutable profile preferences and returns the updated public user profile.
export async function updateUserProfile(input: { baseCurrency?: string }): Promise<AuthUser> {
  const payload = await apiRequest<Schemas['UserEnvelope']>('/api/v1/users/me', { method: 'PATCH', body: input });
  return payload.user;
}

// fetchSession receives an AbortSignal, loads the active browser session, and returns actor metadata.
export async function fetchSession(signal?: AbortSignal): Promise<SessionResult> {
  return apiRequest<SessionResult>('/api/v1/auth/session', { signal });
}

// loginWithPassword submits credentials and optional challenge data and returns either authenticated data or a pending TOTP challenge.
export async function loginWithPassword(
  email: string,
  password: string,
  totpCode?: string,
  turnstileToken?: string,
): Promise<PasswordLoginResult> {
  const payload = await apiRequest<Schemas['AuthResult']>('/api/v1/auth/login', {
    method: 'POST',
    body: {
      email,
      password,
      totp_code: totpCode || undefined,
      turnstile_token: turnstileToken || undefined,
    },
  });
  if (payload.totpRequired) {
    return { kind: 'totpRequired' };
  }
  if (!payload.user || !payload.session) {
    throw new Error('login response missing session');
  }

  return { kind: 'authenticated', user: payload.user, session: payload.session };
}

// registerWithPassword receives email, password, and optional challenge credentials and returns public user data.
export async function registerWithPassword(
  email: string,
  password: string,
  turnstileToken?: string,
): Promise<Schemas['UserEnvelope']> {
  return apiRequest<Schemas['UserEnvelope']>('/api/v1/auth/register', {
    method: 'POST',
    body: { email, password, turnstile_token: turnstileToken || undefined },
  });
}

// logout receives no parameters and clears the active browser session.
export async function logout(): Promise<void> {
  await apiRequest<void>('/api/v1/auth/logout', { method: 'POST' });
}

// requestEmailVerification receives an email address and requests a non-secret verification code delivery.
export async function requestEmailVerification(email: string): Promise<void> {
  await apiRequest<Schemas['AcceptedResponse']>(`/api/v1/auth/email/verification?email=${encodeURIComponent(email)}`);
}

// confirmEmailVerification receives an email address and code and returns the activated public user.
export async function confirmEmailVerification(email: string, code: string): Promise<Schemas['UserEnvelope']> {
  return apiRequest<Schemas['UserEnvelope']>('/api/v1/auth/email/verification', {
    method: 'POST',
    body: { email, code },
  });
}

// requestPasswordReset receives an email address and requests a non-secret reset code delivery.
export async function requestPasswordReset(email: string): Promise<void> {
  await apiRequest<Schemas['AcceptedResponse']>('/api/v1/auth/password-reset/request', {
    method: 'POST',
    body: { email },
  });
}

// confirmPasswordReset receives reset code data and updates the account password.
export async function confirmPasswordReset(
  email: string,
  code: string,
  newPassword: string,
): Promise<Schemas['UserEnvelope']> {
  return apiRequest<Schemas['UserEnvelope']>('/api/v1/auth/password-reset/confirm', {
    method: 'POST',
    body: { email, code, newPassword },
  });
}

// beginPasskeyLogin receives no parameters and starts a discoverable passkey login ceremony.
export async function beginPasskeyLogin(): Promise<PasskeyStart> {
  return apiRequest<PasskeyStart>('/api/v1/auth/passkeys/login/begin', { method: 'POST' });
}

// finishPasskeyLogin receives a ceremony id and WebAuthn credential response and returns an authenticated session.
export async function finishPasskeyLogin(flowId: string, credential: unknown): Promise<AuthResult> {
  return apiRequest<AuthResult>('/api/v1/auth/passkeys/login/finish', { method: 'POST', body: { flowId, credential } });
}

// fetchPasskeys receives an AbortSignal, loads signed-in passkey metadata, and returns a bounded page.
export async function fetchPasskeys(signal?: AbortSignal): Promise<PasskeyList> {
  return apiRequest<PasskeyList>('/api/v1/auth/passkeys?page=1&page_size=20', { signal });
}

// beginPasskeyRegistration receives no parameters and starts an authenticated passkey registration ceremony.
export async function beginPasskeyRegistration(): Promise<PasskeyStart> {
  return apiRequest<PasskeyStart>('/api/v1/auth/passkeys/register/begin', { method: 'POST' });
}

// finishPasskeyRegistration receives a ceremony id, label, and credential response and returns passkey metadata.
export async function finishPasskeyRegistration(
  flowId: string,
  label: string,
  credential: unknown,
): Promise<PasskeyListItem> {
  return apiRequest<PasskeyListItem>('/api/v1/auth/passkeys/register/finish', {
    method: 'POST',
    body: { flowId, label, credential },
  });
}

// updatePasskey receives a passkey id and label, updates the metadata, and returns the updated passkey.
export async function updatePasskey(passkeyId: string, label: string): Promise<PasskeyListItem> {
  return apiRequest<PasskeyListItem>(`/api/v1/auth/passkeys/${encodeURIComponent(passkeyId)}`, {
    method: 'PUT',
    body: { label },
  });
}

// deletePasskey receives a passkey id and deletes it from the signed-in account.
export async function deletePasskey(passkeyId: string): Promise<void> {
  await apiRequest<void>(`/api/v1/auth/passkeys/${encodeURIComponent(passkeyId)}`, { method: 'DELETE' });
}

// fetchTotpStatus receives an AbortSignal, loads the signed-in user's TOTP state, and returns whether it is enabled.
export async function fetchTotpStatus(signal?: AbortSignal): Promise<TotpStatus> {
  return apiRequest<TotpStatus>('/api/v1/auth/totp/status', { signal });
}

// setupTotp receives no parameters, starts a pending TOTP setup, and returns the otpauth URI for enrollment.
export async function setupTotp(): Promise<TotpSetup> {
  return apiRequest<TotpSetup>('/api/v1/auth/totp/setup', { method: 'POST' });
}

// confirmTotp receives a one-time code, confirms pending setup, and returns the updated TOTP status.
export async function confirmTotp(code: string): Promise<TotpStatus> {
  return apiRequest<TotpStatus>('/api/v1/auth/totp/confirm', { method: 'POST', body: { code } });
}

// disableTotp receives a one-time code, disables TOTP, and returns the updated TOTP status.
export async function disableTotp(code: string): Promise<TotpStatus> {
  return apiRequest<TotpStatus>('/api/v1/auth/totp/disable', { method: 'POST', body: { code } });
}
