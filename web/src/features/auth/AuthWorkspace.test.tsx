import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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

describe('AuthWorkspace', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders external SSO when runtime config enables it', () => {
    render(
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
    render(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={vi.fn()} />);

    expect(screen.queryByText('External SSO')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Use SSO' })).not.toBeInTheDocument();
  });

  it('reveals the TOTP field only after the password is verified', async () => {
    const loginSpy = vi
      .spyOn(authApi, 'loginWithPassword')
      .mockResolvedValueOnce({ kind: 'totpRequired' })
      .mockResolvedValueOnce(authenticatedResult);
    const onAuthenticated = vi.fn();

    render(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={onAuthenticated} />);

    // The TOTP field is hidden before the password is submitted.
    expect(screen.queryByLabelText('TOTP code')).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    // Password step succeeded with a TOTP challenge; the code field now appears.
    expect(await screen.findByLabelText('TOTP code')).toBeInTheDocument();
    expect(onAuthenticated).not.toHaveBeenCalled();
    expect(loginSpy).toHaveBeenNthCalledWith(1, 'person@example.test', 'correct horse battery staple', undefined);

    fireEvent.change(screen.getByLabelText('TOTP code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify code' }));

    await waitFor(() =>
      expect(onAuthenticated).toHaveBeenCalledWith({ userId: 'user-1', email: 'person@example.test', status: 'active' }),
    );
    expect(loginSpy).toHaveBeenNthCalledWith(2, 'person@example.test', 'correct horse battery staple', '123456');
  });

  it('drops back to the password step when credentials change after a TOTP challenge', async () => {
    const loginSpy = vi.spyOn(authApi, 'loginWithPassword').mockResolvedValue({ kind: 'totpRequired' });

    render(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={vi.fn()} />);

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
    expect(loginSpy).toHaveBeenNthCalledWith(2, 'person@example.test', 'a different password value', undefined);
  });

  it('keeps the user on the TOTP step and shows an error when the code is rejected', async () => {
    const loginSpy = vi
      .spyOn(authApi, 'loginWithPassword')
      .mockResolvedValueOnce({ kind: 'totpRequired' })
      .mockRejectedValueOnce(new Error('login failed: 401'));
    const onAuthenticated = vi.fn();

    render(<AuthWorkspace runtimeConfig={emptyRuntimeConfig} onAuthenticated={onAuthenticated} />);

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
    expect(loginSpy).toHaveBeenNthCalledWith(2, 'person@example.test', 'correct horse battery staple', '000000');
  });
});
