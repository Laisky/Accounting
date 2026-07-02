import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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

let currentBook = fixtureBook;

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

let currentGroup = fixtureGroup;
let createdAccounts: typeof fixtureAccount[] = [];

const fixtureBookMember = {
  bookId: 'book-1',
  userId: 'user-1',
  role: 'owner',
  displayName: 'Person',
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

let currentCategories: typeof fixtureCategory[] = [fixtureCategory];

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

let currentEntry = fixtureEntry;
let currentTotpEnabled = false;

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

const fixtureImportBatch = {
  id: 'import-batch-1',
  userId: 'user-1',
  source: 'wacai',
  filename: 'wacai.xlsx',
  contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  sourceHash: 'source-hash-1',
  parserVersion: 'wacai-preview-v2',
  status: 'preview',
  detectedSchema: { columns: { occurredAt: '日期时间', type: '类型', amount: '金额' } },
  rows: [
    {
      rowNumber: 8,
      type: 'expense',
      sourceType: '支出',
      occurredAt: '2026-07-01',
      amount: '12.30',
      currency: 'USD',
      account: 'Cash',
      destinationAccount: '',
      category: 'Dining',
      book: 'Household',
      member: 'Self',
      participants: ['Roommate'],
      merchant: 'Market',
      attribute: 'Reviewed',
      note: 'Import lunch',
      tags: ['food', 'work'],
    },
  ],
  detected: {
    books: ['Household'],
    accounts: ['Cash'],
    categories: ['Dining'],
    currencies: ['USD'],
    members: ['Self', 'Roommate'],
    merchants: ['Market'],
    tags: ['food', 'work'],
  },
  errorCount: 0,
  warningCount: 0,
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

// ledgerResponse receives a URL and request init and returns a mocked ledger API response when matched.
function ledgerResponse(url: string, init?: RequestInit): Response | null {
  if (url === '/api/books' || url === '/api/books?page=1&page_size=50') {
    return response(init?.method === 'POST' ? currentBook : { items: [currentBook], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/books/book-1' && init?.method === 'PATCH') {
    const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureBook>;
    currentBook = {
      ...currentBook,
      name: body.name ?? currentBook.name,
      reportingCurrency: body.reportingCurrency ?? currentBook.reportingCurrency,
      updatedAt: '2026-07-01T01:00:00Z',
    };
    return response(currentBook);
  }
  if (url === '/api/books/book-1/members?page=1&page_size=50') {
    return response({ items: [fixtureBookMember], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/accounts/groups' || url === '/api/accounts/groups?page=1&page_size=50') {
    return response(init?.method === 'POST' ? currentGroup : { items: [currentGroup], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/accounts/groups/group-1' && init?.method === 'PATCH') {
    const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureGroup>;
    currentGroup = {
      ...currentGroup,
      name: body.name ?? currentGroup.name,
      sortOrder: body.sortOrder ?? currentGroup.sortOrder,
      updatedAt: '2026-07-01T01:00:00Z',
    };
    return response(currentGroup);
  }
  if (url === '/api/accounts' || url === '/api/accounts?page=1&page_size=50') {
    if (init?.method === 'POST') {
      const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureAccount>;
      const account = {
        ...fixtureAccount,
        id: 'account-created',
        name: body.name ?? fixtureAccount.name,
        type: body.type ?? fixtureAccount.type,
        currency: body.currency ?? fixtureAccount.currency,
        openingBalanceCents: body.openingBalanceCents ?? 0,
      };
      createdAccounts = [account];
      return response(account);
    }

    return response({ items: [fixtureAccount, ...createdAccounts], page: 1, pageSize: 50, total: 1 + createdAccounts.length });
  }
  if (url === '/api/books/book-1/categories' || url === '/api/books/book-1/categories?page=1&page_size=50') {
    if (init?.method === 'POST') {
      const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureCategory>;
      const category = {
        ...fixtureCategory,
        id: 'category-created',
        name: body.name ?? fixtureCategory.name,
        direction: body.direction ?? fixtureCategory.direction,
        archived: false,
      };
      currentCategories = [...currentCategories, category];
      return response(category);
    }

    return response({ items: currentCategories, page: 1, pageSize: 50, total: currentCategories.length });
  }
  if (url.startsWith('/api/books/book-1/categories/') && init?.method === 'PATCH') {
    const categoryId = url.split('/').pop();
    const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureCategory>;
    const category = {
      ...(currentCategories.find((item) => item.id === categoryId) ?? fixtureCategory),
      ...body,
      updatedAt: '2026-07-01T01:00:00Z',
    };
    currentCategories = currentCategories.map((item) => (item.id === category.id ? category : item));
    return response(category);
  }
  if (url === '/api/books/book-1/imports/import-batch-1/apply' && init?.method === 'POST') {
    const importedEntry = {
      ...fixtureEntry,
      id: 'entry-imported',
      note: 'Import lunch',
      merchant: 'Market',
      tags: ['food', 'work'],
    };
    currentEntry = importedEntry;
    return response({
      batchId: 'import-batch-1',
      bookId: 'book-1',
      status: 'applied',
      importedCount: 1,
      skippedCount: 0,
      entries: [importedEntry],
    });
  }
  if (url.startsWith('/api/books/book-1/entries')) {
    if (init?.method === 'POST') {
      return response({ ...fixtureEntry, id: 'entry-created', note: 'Team lunch' });
    }
    if (init?.method === 'PATCH') {
      const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureEntry>;
      currentEntry = { ...currentEntry, ...body, updatedAt: '2026-07-01T01:00:00Z' };
      return response(currentEntry);
    }
    if (init?.method === 'DELETE') {
      return response({ status: 'ok' });
    }
    if (url.includes('page_size=100')) {
      return response({ entries: [currentEntry, fixtureCNYEntry], page: 1, pageSize: 100, total: 2 });
    }
    return response({ entries: [currentEntry], page: 1, pageSize: 20, total: 1 });
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
    currentBook = fixtureBook;
    currentGroup = fixtureGroup;
    createdAccounts = [];
    currentCategories = [fixtureCategory];
    currentEntry = fixtureEntry;
    currentTotpEnabled = false;
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
        if (url === '/api/auth/totp/status') {
          return Promise.resolve(response({ enabled: currentTotpEnabled }));
        }
        if (url === '/api/auth/totp/setup' && init?.method === 'POST') {
          return Promise.resolve(response({ otpauth: 'otpauth://totp/Accounting:person@example.test?secret=JBSWY3DPEHPK3PXP&issuer=Accounting', expiresAt: '2026-07-01T00:10:00Z' }));
        }
        if (url === '/api/auth/totp/confirm' && init?.method === 'POST') {
          currentTotpEnabled = true;
          return Promise.resolve(response({ enabled: currentTotpEnabled }));
        }
        if (url === '/api/auth/totp/disable' && init?.method === 'POST') {
          currentTotpEnabled = false;
          return Promise.resolve(response({ enabled: currentTotpEnabled }));
        }
        if (url === '/api/audit?page=1&page_size=20') {
          return Promise.resolve(response({ items: [fixtureAuditEvent], page: 1, pageSize: 20, total: 1 }));
        }
        if (url === '/api/exchange-rates') {
          return Promise.resolve(response([{ currency: 'CNY', unitsPerUsd: '7.1', source: 'test', updatedAt: '2026-07-01T00:00:00Z' }]));
        }
        if (url === '/api/imports/wacai/preview' && init?.method === 'POST') {
          return Promise.resolve(response(fixtureImportBatch));
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

  it('renders the mobile workspace with five bottom tabs', async () => {
    render(<App />);

    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Income' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Expense' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Transfer' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Borrow/Lend' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('region', { name: 'Selected category' })).toHaveTextContent('Dining'));
    expect(screen.getByRole('group', { name: 'Entry fields' })).toBeInTheDocument();

    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getAllByRole('button')).toHaveLength(5);
    expect(within(nav).getByRole('button', { name: 'Accounts' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Record' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Reports' })).toBeInTheDocument();
    expect(within(nav).getByRole('button', { name: 'Import' })).toBeInTheDocument();
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
    const membersPanel = await screen.findByRole('article', { name: 'Book members' });
    await waitFor(() => expect(membersPanel).toHaveTextContent('Person'));
    expect(membersPanel).toHaveTextContent('owner');
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/members?page=1&page_size=50');

    fireEvent.change(screen.getByLabelText('Book name'), { target: { value: 'Household 2026' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save book' }));

    expect(await screen.findByText('Book updated.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Book name')).toHaveValue('Household 2026');
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Household 2026"'),
    });

    fireEvent.change(screen.getByLabelText('Account group name'), { target: { value: 'Daily Wallets' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save group' }));

    expect(await screen.findByText('Account group updated.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Account group name')).toHaveValue('Daily Wallets');
    expect(fetch).toHaveBeenCalledWith('/api/accounts/groups/group-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Daily Wallets"'),
    });

    fireEvent.click(screen.getByRole('button', { name: 'Prepare account' }));
    expect(await screen.findByText('Account ready.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Account name'), { target: { value: 'Travel wallet' } });
    fireEvent.change(screen.getByLabelText('Opening balance'), { target: { value: '123.45' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

    expect(await screen.findByText('Account created.')).toBeInTheDocument();
    expect(await screen.findByText('Travel wallet')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/accounts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Travel wallet"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/accounts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"openingBalanceCents":12345'),
    });
  });

  it('posts a quick ledger entry from the record tab', async () => {
    render(<App />);

    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('group', { name: 'Category shortcuts' })).toHaveTextContent('Dining'));
    fireEvent.click(screen.getByRole('button', { name: '2' }));
    fireEvent.click(screen.getByRole('button', { name: '4' }));
    fireEvent.change(screen.getByPlaceholderText('Add a note...'), { target: { value: 'Team lunch' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('Entry posted.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"note":"Team lunch"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"type":"expense"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"amountCents":24'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"transactionCurrency":"USD"'),
    });
  });

  it('creates, renames, and archives a category from the record tab', async () => {
    render(<App />);

    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('region', { name: 'Categories' })).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Category name'), { target: { value: 'Fuel' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create category' }));

    expect(await screen.findByText('Category created.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Name for Fuel')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/categories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Fuel"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/categories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"direction":"expense"'),
    });

    const fuelRow = screen.getByLabelText('Name for Fuel').closest('li') as HTMLElement;
    fireEvent.change(within(fuelRow).getByLabelText('Name for Fuel'), { target: { value: 'Road fuel' } });
    fireEvent.click(within(fuelRow).getByRole('button', { name: 'Save category' }));

    expect(await screen.findByText('Category updated.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/categories/category-created', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Road fuel"'),
    });

    const renamedRow = await screen.findByLabelText('Name for Road fuel');
    const row = renamedRow.closest('li') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: 'Archive' }));
    fireEvent.click(within(row).getByRole('button', { name: 'Save category' }));

    expect(await screen.findByText('Category updated.')).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: 'Restore' })).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/categories/category-created', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"archived":true'),
    });
  });

  it('updates and deletes a recent transaction from the record tab', async () => {
    render(<App />);

    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: 'Edit details' }));
    fireEvent.change(await screen.findByLabelText('Amount for Lunch'), { target: { value: '45.67' } });
    fireEvent.change(screen.getByLabelText('Time for Lunch'), { target: { value: '2026-07-01T08:30' } });
    const noteInput = await screen.findByLabelText('Note for Lunch');
    fireEvent.change(noteInput, { target: { value: 'Updated lunch' } });
    fireEvent.change(screen.getByLabelText('Merchant for Lunch'), { target: { value: 'Corner Cafe' } });
    fireEvent.change(screen.getByLabelText('Tags for Lunch'), { target: { value: 'food, work' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save details' }));

    expect(await screen.findByText('Entry updated.')).toBeInTheDocument();
    expect(await screen.findByText('Updated lunch')).toBeInTheDocument();
    expect(screen.getByText('$45.67')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"note":"Updated lunch"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"amountCents":4567'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"occurredAt":"2026-07-01T08:30:00.000Z"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"accountId":"account-1"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"categoryId":"category-1"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"merchant":"Corner Cafe"'),
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"tags":["food","work"]'),
    });

    fireEvent.click(screen.getByRole('button', { name: 'Delete entry' }));
    expect(await screen.findByText('Entry deleted.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries/entry-1', {
      method: 'DELETE',
    });
  });

  it('searches saved transactions from the header search control', async () => {
    render(<App />);

    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('group', { name: 'Category shortcuts' })).toHaveTextContent('Dining'));
    fireEvent.click(screen.getByRole('button', { name: 'Search transactions' }));

    expect(await screen.findByRole('region', { name: 'Search transactions' })).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/entries?page=1&page_size=100');
    fireEvent.change(screen.getByRole('textbox', { name: 'Search transactions' }), { target: { value: 'Converted' } });

    expect(await screen.findByRole('list', { name: 'Search results' })).toHaveTextContent('Converted lunch');
    expect(screen.getByRole('list', { name: 'Search results' })).toHaveTextContent('CN¥100.00');
    fireEvent.click(screen.getByRole('button', { name: 'Close search' }));
    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
  });

  it('stages and applies a Wacai import from the import tab', async () => {
    render(<App />);

    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Import' }));

    expect(await screen.findByRole('region', { name: 'Import data' })).toBeInTheDocument();
    expect(screen.getByLabelText('Destination book')).toHaveTextContent('Household');
    fireEvent.change(screen.getByLabelText('New book'), { target: { value: 'Fish pond' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/books', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: expect.stringContaining('"name":"Fish pond"'),
    }));

    const file = new File(['xlsx-bytes'], 'wacai.xlsx', { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
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
    fireEvent.change(screen.getByLabelText('UID or email for Roommate'), { target: { value: 'roommate@example.test' } });
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
    expect(await screen.findByRole('article', { name: 'Authenticator app' })).toHaveTextContent('Authenticator app is off.');

    fireEvent.click(screen.getByRole('button', { name: 'Set up TOTP' }));
    expect(await screen.findByText('TOTP setup started.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Authenticator setup URI')).toHaveValue('otpauth://totp/Accounting:person@example.test?secret=JBSWY3DPEHPK3PXP&issuer=Accounting');
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

    fireEvent.click(screen.getByRole('button', { name: 'Load activity' }));
    expect(await screen.findByText('entry / created')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/audit?page=1&page_size=20');
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
      if (url === '/api/exchange-rates') {
        return Promise.resolve(response([{ currency: 'CNY', unitsPerUsd: '7.1', source: 'test', updatedAt: '2026-07-01T00:00:00Z' }]));
      }
      const ledger = ledgerResponse(url, init);
      if (ledger) {
        return Promise.resolve(ledger);
      }

      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as Response);
    });

    render(<App />);

    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('region', { name: 'Selected category' })).toHaveTextContent('Dining'));
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

    expect(await screen.findByRole('region', { name: 'Record entry' })).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    fireEvent.click(within(nav).getByRole('button', { name: 'Me' }));
    expect(await screen.findByRole('region', { name: 'Me' })).toBeInTheDocument();
    expect(screen.getByText('person@example.test')).toBeInTheDocument();
  });
});
