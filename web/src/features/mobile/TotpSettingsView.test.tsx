import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import * as authApi from '@/lib/api/auth';
import { TotpSettingsView } from './TotpSettingsView';

describe('TotpSettingsView', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('starts setup, confirms TOTP, and disables it through the status cache', async () => {
    vi.spyOn(authApi, 'fetchTotpStatus').mockResolvedValue({ enabled: false });
    const setupSpy = vi.spyOn(authApi, 'setupTotp').mockResolvedValue({
      expiresAt: '2026-07-01T00:05:00Z',
      otpauth: 'otpauth://totp/Accounting:person@example.test?secret=ABC123&issuer=Accounting',
    });
    const confirmSpy = vi.spyOn(authApi, 'confirmTotp').mockResolvedValue({ enabled: true });
    const disableSpy = vi.spyOn(authApi, 'disableTotp').mockResolvedValue({ enabled: false });

    renderTotpSettings();

    expect(await screen.findByText('Authenticator app is off.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Set up TOTP' }));
    await waitFor(() => expect(setupSpy).toHaveBeenCalledOnce());
    expect(await screen.findByText('TOTP setup started.')).toBeInTheDocument();
    expect(screen.getByLabelText('Authenticator setup URI')).toHaveValue(
      'otpauth://totp/Accounting:person@example.test?secret=ABC123&issuer=Accounting',
    );

    fireEvent.change(screen.getByLabelText('TOTP code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm TOTP' }));

    await waitFor(() => expect(confirmSpy).toHaveBeenCalledWith('123456'));
    expect(await screen.findByText('TOTP enabled.')).toBeInTheDocument();
    expect(screen.getByText('Authenticator app is on.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('TOTP code'), { target: { value: '654321' } });
    fireEvent.click(screen.getByRole('button', { name: 'Disable TOTP' }));

    await waitFor(() => expect(disableSpy).toHaveBeenCalledWith('654321'));
    expect(await screen.findByText('TOTP disabled.')).toBeInTheDocument();
    expect(screen.getByText('Authenticator app is off.')).toBeInTheDocument();
  });

  it('shows an inline error when TOTP status fails to load', async () => {
    vi.spyOn(authApi, 'fetchTotpStatus').mockRejectedValue(new Error('network down'));

    renderTotpSettings();

    expect(await screen.findByText('TOTP status could not be loaded.')).toBeInTheDocument();
  });
});

// renderTotpSettings mounts TotpSettingsView with an isolated QueryClient.
function renderTotpSettings() {
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <TotpSettingsView featureEnabled />
    </QueryClientProvider>,
  );
}
