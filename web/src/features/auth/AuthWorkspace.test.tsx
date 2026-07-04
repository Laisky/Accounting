import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { type ReactNode } from 'react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import * as authApi from '../../lib/api/auth';
import { AuthWorkspace } from './AuthWorkspace';
import { emptyRuntimeConfig } from '../../lib/api/runtimeConfig';

const authenticatedResult = {
  kind: 'authenticated' as const,
  user: {
    id: 'user-1',
    email: 'person@example.test',
    status: 'active',
    emailVerified: true,
    totpEnabled: true,
    baseCurrency: 'USD',
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  },
  session: {
    id: 'session-1',
    userId: 'user-1',
    userEmail: 'person@example.test',
    status: 'active',
    expiresAt: '2026-07-02T00:00:00Z',
    createdAt: '2026-07-01T00:00:00Z',
  },
};

// renderWithRouter mounts auth screens with the router context required by AuthWorkspace.
function renderWithRouter(children: ReactNode, path = '/login') {
  return render(<MemoryRouter initialEntries={[path]}>{children}</MemoryRouter>);
}

describe('AuthWorkspace', () => {
  const originalCredentials = navigator.credentials;

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    Object.defineProperty(navigator, 'credentials', {
      configurable: true,
      value: originalCredentials,
    });
  });

  it('renders external SSO when runtime config enables it', () => {
    renderWithRouter(
      <AuthWorkspace
        runtimeConfig={{
          ...emptyRuntimeConfig,
          features: {
            ...emptyRuntimeConfig.features,
            externalSsoEnabled: true,
          },
          sso: {
            enabled: true,
            startPath: '/api/auth/sso/start',
          },
        }}
        onAuthenticated={vi.fn()}
      />,
    );

    expect(screen.getByText('External SSO')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Use SSO' })).toHaveAttribute('href', '/api/auth/sso/start');
  });

  it('hides external SSO when runtime config disables it', () => {
    renderWithRouter(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={vi.fn()} />);

    expect(screen.queryByText('External SSO')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Use SSO' })).not.toBeInTheDocument();
  });

  it('finishes passkey sign-in through the browser credential API', async () => {
    const getCredential = vi.fn().mockResolvedValue(assertionCredential());
    Object.defineProperty(navigator, 'credentials', {
      configurable: true,
      value: { get: getCredential },
    });
    vi.stubGlobal('PublicKeyCredential', function PublicKeyCredential() {});
    const beginSpy = vi.spyOn(authApi, 'beginPasskeyLogin').mockResolvedValue({
      flowId: 'flow-1',
      options: {
        publicKey: {
          challenge: 'AQID',
          userVerification: 'required',
        },
      },
    });
    const finishSpy = vi.spyOn(authApi, 'finishPasskeyLogin').mockResolvedValue(authenticatedResult);
    const onAuthenticated = vi.fn();

    renderWithRouter(
      <AuthWorkspace
        runtimeConfig={{
          ...emptyRuntimeConfig,
          features: { ...emptyRuntimeConfig.features, passkeyEnabled: true },
        }}
        onAuthenticated={onAuthenticated}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Use passkey' }));

    await waitFor(() =>
      expect(onAuthenticated).toHaveBeenCalledWith({ userId: 'user-1', email: 'person@example.test', status: 'active' }),
    );
    expect(beginSpy).toHaveBeenCalledOnce();
    expect(getCredential).toHaveBeenCalledWith({
      publicKey: {
        challenge: expect.any(ArrayBuffer),
        userVerification: 'required',
      },
    });
    expect(finishSpy).toHaveBeenCalledWith('flow-1', {
      id: 'credential-1',
      rawId: 'CQ',
      type: 'public-key',
      authenticatorAttachment: 'platform',
      response: {
        authenticatorData: 'Ag',
        clientDataJSON: 'AQ',
        signature: 'Aw',
        userHandle: 'BA',
      },
      clientExtensionResults: {},
    });
  });

  it('passes a Turnstile token when registration is protected', async () => {
    stubTurnstile('turnstile-register-token');
    const registerSpy = vi.spyOn(authApi, 'registerWithPassword').mockResolvedValue({
      user: authenticatedResult.user,
    });

    renderWithRouter(
      <AuthWorkspace
        runtimeConfig={{
          ...emptyRuntimeConfig,
          auth: { ...emptyRuntimeConfig.auth, emailVerificationRequired: false },
          features: { ...emptyRuntimeConfig.features, turnstileEnabled: true },
          turnstile: { enabled: true, loginMode: 'always', siteKey: 'turnstile-site' },
        }}
        onAuthenticated={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Register' }));
    await screen.findByLabelText('Turnstile challenge');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create account' })).toBeEnabled());
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

    await waitFor(() =>
      expect(registerSpy).toHaveBeenCalledWith('person@example.test', 'correct horse battery staple', 'turnstile-register-token'),
    );
  });

  it('asks for Turnstile after a failed login when configured for after-failure mode', async () => {
    stubTurnstile('turnstile-login-token');
    const loginSpy = vi
      .spyOn(authApi, 'loginWithPassword')
      .mockRejectedValueOnce(new Error('login failed: 401'))
      .mockResolvedValueOnce(authenticatedResult);
    const onAuthenticated = vi.fn();

    renderWithRouter(
      <AuthWorkspace
        runtimeConfig={{
          ...emptyRuntimeConfig,
          features: { ...emptyRuntimeConfig.features, turnstileEnabled: true },
          turnstile: { enabled: true, loginMode: 'after_failure', siteKey: 'turnstile-site' },
        }}
        onAuthenticated={onAuthenticated}
      />,
    );

    expect(screen.queryByLabelText('Turnstile challenge')).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrong password value' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    expect(await screen.findByText('Sign-in failed.')).toBeInTheDocument();
    expect(loginSpy).toHaveBeenNthCalledWith(1, 'person@example.test', 'wrong password value', undefined, undefined);
    await screen.findByLabelText('Turnstile challenge');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Sign in with email' })).toBeEnabled());
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    await waitFor(() =>
      expect(onAuthenticated).toHaveBeenCalledWith({ userId: 'user-1', email: 'person@example.test', status: 'active' }),
    );
    expect(loginSpy).toHaveBeenNthCalledWith(2, 'person@example.test', 'correct horse battery staple', undefined, 'turnstile-login-token');
  });

  it('confirms email verification after registration when verification is required', async () => {
    const registerSpy = vi.spyOn(authApi, 'registerWithPassword').mockResolvedValue({
      user: {
        ...authenticatedResult.user,
        status: 'pending_verification',
        emailVerified: false,
      },
    });
    const requestVerificationSpy = vi.spyOn(authApi, 'requestEmailVerification').mockResolvedValue();
    const confirmVerificationSpy = vi.spyOn(authApi, 'confirmEmailVerification').mockResolvedValue({
      user: authenticatedResult.user,
    });

    renderWithRouter(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Register' }));
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

    expect(await screen.findByText('Verification email requested.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Verification code')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Verify email' })).toBeInTheDocument();
    expect(registerSpy).toHaveBeenCalledWith('person@example.test', 'correct horse battery staple', undefined);
    expect(requestVerificationSpy).toHaveBeenCalledWith('person@example.test');

    fireEvent.change(screen.getByLabelText('Verification code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify email' }));

    expect(await screen.findByText('Email verified. Sign in to continue.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(screen.getByRole('button', { name: 'Sign in with email' })).toBeInTheDocument();
    expect(confirmVerificationSpy).toHaveBeenCalledWith('person@example.test', '123456');
  });

  it('reveals the TOTP field only after the password is verified', async () => {
    const loginSpy = vi
      .spyOn(authApi, 'loginWithPassword')
      .mockResolvedValueOnce({ kind: 'totpRequired' })
      .mockResolvedValueOnce(authenticatedResult);
    const onAuthenticated = vi.fn();

    renderWithRouter(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={onAuthenticated} />);

    // The TOTP field is hidden before the password is submitted.
    expect(screen.queryByLabelText('TOTP code')).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    // Password step succeeded with a TOTP challenge; the code field now appears.
    expect(await screen.findByLabelText('TOTP code')).toBeInTheDocument();
    expect(onAuthenticated).not.toHaveBeenCalled();
    expect(loginSpy).toHaveBeenNthCalledWith(1, 'person@example.test', 'correct horse battery staple', undefined, undefined);

    fireEvent.change(screen.getByLabelText('TOTP code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify code' }));

    await waitFor(() =>
      expect(onAuthenticated).toHaveBeenCalledWith({ userId: 'user-1', email: 'person@example.test', status: 'active' }),
    );
    expect(loginSpy).toHaveBeenNthCalledWith(2, 'person@example.test', 'correct horse battery staple', '123456', undefined);
  });

  it('drops back to the password step when credentials change after a TOTP challenge', async () => {
    const loginSpy = vi.spyOn(authApi, 'loginWithPassword').mockResolvedValue({ kind: 'totpRequired' });

    renderWithRouter(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={vi.fn()} />);

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    fireEvent.change(await screen.findByLabelText('TOTP code'), { target: { value: '123456' } });

    // Editing the password invalidates the pending challenge and hides the code field again.
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'a different password value' } });
    expect(screen.queryByLabelText('TOTP code')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign in with email' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));
    await waitFor(() => expect(loginSpy).toHaveBeenCalledTimes(2));
    // The re-submitted password step carries no stale TOTP code.
    expect(loginSpy).toHaveBeenNthCalledWith(2, 'person@example.test', 'a different password value', undefined, undefined);
  });

  it('keeps the user on the TOTP step and shows an error when the code is rejected', async () => {
    const loginSpy = vi
      .spyOn(authApi, 'loginWithPassword')
      .mockResolvedValueOnce({ kind: 'totpRequired' })
      .mockRejectedValueOnce(new Error('login failed: 401'));
    const onAuthenticated = vi.fn();

    renderWithRouter(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={onAuthenticated} />);

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    fireEvent.change(await screen.findByLabelText('TOTP code'), { target: { value: '000000' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify code' }));

    // The invalid code surfaces a generic error but leaves the user on the TOTP step.
    expect(await screen.findByText('Sign-in failed.')).toBeInTheDocument();
    expect(onAuthenticated).not.toHaveBeenCalled();
    expect(screen.getByLabelText('TOTP code')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Verify code' })).toBeInTheDocument();
    expect(loginSpy).toHaveBeenNthCalledWith(2, 'person@example.test', 'correct horse battery staple', '000000', undefined);
  });
});

// stubTurnstile receives a token and installs a deterministic Turnstile browser API for tests.
function stubTurnstile(token: string): void {
  vi.stubGlobal('turnstile', {
    render: (_container: HTMLElement, options: { callback: (value: string) => void }) => {
      queueMicrotask(() => options.callback(token));
      return `widget-${token}`;
    },
    remove: vi.fn(),
    reset: vi.fn(),
  });
}

// assertionCredential receives no parameters and returns a minimal WebAuthn assertion credential for tests.
function assertionCredential(): PublicKeyCredential {
  return {
    id: 'credential-1',
    rawId: Uint8Array.from([9]).buffer,
    type: 'public-key',
    authenticatorAttachment: 'platform',
    response: {
      clientDataJSON: Uint8Array.from([1]).buffer,
      authenticatorData: Uint8Array.from([2]).buffer,
      signature: Uint8Array.from([3]).buffer,
      userHandle: Uint8Array.from([4]).buffer,
    },
    getClientExtensionResults: () => ({}),
  } as unknown as PublicKeyCredential;
}
