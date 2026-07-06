import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import * as authApi from '@/lib/api/auth';
import { PasskeySettingsView } from './PasskeySettingsView';

const existingPasskey = {
  id: 'passkey-1',
  label: 'Security key',
  transports: ['internal'],
  backupEligible: false,
  backupState: false,
  signCount: 1,
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

type RegistrationOptionsJSON = {
  publicKey: {
    rp: PublicKeyCredentialRpEntity;
    user: Omit<PublicKeyCredentialUserEntity, 'id'> & { id: string };
    challenge: string;
    pubKeyCredParams: PublicKeyCredentialParameters[];
    authenticatorSelection: AuthenticatorSelectionCriteria;
  };
};

describe('PasskeySettingsView', () => {
  const originalCredentials = navigator.credentials;

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    Object.defineProperty(navigator, 'credentials', {
      configurable: true,
      value: originalCredentials,
    });
  });

  it('registers, renames, and deletes passkeys', async () => {
    const createCredential = vi.fn().mockResolvedValue(attestationCredential());
    Object.defineProperty(navigator, 'credentials', {
      configurable: true,
      value: { create: createCredential },
    });
    vi.stubGlobal('PublicKeyCredential', function PublicKeyCredential() {});
    const fetchSpy = vi.spyOn(authApi, 'fetchPasskeys').mockResolvedValue({
      items: [existingPasskey],
      page: 1,
      pageSize: 20,
      total: 1,
    });
    const beginSpy = vi.spyOn(authApi, 'beginPasskeyRegistration').mockResolvedValue({
      flowId: 'register-flow-1',
      options: registrationOptions(),
    });
    const finishSpy = vi.spyOn(authApi, 'finishPasskeyRegistration').mockResolvedValue({
      ...existingPasskey,
      id: 'passkey-2',
      label: 'Laptop',
      createdAt: '2026-07-02T00:00:00Z',
      updatedAt: '2026-07-02T00:00:00Z',
    });
    const updateSpy = vi.spyOn(authApi, 'updatePasskey').mockResolvedValue({
      ...existingPasskey,
      label: 'Renamed security key',
      updatedAt: '2026-07-02T01:00:00Z',
    });
    const deleteSpy = vi.spyOn(authApi, 'deletePasskey').mockResolvedValue();

    renderPasskeySettings();

    expect(await screen.findByLabelText('Label for Security key')).toBeInTheDocument();
    expect(fetchSpy).toHaveBeenCalledOnce();

    fireEvent.change(screen.getByLabelText('Passkey label'), { target: { value: 'Laptop' } });
    fireEvent.click(screen.getByRole('button', { name: 'Register passkey' }));

    expect(await screen.findByText('Passkey registered.')).toBeInTheDocument();
    expect(beginSpy).toHaveBeenCalledOnce();
    expect(createCredential).toHaveBeenCalledWith({
      publicKey: {
        ...registrationOptions().publicKey,
        challenge: expect.any(ArrayBuffer),
        user: {
          ...registrationOptions().publicKey.user,
          id: expect.any(ArrayBuffer),
        },
      },
    });
    expect(finishSpy).toHaveBeenCalledWith('register-flow-1', 'Laptop', {
      id: 'credential-1',
      rawId: 'CQ',
      type: 'public-key',
      authenticatorAttachment: 'platform',
      response: {
        attestationObject: 'BQ',
        clientDataJSON: 'AQ',
        transports: ['internal'],
      },
      clientExtensionResults: {},
    });

    const existingRow = screen.getByLabelText('Label for Security key').closest('li');
    expect(existingRow).not.toBeNull();
    fireEvent.change(screen.getByLabelText('Label for Security key'), { target: { value: 'Renamed security key' } });
    fireEvent.click(within(existingRow as HTMLElement).getByRole('button', { name: 'Rename' }));

    await waitFor(() => expect(screen.getByText('Passkey renamed.')).toBeInTheDocument());
    expect(updateSpy).toHaveBeenCalledWith('passkey-1', 'Renamed security key');

    const renamedRow = screen.getByLabelText('Label for Renamed security key').closest('li');
    expect(renamedRow).not.toBeNull();
    fireEvent.click(within(renamedRow as HTMLElement).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(screen.getByText('Passkey deleted.')).toBeInTheDocument());
    expect(deleteSpy).toHaveBeenCalledWith('passkey-1');
  });

  it('shows an inline error when passkey metadata fails to load', async () => {
    vi.spyOn(authApi, 'fetchPasskeys').mockRejectedValue(new Error('network down'));

    renderPasskeySettings();

    expect(await screen.findByText('Passkeys could not be loaded.')).toBeInTheDocument();
  });
});

// renderPasskeySettings mounts PasskeySettingsView with an isolated QueryClient.
function renderPasskeySettings() {
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <PasskeySettingsView featureEnabled />
    </QueryClientProvider>,
  );
}

// registrationOptions receives no parameters and returns minimal WebAuthn creation options for tests.
function registrationOptions(): RegistrationOptionsJSON {
  return {
    publicKey: {
      rp: { name: 'Accounting Test', id: 'localhost' },
      user: { id: 'BA', name: 'person@example.test', displayName: 'person@example.test' },
      challenge: 'AQID',
      pubKeyCredParams: [{ type: 'public-key', alg: -7 }],
      authenticatorSelection: {
        residentKey: 'required',
        requireResidentKey: true,
        userVerification: 'required',
      },
    },
  };
}

// attestationCredential receives no parameters and returns a minimal WebAuthn attestation credential for tests.
function attestationCredential(): PublicKeyCredential {
  return {
    id: 'credential-1',
    rawId: Uint8Array.from([9]).buffer,
    type: 'public-key',
    authenticatorAttachment: 'platform',
    response: {
      clientDataJSON: Uint8Array.from([1]).buffer,
      attestationObject: Uint8Array.from([5]).buffer,
      getTransports: () => ['internal'],
    },
    getClientExtensionResults: () => ({}),
  } as unknown as PublicKeyCredential;
}
