import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { emptyRuntimeConfig } from './lib/api/runtimeConfig';

const fixtureBook = {
  id: 'book-1',
  ownerUserId: 'user-1',
  name: 'Household',
  reportingCurrency: 'USD',
  role: 'owner',
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

const fixtureGroup = {
  id: 'group-1',
  userId: 'user-1',
  name: 'Everyday',
  sortOrder: 0,
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

const fixtureAccount = {
  id: 'account-1',
  userId: 'user-1',
  groupId: 'group-1',
  name: 'Cash',
  type: 'cash',
  currency: 'USD',
  sharedBookIds: ['book-1'],
  openingBalanceCents: 0,
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

const fixtureCategory = {
  id: 'category-1',
  bookId: 'book-1',
  name: 'Dining',
  direction: 'expense',
  sortOrder: 0,
  archived: false,
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

const fixtureEntry = {
  id: 'entry-1',
  bookId: 'book-1',
  creatorUserId: 'user-1',
  type: 'expense',
  accountId: 'account-1',
  categoryId: 'category-1',
  amountCents: 1230,
  transactionCurrency: 'USD',
  accountCurrency: 'USD',
  bookReportingCurrency: 'USD',
  occurredAt: '2026-07-01T00:00:00Z',
  note: 'Lunch',
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

const fixtureCNYEntry = {
  ...fixtureEntry,
  id: 'entry-cny',
  amountCents: 10000,
  transactionCurrency: 'CNY',
  accountCurrency: 'CNY',
  bookReportingCurrency: 'USD',
  exchangeRate: 'CNY/USD=0.14',
  note: 'Converted lunch',
};

const fixtureAuditEvent = {
  id: 'audit-1',
  actorId: 'user-1',
  actorEmail: 'person@example.test',
  action: 'entry.created',
  targetType: 'entry',
  targetId: 'entry-1',
  createdAt: '2026-07-01T00:00:00Z',
};

// ledgerResponse receives a URL and request init and returns a mocked ledger API response when matched.
function ledgerResponse(url: string, init?: RequestInit): Response | null {
  if (url === '/api/books' || url === '/api/books?page=1&page_size=50') {
    return response(init?.method === 'POST' ? fixtureBook : { items: [fixtureBook], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/accounts/groups' || url === '/api/accounts/groups?page=1&page_size=50') {
    return response(init?.method === 'POST' ? fixtureGroup : { items: [fixtureGroup], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/accounts' || url === '/api/accounts?page=1&page_size=50') {
    return response(init?.method === 'POST' ? fixtureAccount : { items: [fixtureAccount], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/books/book-1/categories' || url === '/api/books/book-1/categories?page=1&page_size=50') {
    return response(init?.method === 'POST' ? fixtureCategory : { items: [fixtureCategory], page: 1, pageSize: 50, total: 1 });
  }
  if (url.startsWith('/api/books/book-1/entries')) {
    if (init?.method === 'POST') {
      return response({ ...fixtureEntry, id: 'entry-created', note: 'Team lunch' });
    }
    if (url.includes('page_size=100')) {
      return response({ entries: [fixtureEntry, fixtureCNYEntry], page: 1, pageSize: 100, total: 2 });
    }
    return response({ entries: [fixtureEntry], page: 1, pageSize: 20, total: 1 });
  }

  return null;
}

// response receives JSON data and returns a minimal successful fetch Response.
function response(data: unknown): Response {
  return {
    ok: true,
    json: () => Promise.resolve(data),
  } as Response;
}

describe('App', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string, init?: RequestInit) => {
        if (url === '/api/runtime-config') {
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve({
                serverName: 'test',
                apiBase: '/api',
                auth: {
                  emailLoginEnabled: true,
                  emailRegisterEnabled: false,
                  emailVerificationRequired: true,
                },
                features: {
                  totpEnabled: true,
                  passkeyEnabled: true,
                  turnstileEnabled: true,
                },
                passkey: {
                  enabled: true,
                  rpDisplayName: 'Accounting Test',
                  rpId: 'accounts.example.test',
                  rpOrigin: 'https://accounts.example.test',
                },
                turnstile: {
                  enabled: true,
                  loginMode: 'after_failure',
                  siteKey: 'turnstile-site',
                },
              }),
          });
        }
        if (url === '/api/auth/session') {
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve({
                actor: {
                  userId: 'user-1',
                  email: 'person@example.test',
                  status: 'active',
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
          });
        }
        if (url === '/api/audit?page=1&page_size=8') {
          return Promise.resolve(response({ items: [fixtureAuditEvent], page: 1, pageSize: 8, total: 1 }));
        }
        if (url === '/api/exchange-rates') {
          return Promise.resolve(response([{ currency: 'CNY', unitsPerUsd: '7.1', source: 'test', updatedAt: '2026-07-01T00:00:00Z' }]));
        }
        const ledger = ledgerResponse(url, init);
        if (ledger) {
          return Promise.resolve(ledger);
        }

        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ balanceCents: 12500, currency: 'USD', entryCount: 3 }),
        });
      }),
    );
  });

  it('renders the Wacai-style mobile workspace with four bottom tabs', async () => {
    render(<App />);

    expect(await screen.findByRole('region', { name: 'Monthly spending budget' })).toBeInTheDocument();
    expect(screen.getByText('Remaining')).toBeInTheDocument();
    expect(await screen.findByText('Lunch')).toBeInTheDocument();
    expect(screen.getByText('-$12.30')).toBeInTheDocument();

    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getAllByRole('button')).toHaveLength(4);
    expect(within(nav).getByRole('button', { name: 'Accounts' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Record' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Reports' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Me' })).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/ledger/summary', { signal: expect.any(AbortSignal) });
  });

  it('shows accounts and can prepare starter account data', async () => {
    render(<App />);

    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Accounts' }));

    expect(await screen.findByRole('region', { name: 'Accounts' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Accounts' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Net assets' })).toBeInTheDocument();
    expect(screen.getByText('Credit cards')).toBeInTheDocument();
    expect(screen.getByText('Savings and IOUs')).toBeInTheDocument();
    expect(screen.getByText('cash / USD')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Prepare account' }));
    expect(await screen.findByText('Account ready.')).toBeInTheDocument();
  });

  it('posts a quick ledger entry from the record tab', async () => {
    render(<App />);

    expect(await screen.findByRole('region', { name: 'Record' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Note'), { target: { value: 'Team lunch' } });
    fireEvent.click(screen.getByRole('button', { name: 'Post entry' }));

    expect(await screen.findByText('Entry posted.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"note":"Team lunch"'),
    });
  });

  it('opens the reports tab with existing report drilldowns', async () => {
    render(<App />);

    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Reports' }));

    const reports = await screen.findByRole('region', { name: 'Reports' });
    expect(reports).toBeInTheDocument();
    expect(await screen.findByRole('tabpanel', { name: 'Category' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Category expense' })).toBeInTheDocument();
    expect((await screen.findAllByText('Dining')).length).toBeGreaterThan(0);
  });

  it('opens the profile tab and loads audit activity', async () => {
    render(<App />);

    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Me' }));
    expect(await screen.findByRole('region', { name: 'Me' })).toBeInTheDocument();
    expect(screen.getByText('person@example.test')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Load activity' }));
    expect(await screen.findByText('entry / created')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/audit?page=1&page_size=8');
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
      if (url === '/api/ledger/summary') {
        return Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({}) } as Response);
      }
      const ledger = ledgerResponse(url, init);
      if (ledger) {
        return Promise.resolve(ledger);
      }

      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as Response);
    });

    render(<App />);

    expect(await screen.findByRole('region', { name: 'Monthly spending budget' })).toBeInTheDocument();
    expect(screen.getByText('Total $30,000.00')).toBeInTheDocument();
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
        return Promise.resolve(response([{ currency: 'CNY', unitsPerUsd: '7.1', source: 'test', updatedAt: '2026-07-01T00:00:00Z' }]));
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

    render(<App />);

    expect(await screen.findByRole('heading', { name: 'Enter the ledger with an auditable identity.' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.test' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with email' }));

    expect(await screen.findByRole('region', { name: 'Monthly spending budget' })).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Me' }));
    expect(await screen.findByRole('region', { name: 'Me' })).toBeInTheDocument();
    expect(screen.getByText('person@example.test')).toBeInTheDocument();
  });
});
