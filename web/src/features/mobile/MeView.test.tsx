import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import * as authApi from '../../lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { MeView } from './MeView';

const actor = { userId: 'user-1', email: 'person@example.test', status: 'active' };

const runtimeConfig: RuntimeConfig = {
  ...emptyRuntimeConfig,
  features: { ...emptyRuntimeConfig.features, passkeyEnabled: true, totpEnabled: true },
};

const activityEvents = [
  {
    id: 'audit-1',
    action: 'entry.created',
    targetType: 'entry',
    createdAt: '2026-07-01T00:00:00Z',
  },
];

// renderMeView mounts MeView with stubbed security APIs and returns the interaction spies.
function renderMeView(overrides: Partial<Parameters<typeof MeView>[0]> = {}) {
  vi.spyOn(authApi, 'fetchPasskeys').mockResolvedValue({ items: [], page: 1, pageSize: 20, total: 0 });
  vi.spyOn(authApi, 'fetchTotpStatus').mockResolvedValue({ enabled: false });
  const onLoadActivity = vi.fn();
  const onLogout = vi.fn();
  const onOpenImports = vi.fn();
  render(
    <MeView
      actor={actor}
      activityEvents={activityEvents}
      isActivityLoading={false}
      isLoggingOut={false}
      onLoadActivity={onLoadActivity}
      onLogout={onLogout}
      onOpenImports={onOpenImports}
      runtimeConfig={runtimeConfig}
      {...overrides}
    />,
  );
  return { onLoadActivity, onLogout, onOpenImports };
}

describe('MeView', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders a grouped identity + security + preferences + data index', async () => {
    renderMeView();

    // Identity banner exposes the account region, email, UID and status.
    expect(screen.getByRole('region', { name: 'Me' })).toBeInTheDocument();
    expect(screen.getByText('person@example.test')).toBeInTheDocument();
    expect(screen.getByText('user-1')).toBeInTheDocument();

    // Grouped section headers give the tab a scannable structure.
    expect(screen.getByRole('heading', { name: 'Security' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Recent activity' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Preferences' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Data' })).toBeInTheDocument();

    // Security panels render with passkeys framed as recommended.
    expect(await screen.findByRole('article', { name: 'Passkeys' })).toBeInTheDocument();
    expect(screen.getByRole('article', { name: 'Authenticator app' })).toBeInTheDocument();
    expect(screen.getByText('Recommended')).toBeInTheDocument();

    // The recent-activity list preserves the "domain / action" audit format.
    expect(screen.getByText('entry / created')).toBeInTheDocument();
  });

  it('copies the account UID and confirms with an accessible status', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    const original = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });

    try {
      renderMeView();
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

  it('wires the import, activity, and sign-out actions', () => {
    const { onLoadActivity, onLogout, onOpenImports } = renderMeView();

    fireEvent.click(screen.getByRole('button', { name: 'Import' }));
    expect(onOpenImports).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole('button', { name: 'Load activity' }));
    expect(onLoadActivity).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    expect(onLogout).toHaveBeenCalledOnce();
  });

  it('notes email-only sign-in when no stronger factors are available', () => {
    renderMeView({
      runtimeConfig: { ...emptyRuntimeConfig, features: { ...emptyRuntimeConfig.features, passkeyEnabled: false, totpEnabled: false } },
    });

    expect(screen.getByText('You sign in with your email and password.')).toBeInTheDocument();
    expect(screen.queryByRole('article', { name: 'Passkeys' })).not.toBeInTheDocument();
  });
});
