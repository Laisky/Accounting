import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { emptyRuntimeConfig } from './lib/api/runtimeConfig';
import {
  fixtureUser,
  installAppTestFetchMock,
  ledgerResponse,
  openImportFromMe,
  openMeProfile,
  openMeSecurity,
  renderApp,
  response,
} from './test/appTestHarness';

describe('App', () => {
  beforeEach(installAppTestFetchMock);

  it('stages and applies a Wacai import from the import tab', async () => {
    renderApp();

    expect(await openImportFromMe()).toBeInTheDocument();
    expect(screen.getByLabelText('Destination book')).toHaveTextContent('Household');
    fireEvent.change(screen.getByLabelText('New book'), { target: { value: 'Fish pond' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith('/api/books', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: expect.stringContaining('"name":"Fish pond"'),
      }),
    );

    const file = new File(['xlsx-bytes'], 'wacai.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    });
    fireEvent.change(screen.getByLabelText('Upload Wacai export file'), { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: 'Stage import' }));

    expect(await screen.findByText('Import staged')).toBeInTheDocument();
    expect(screen.getByLabelText('Import preview summary')).toHaveTextContent('Rows');
    expect(screen.getByLabelText('Detected import values')).toHaveTextContent('Dining');
    expect(screen.getByLabelText('Import row diagnostics')).toHaveTextContent('Self');
    expect(screen.getByLabelText('Import row diagnostics')).toHaveTextContent('Roommate');
    expect(screen.getByLabelText('Member mappings')).toHaveTextContent('Roommate');
    expect(screen.getByText('Add member mappings before applying.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Apply import' })).toBeDisabled();
    fireEvent.change(screen.getByLabelText('UID or email for Roommate'), {
      target: { value: 'roommate@example.test' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Apply import' }));

    expect(await screen.findByText('Imported 1 rows, skipped 0.')).toBeInTheDocument();
    expect(await screen.findByText('Import applied.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/imports/import-batch-1/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{"sourceHash":"source-hash-1","memberMappings":{"Roommate":"roommate@example.test"}}',
    });
  });

  it('opens the reports tab with existing report drilldowns', async () => {
    renderApp();

    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Reports' }));

    const reports = await screen.findByRole('region', { name: 'Reports' });
    expect(reports).toBeInTheDocument();
    expect(await screen.findByRole('tabpanel', { name: 'Category' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Category expense' })).toBeInTheDocument();
    expect((await screen.findAllByText('Dining')).length).toBeGreaterThan(0);
  });

  it('opens the profile tab and loads audit activity', async () => {
    renderApp();

    expect(await openMeProfile()).toBeInTheDocument();
    expect(screen.getByText('person@example.test')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Back to Me' }));
    expect(await openMeSecurity()).toBeInTheDocument();
    expect(await screen.findByRole('article', { name: 'Authenticator app' })).toHaveTextContent(
      'Authenticator app is off.',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Set up TOTP' }));
    expect(await screen.findByText('TOTP setup started.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Authenticator setup URI')).toHaveValue(
      'otpauth://totp/Accounting:person@example.test?secret=JBSWY3DPEHPK3PXP&issuer=Accounting',
    );
    fireEvent.change(screen.getByLabelText('TOTP code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm TOTP' }));

    expect(await screen.findByText('TOTP enabled.')).toBeInTheDocument();
    expect(screen.getByRole('article', { name: 'Authenticator app' })).toHaveTextContent('Authenticator app is on.');
    expect(fetch).toHaveBeenCalledWith('/api/auth/totp/confirm', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{"code":"123456"}',
    });

    fireEvent.change(screen.getByLabelText('TOTP code'), { target: { value: '654321' } });
    fireEvent.click(screen.getByRole('button', { name: 'Disable TOTP' }));

    expect(await screen.findByText('TOTP disabled.')).toBeInTheDocument();
    expect(screen.getByRole('article', { name: 'Authenticator app' })).toHaveTextContent('Authenticator app is off.');
    expect(fetch).toHaveBeenCalledWith('/api/auth/totp/disable', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{"code":"654321"}',
    });

    fireEvent.click(screen.getByRole('button', { name: 'Back to Me' }));
    expect(await openMeProfile()).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Load activity' }));
    expect(await screen.findByText('entry / created')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/audit?page=1&page_size=20');
  });

  it('shows the account UID on the profile tab and copies it to the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });

    try {
      renderApp();

      expect(await openMeProfile()).toBeInTheDocument();
      expect(screen.getByText('user-1')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Copy account UID' }));
      await waitFor(() => expect(writeText).toHaveBeenCalledWith('user-1'));
      expect(await screen.findByText('Copied')).toBeInTheDocument();
    } finally {
      if (originalClipboard) {
        Object.defineProperty(navigator, 'clipboard', originalClipboard);
      } else {
        delete (navigator as { clipboard?: unknown }).clipboard;
      }
    }
  });

  it('opens the import view from the Me tab', async () => {
    renderApp();

    expect(await openImportFromMe()).toBeInTheDocument();
    // The Me tab stays highlighted while the import sub-view is open.
    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getByRole('button', { name: 'Me' })).toHaveAttribute('aria-current', 'page');
  });

  it('renders the zero-value budget fallback when summary loading fails', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/auth/session') {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              actor: { userId: 'user-1', email: 'person@example.test', status: 'active' },
              session: { id: 'session-1', userId: 'user-1', userEmail: 'person@example.test', status: 'active' },
            }),
        } as Response);
      }
      if (url === '/api/runtime-config') {
        return Promise.resolve({
          ok: false,
          status: 500,
          json: () => Promise.resolve({}),
        } as Response);
      }
      if (url === '/api/users/me') {
        return Promise.resolve(response({ user: fixtureUser }));
      }
      if (url === '/api/ledger/summary') {
        return Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({}) } as Response);
      }
      if (url === '/api/exchange-rates') {
        return Promise.resolve(
          response([{ currency: 'CNY', unitsPerUsd: '7.1', source: 'test', updatedAt: '2026-07-01T00:00:00Z' }]),
        );
      }
      const ledger = ledgerResponse(url, init);
      if (ledger) {
        return Promise.resolve(ledger);
      }

      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as Response);
    });

    renderApp();

    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('region', { name: 'Transactions' })).toHaveTextContent('Lunch'));
  });

  it('shows the public landing page before authentication', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/auth/session') {
        return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({}) } as Response);
      }
      if (url === '/api/runtime-config') {
        return Promise.resolve({ ok: true, json: () => Promise.resolve(emptyRuntimeConfig) } as Response);
      }

      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as Response);
    });

    renderApp();

    expect(await screen.findByRole('heading', { name: 'A ledger for every shared money story.' })).toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { name: 'Enter the ledger with an auditable identity.' }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('link', { name: 'Sign in' }));

    expect(
      await screen.findByRole('heading', { name: 'Enter the ledger with an auditable identity.' }),
    ).toBeInTheDocument();
  });

  it('routes the landing sign-in action to SSO when password login is disabled', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/auth/session') {
        return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({}) } as Response);
      }
      if (url === '/api/runtime-config') {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              ...emptyRuntimeConfig,
              auth: { ...emptyRuntimeConfig.auth, emailLoginEnabled: false },
              features: { ...emptyRuntimeConfig.features, externalSsoEnabled: true },
              sso: { enabled: true, startPath: '/api/auth/sso/start' },
            }),
        } as Response);
      }

      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as Response);
    });

    renderApp();

    expect(await screen.findByRole('heading', { name: 'A ledger for every shared money story.' })).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/api/auth/sso/start'),
    );
  });

  it('signs in from the authentication screen', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/auth/session') {
        return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({}) } as Response);
      }
      if (url === '/api/runtime-config') {
        return Promise.resolve({ ok: true, json: () => Promise.resolve(emptyRuntimeConfig) } as Response);
      }
      if (url === '/api/exchange-rates') {
        return Promise.resolve(
          response([{ currency: 'CNY', unitsPerUsd: '7.1', source: 'test', updatedAt: '2026-07-01T00:00:00Z' }]),
        );
      }
      if (url === '/api/auth/login' && init?.method === 'POST') {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
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
              session: {
                id: 'session-1',
                userId: 'user-1',
                userEmail: 'person@example.test',
                status: 'active',
                createdAt: '2026-07-01T00:00:00Z',
                expiresAt: '2026-07-02T00:00:00Z',
              },
            }),
        } as Response);
      }
      const ledger = ledgerResponse(url, init);
      if (ledger) {
        return Promise.resolve(ledger);
      }

      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ balanceCents: 12500, currency: 'USD', entryCount: 3 }),
      } as Response);
    });

    renderApp('/login');

    expect(
      await screen.findByRole('heading', { name: 'Enter the ledger with an auditable identity.' }),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    expect(await screen.findByRole('region', { name: 'Home' })).toBeInTheDocument();
    expect(await openMeProfile()).toBeInTheDocument();
    expect(screen.getByText('person@example.test')).toBeInTheDocument();
  });
});
