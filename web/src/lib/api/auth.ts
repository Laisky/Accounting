export type AuthUser = {
  id: string;
  email: string;
  status: string;
  emailVerified: boolean;
  totpEnabled: boolean;
  baseCurrency: string;
  createdAt: string;
  updatedAt: string;
};

export type AuthActor = {
  userId: string;
  email: string;
  status: string;
};

export type AuthSession = {
  id: string;
  userId: string;
  userEmail: string;
  status: string;
  expiresAt: string;
  createdAt: string;
};

export type AuthResult = {
  user: AuthUser;
  session: AuthSession;
};

// PasswordLoginResult is either a completed sign-in or a pending TOTP challenge
// returned once the password has been verified for a TOTP-enabled account.
export type PasswordLoginResult =
  | ({ kind: 'authenticated' } & AuthResult)
  | { kind: 'totpRequired' };

export type SessionResult = {
  actor: AuthActor;
  session: AuthSession;
};

export type PasskeyStart = {
  flowId: string;
  options: unknown;
};

export type PasskeyListItem = {
  id: string;
  label: string;
  transports: string[];
  backupEligible: boolean;
  backupState: boolean;
  signCount: number;
  createdAt: string;
  updatedAt: string;
  lastUsedAt?: string;
};

export type PasskeyList = {
  items: PasskeyListItem[];
  page: number;
  pageSize: number;
  total: number;
};

export type TotpStatus = {
  enabled: boolean;
};

export type TotpSetup = {
  otpauth: string;
  expiresAt: string;
};

// fetchUserProfile receives an AbortSignal, loads the current user's public profile, and returns user preferences.
export async function fetchUserProfile(signal?: AbortSignal): Promise<AuthUser> {
  const response = await fetch('/api/users/me', { signal });
  if (!response.ok) {
    throw new Error(`user profile request failed: ${response.status}`);
  }

  const payload = (await response.json()) as { user: AuthUser };
  return payload.user;
}

// updateUserProfile receives mutable profile preferences and returns the updated public user profile.
export async function updateUserProfile(input: { baseCurrency?: string }): Promise<AuthUser> {
  const response = await fetch('/api/users/me', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new Error(`user profile update failed: ${response.status}`);
  }

  const payload = (await response.json()) as { user: AuthUser };
  return payload.user;
}

// fetchSession receives an AbortSignal, loads the active browser session, and returns actor metadata.
export async function fetchSession(signal?: AbortSignal): Promise<SessionResult> {
  const response = await fetch('/api/auth/session', { signal });
  if (!response.ok) {
    throw new Error(`session request failed: ${response.status}`);
  }

  return response.json() as Promise<SessionResult>;
}

// loginWithPassword submits credentials and optional challenge data and returns either authenticated data or a pending TOTP challenge.
export async function loginWithPassword(email: string, password: string, totpCode?: string, turnstileToken?: string): Promise<PasswordLoginResult> {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, totp_code: totpCode || undefined, turnstile_token: turnstileToken || undefined }),
  });
  if (!response.ok) {
    throw new Error(`login failed: ${response.status}`);
  }

  const payload = (await response.json()) as Partial<AuthResult> & { totpRequired?: boolean };
  if (payload.totpRequired) {
    return { kind: 'totpRequired' };
  }
  if (!payload.user || !payload.session) {
    throw new Error('login response missing session');
  }

  return { kind: 'authenticated', user: payload.user, session: payload.session };
}

// registerWithPassword receives email, password, and optional challenge credentials and returns public user data.
export async function registerWithPassword(email: string, password: string, turnstileToken?: string): Promise<{ user: AuthUser }> {
  const response = await fetch('/api/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, turnstile_token: turnstileToken || undefined }),
  });
  if (!response.ok) {
    throw new Error(`registration failed: ${response.status}`);
  }

  return response.json() as Promise<{ user: AuthUser }>;
}

// logout receives no parameters and clears the active browser session.
export async function logout(): Promise<void> {
  const response = await fetch('/api/auth/logout', {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`logout failed: ${response.status}`);
  }
}

// requestEmailVerification receives an email address and requests a non-secret verification code delivery.
export async function requestEmailVerification(email: string): Promise<void> {
  const response = await fetch(`/api/auth/email/verification?email=${encodeURIComponent(email)}`);
  if (!response.ok) {
    throw new Error(`verification request failed: ${response.status}`);
  }
}

// confirmEmailVerification receives an email address and code and returns the activated public user.
export async function confirmEmailVerification(email: string, code: string): Promise<{ user: AuthUser }> {
  const response = await fetch('/api/auth/email/verification', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code }),
  });
  if (!response.ok) {
    throw new Error(`email verification confirmation failed: ${response.status}`);
  }

  return response.json() as Promise<{ user: AuthUser }>;
}

// requestPasswordReset receives an email address and requests a non-secret reset code delivery.
export async function requestPasswordReset(email: string): Promise<void> {
  const response = await fetch('/api/auth/password-reset/request', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  });
  if (!response.ok) {
    throw new Error(`password reset request failed: ${response.status}`);
  }
}

// confirmPasswordReset receives reset code data and updates the account password.
export async function confirmPasswordReset(email: string, code: string, newPassword: string): Promise<{ user: AuthUser }> {
  const response = await fetch('/api/auth/password-reset/confirm', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, newPassword }),
  });
  if (!response.ok) {
    throw new Error(`password reset confirmation failed: ${response.status}`);
  }

  return response.json() as Promise<{ user: AuthUser }>;
}

// beginPasskeyLogin receives no parameters and starts a discoverable passkey login ceremony.
export async function beginPasskeyLogin(): Promise<PasskeyStart> {
  const response = await fetch('/api/auth/passkeys/login/begin', {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`passkey login start failed: ${response.status}`);
  }

  return response.json() as Promise<PasskeyStart>;
}

// finishPasskeyLogin receives a ceremony id and WebAuthn credential response and returns an authenticated session.
export async function finishPasskeyLogin(flowId: string, credential: unknown): Promise<AuthResult> {
  const response = await fetch('/api/auth/passkeys/login/finish', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ flowId, credential }),
  });
  if (!response.ok) {
    throw new Error(`passkey login finish failed: ${response.status}`);
  }

  return response.json() as Promise<AuthResult>;
}

// fetchPasskeys receives an AbortSignal, loads signed-in passkey metadata, and returns a bounded page.
export async function fetchPasskeys(signal?: AbortSignal): Promise<PasskeyList> {
  const response = await fetch('/api/auth/passkeys?page=1&page_size=20', { signal });
  if (!response.ok) {
    throw new Error(`passkey list failed: ${response.status}`);
  }

  return response.json() as Promise<PasskeyList>;
}

// beginPasskeyRegistration receives no parameters and starts an authenticated passkey registration ceremony.
export async function beginPasskeyRegistration(): Promise<PasskeyStart> {
  const response = await fetch('/api/auth/passkeys/register/begin', {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`passkey registration start failed: ${response.status}`);
  }

  return response.json() as Promise<PasskeyStart>;
}

// finishPasskeyRegistration receives a ceremony id, label, and credential response and returns passkey metadata.
export async function finishPasskeyRegistration(flowId: string, label: string, credential: unknown): Promise<PasskeyListItem> {
  const response = await fetch('/api/auth/passkeys/register/finish', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ flowId, label, credential }),
  });
  if (!response.ok) {
    throw new Error(`passkey registration failed: ${response.status}`);
  }

  return response.json() as Promise<PasskeyListItem>;
}

// updatePasskey receives a passkey id and label, updates the metadata, and returns the updated passkey.
export async function updatePasskey(passkeyId: string, label: string): Promise<PasskeyListItem> {
  const response = await fetch(`/api/auth/passkeys/${encodeURIComponent(passkeyId)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ label }),
  });
  if (!response.ok) {
    throw new Error(`passkey update failed: ${response.status}`);
  }

  return response.json() as Promise<PasskeyListItem>;
}

// deletePasskey receives a passkey id and deletes it from the signed-in account.
export async function deletePasskey(passkeyId: string): Promise<void> {
  const response = await fetch(`/api/auth/passkeys/${encodeURIComponent(passkeyId)}`, {
    method: 'DELETE',
  });
  if (!response.ok) {
    throw new Error(`passkey delete failed: ${response.status}`);
  }
}

// fetchTotpStatus receives an AbortSignal, loads the signed-in user's TOTP state, and returns whether it is enabled.
export async function fetchTotpStatus(signal?: AbortSignal): Promise<TotpStatus> {
  const response = await fetch('/api/auth/totp/status', { signal });
  if (!response.ok) {
    throw new Error(`totp status request failed: ${response.status}`);
  }

  return response.json() as Promise<TotpStatus>;
}

// setupTotp receives no parameters, starts a pending TOTP setup, and returns the otpauth URI for enrollment.
export async function setupTotp(): Promise<TotpSetup> {
  const response = await fetch('/api/auth/totp/setup', {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`totp setup failed: ${response.status}`);
  }

  return response.json() as Promise<TotpSetup>;
}

// confirmTotp receives a one-time code, confirms pending setup, and returns the updated TOTP status.
export async function confirmTotp(code: string): Promise<TotpStatus> {
  const response = await fetch('/api/auth/totp/confirm', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  });
  if (!response.ok) {
    throw new Error(`totp confirmation failed: ${response.status}`);
  }

  return response.json() as Promise<TotpStatus>;
}

// disableTotp receives a one-time code, disables TOTP, and returns the updated TOTP status.
export async function disableTotp(code: string): Promise<TotpStatus> {
  const response = await fetch('/api/auth/totp/disable', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  });
  if (!response.ok) {
    throw new Error(`totp disable failed: ${response.status}`);
  }

  return response.json() as Promise<TotpStatus>;
}
