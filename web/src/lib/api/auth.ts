export type AuthUser = {
  id: string;
  email: string;
  status: string;
  emailVerified: boolean;
  totpEnabled: boolean;
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
  options: {
    publicKey: {
      rpId?: string;
      userVerification?: string;
    };
  };
};

// fetchSession receives an AbortSignal, loads the active browser session, and returns actor metadata.
export async function fetchSession(signal?: AbortSignal): Promise<SessionResult> {
  const response = await fetch('/api/auth/session', { signal });
  if (!response.ok) {
    throw new Error(`session request failed: ${response.status}`);
  }

  return response.json() as Promise<SessionResult>;
}

// loginWithPassword submits credentials and returns either authenticated data or a pending TOTP challenge.
export async function loginWithPassword(email: string, password: string, totpCode?: string): Promise<PasswordLoginResult> {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, totp_code: totpCode || undefined }),
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

// registerWithPassword receives email and password credentials and returns public user data.
export async function registerWithPassword(email: string, password: string): Promise<{ user: AuthUser }> {
  const response = await fetch('/api/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
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
