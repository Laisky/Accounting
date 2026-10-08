import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import * as auditApi from '@/lib/api/audit';
import * as authApi from '@/lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from '@/lib/api/runtimeConfig';
import { MeView } from './MeView';

const actor = { userId: 'user-1', email: 'person@example.test', status: 'active' };

const runtimeConfig: RuntimeConfig = {
  ...emptyRuntimeConfig,
  features: { ...emptyRuntimeConfig.features, passkeyEnabled: true, totpEnabled: true },
};

const activityEvents = [
  {
    id: 'audit-1',
    seq: 1,
    hash: 'hash-1',
    action: 'entry.created',
    targetType: 'entry',
    createdAt: '2026-07-01T00:00:00Z',
  },
];

// renderMeView mounts MeView with stubbed security APIs and returns the interaction spies.
function renderMeView(overrides: Partial<Parameters<typeof MeView>[0]> = {}) {
  vi.spyOn(auditApi, 'fetchAuditEvents').mockResolvedValue({
    items: activityEvents,
    page: 1,
    pageSize: 20,
    total: activityEvents.length,
  });
  vi.spyOn(authApi, 'fetchPasskeys').mockResolvedValue({ items: [], page: 1, pageSize: 20, total: 0 });
  vi.spyOn(authApi, 'fetchTotpStatus').mockResolvedValue({ enabled: false });
  vi.spyOn(authApi, 'requestPasswordReset').mockResolvedValue(undefined);
  vi.spyOn(authApi, 'confirmPasswordReset').mockResolvedValue({
    user: {
      id: 'user-1',
      email: 'person@example.test',
      status: 'active',
      emailVerified: true,
      totpEnabled: false,
      baseCurrency: 'USD',
      createdAt: '2026-07-01T00:00:00Z',
      updatedAt: '2026-07-01T00:00:00Z',
    },
  });
  const onBack = vi.fn();
  const onLogout = vi.fn();
  const onOpenImports = vi.fn();
  const onOpenProfile = vi.fn();
  const onOpenSecurity = vi.fn();
  const onUpdateBaseCurrency = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MeView
        actor={actor}
        baseCurrency="USD"
        isLoggingOut={false}
        isProfileSaving={false}
        meSection="index"
        onBack={onBack}
        onLogout={onLogout}
        onOpenImports={onOpenImports}
        onOpenProfile={onOpenProfile}
        onOpenSecurity={onOpenSecurity}
        onUpdateBaseCurrency={onUpdateBaseCurrency}
        runtimeConfig={runtimeConfig}
        {...overrides}
      />
    </QueryClientProvider>,
  );
  return { onBack, onLogout, onOpenImports, onOpenProfile, onOpenSecurity, onUpdateBaseCurrency };
}

describe('MeView', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders a compact Me index with drill-in buttons only', () => {
    renderMeView();

    expect(screen.getByRole('region', { name: 'Me' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Profile/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Security/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Import/ })).toBeInTheDocument();
    expect(screen.queryByText('person@example.test')).not.toBeInTheDocument();
    expect(screen.queryByRole('article', { name: 'Passkeys' })).not.toBeInTheDocument();
  });

  it('renders profile details after entering the profile page', () => {
    renderMeView({ meSection: 'profile' });

    expect(screen.getByRole('region', { name: 'Profile' })).toBeInTheDocument();
    expect(screen.getByText('person@example.test')).toBeInTheDocument();
    expect(screen.getByText('user-1')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Recent activity' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Preferences' })).toBeInTheDocument();
    expect(screen.getByLabelText(/Primary currency/)).toHaveValue('USD');
    expect(screen.getByText('No recent activity loaded yet.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument();
  });

  it('saves the selected profile currency from the profile page', () => {
    const { onUpdateBaseCurrency } = renderMeView({ meSection: 'profile' });

    fireEvent.change(screen.getByLabelText(/Primary currency/), { target: { value: 'EUR' } });

    expect(onUpdateBaseCurrency).toHaveBeenCalledWith('EUR');
  });

  it('renders password, passkey, and authenticator settings after entering security', async () => {
    renderMeView({ meSection: 'security' });

    expect(screen.getByRole('region', { name: 'Security' })).toBeInTheDocument();
    expect(screen.getByRole('article', { name: 'Password' })).toBeInTheDocument();
    expect(await screen.findByRole('article', { name: 'Passkeys' })).toBeInTheDocument();
    expect(screen.getByRole('article', { name: 'Authenticator app' })).toBeInTheDocument();
    expect(screen.getByText('Recommended')).toBeInTheDocument();
  });

  it('copies the account UID and confirms with an accessible status', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    const original = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });

    try {
      renderMeView({ meSection: 'profile' });
      fireEvent.click(screen.getByRole('button', { name: 'Copy account UID' }));
      await waitFor(() => expect(writeText).toHaveBeenCalledWith('user-1'));
      expect(await screen.findByText('Copied')).toBeInTheDocument();
    } finally {
      if (original) {
        Object.defineProperty(navigator, 'clipboard', original);
      } else {
        delete (navigator as { clipboard?: unknown }).clipboard;
      }
    }
  });

  it('wires index navigation actions', () => {
    const { onOpenImports, onOpenProfile, onOpenSecurity } = renderMeView();

    fireEvent.click(screen.getByRole('button', { name: /Profile/ }));
    expect(onOpenProfile).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole('button', { name: /Security/ }));
    expect(onOpenSecurity).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole('button', { name: /Import/ }));
    expect(onOpenImports).toHaveBeenCalledOnce();
  });

  it('wires profile back, activity, and sign-out actions', async () => {
    const auditSpy = vi.spyOn(auditApi, 'fetchAuditEvents');
    const { onBack, onLogout } = renderMeView({ meSection: 'profile' });

    fireEvent.click(screen.getByRole('button', { name: 'Back to Me' }));
    expect(onBack).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole('button', { name: 'Load activity' }));
    await waitFor(() => expect(auditSpy).toHaveBeenCalledOnce());
    expect(await screen.findByText('entry / created')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    expect(onLogout).toHaveBeenCalledOnce();
  });

  it('shows an inline activity load failure from the audit query', async () => {
    renderMeView({ meSection: 'profile' });
    vi.spyOn(auditApi, 'fetchAuditEvents').mockRejectedValue(new Error('network down'));

    fireEvent.click(screen.getByRole('button', { name: 'Load activity' }));

    expect(await screen.findByText('Activity could not be loaded.')).toBeInTheDocument();
  });

  it('requests and confirms a password reset from the security page', async () => {
    const requestSpy = vi.spyOn(authApi, 'requestPasswordReset');
    const confirmSpy = vi.spyOn(authApi, 'confirmPasswordReset');
    renderMeView({ meSection: 'security' });

    fireEvent.click(screen.getByRole('button', { name: 'Send reset email' }));
    await waitFor(() => expect(requestSpy).toHaveBeenCalledWith('person@example.test'));
    expect(await screen.findByText('Password reset email requested.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Reset code'), { target: { value: '123456' } });
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'new correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Reset password' }));

    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith('person@example.test', '123456', 'new correct horse battery staple'),
    );
    expect(await screen.findByText('Password updated. Sign in with the new password.')).toBeInTheDocument();
  });

  it('notes email-only sign-in when no stronger factors are available', () => {
    renderMeView({
      meSection: 'security',
      runtimeConfig: {
        ...emptyRuntimeConfig,
        features: { ...emptyRuntimeConfig.features, passkeyEnabled: false, totpEnabled: false },
      },
    });

    expect(screen.getByText('You sign in with your email and password.')).toBeInTheDocument();
    expect(screen.queryByRole('article', { name: 'Passkeys' })).not.toBeInTheDocument();
  });
});
