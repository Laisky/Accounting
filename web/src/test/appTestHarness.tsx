import { fireEvent, render, screen, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { vi } from 'vitest';
import { App } from '../App';

const fixtureBook = {
  id: 'book-1',
  ownerUserId: 'user-1',
  name: 'Household',
  reportingCurrency: 'USD',
  role: 'owner',
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

export const fixtureUser = {
  id: 'user-1',
  email: 'person@example.test',
  status: 'active',
  emailVerified: true,
  totpEnabled: false,
  baseCurrency: 'USD',
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

const fixtureCreditAccounts = Array.from({ length: 4 }, (_, index) => ({
  ...fixtureAccount,
  id: `credit-account-${index + 1}`,
  name: ['Visa Blue', 'Travel Rewards', 'Family Credit', 'Backup Card'][index],
  type: 'credit_card',
  openingBalanceCents: -1000 * (index + 1),
}));

let currentGroup = fixtureGroup;
let createdAccounts: (typeof fixtureAccount)[] = [];

const fixtureBookMember = {
  bookId: 'book-1',
  userId: 'user-1',
  role: 'owner',
  displayName: 'Person',
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

export const fixtureCategory = {
  id: 'category-1',
  bookId: 'book-1',
  parentId: undefined as string | undefined,
  name: 'Dining',
  direction: 'expense',
  sortOrder: 0,
  archived: false,
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
};

let currentCategories: (typeof fixtureCategory)[] = [fixtureCategory];

export function setCurrentCategories(categories: (typeof fixtureCategory)[]) {
  currentCategories = categories;
}

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
export function ledgerResponse(url: string, init?: RequestInit): Response | null {
  if (url === '/api/v1/books' || url === '/api/v1/books?page=1&page_size=50') {
    return response(init?.method === 'POST' ? currentBook : { items: [currentBook], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/v1/books/book-1' && init?.method === 'PATCH') {
    const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureBook>;
    currentBook = {
      ...currentBook,
      name: body.name ?? currentBook.name,
      reportingCurrency: body.reportingCurrency ?? currentBook.reportingCurrency,
      updatedAt: '2026-07-01T01:00:00Z',
    };
    return response(currentBook);
  }
  if (url === '/api/v1/books/book-1/members?page=1&page_size=50') {
    return response({ items: [fixtureBookMember], page: 1, pageSize: 50, total: 1 });
  }
  if (url === '/api/v1/accounts/groups' || url === '/api/v1/accounts/groups?page=1&page_size=50') {
    return response(
      init?.method === 'POST' ? currentGroup : { items: [currentGroup], page: 1, pageSize: 50, total: 1 },
    );
  }
  if (url === '/api/v1/accounts/groups/group-1' && init?.method === 'PATCH') {
    const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureGroup>;
    currentGroup = {
      ...currentGroup,
      name: body.name ?? currentGroup.name,
      sortOrder: body.sortOrder ?? currentGroup.sortOrder,
      updatedAt: '2026-07-01T01:00:00Z',
    };
    return response(currentGroup);
  }
  if (url === '/api/v1/accounts' || url === '/api/v1/accounts?page=1&page_size=50') {
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

    const accounts = [fixtureAccount, ...fixtureCreditAccounts, ...createdAccounts];
    return response({ items: accounts, page: 1, pageSize: 50, total: accounts.length });
  }
  if (url === '/api/v1/books/book-1/categories' || url === '/api/v1/books/book-1/categories?page=1&page_size=50') {
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
  if (url.startsWith('/api/v1/books/book-1/categories/') && init?.method === 'PATCH') {
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
  if (url === '/api/v1/books/book-1/imports/import-batch-1/apply' && init?.method === 'POST') {
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
  if (url.startsWith('/api/v1/books/book-1/entries')) {
    if (init?.method === 'POST') {
      const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof fixtureEntry>;
      const createdEntry = { ...fixtureEntry, ...body, id: 'entry-created', note: body.note ?? 'Team lunch' };
      currentEntry = createdEntry;
      return response(createdEntry);
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
export function response(data: unknown): Response {
  return {
    ok: true,
    json: () => Promise.resolve(data),
  } as Response;
}

// renderApp mounts App with the router context it requires.
export function renderApp(path = '/') {
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

// openRecordTab clicks the home plus action and waits for the record composer.
export async function openRecordTab() {
  const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
  fireEvent.click(within(nav).getByRole('button', { name: 'Record' }));
  return screen.findByRole('region', { name: 'Record entry' });
}

// openMeIndex clicks the bottom Me navigation item and waits for the compact settings index.
async function openMeIndex() {
  const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
  fireEvent.click(within(nav).getByRole('button', { name: 'Me' }));
  return screen.findByRole('region', { name: 'Me' });
}

// openMeProfile enters the compact Me index and opens the profile subpage.
export async function openMeProfile() {
  await openMeIndex();
  fireEvent.click(await screen.findByRole('button', { name: /Profile/ }));
  return screen.findByRole('region', { name: 'Profile' });
}

// openMeSecurity enters the compact Me index and opens the security subpage.
export async function openMeSecurity() {
  await openMeIndex();
  fireEvent.click(await screen.findByRole('button', { name: /Security/ }));
  return screen.findByRole('region', { name: 'Security' });
}

// openImportFromMe enters the compact Me index and opens the import workflow.
export async function openImportFromMe() {
  await openMeIndex();
  fireEvent.click(await screen.findByRole('button', { name: /Import/ }));
  return screen.findByRole('region', { name: 'Import data' });
}

export function installAppTestFetchMock() {
  currentBook = fixtureBook;
  currentGroup = fixtureGroup;
  createdAccounts = [];
  currentCategories = [fixtureCategory];
  currentEntry = fixtureEntry;
  currentTotpEnabled = false;
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/v1/runtime-config') {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              serverName: 'test',
              apiBase: '/api/v1',
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
      if (url === '/api/v1/auth/session') {
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
      if (url === '/api/v1/users/me') {
        return Promise.resolve(response({ user: fixtureUser }));
      }
      if (url === '/api/v1/auth/totp/status') {
        return Promise.resolve(response({ enabled: currentTotpEnabled }));
      }
      if (url === '/api/v1/auth/totp/setup' && init?.method === 'POST') {
        return Promise.resolve(
          response({
            otpauth: 'otpauth://totp/Accounting:person@example.test?secret=JBSWY3DPEHPK3PXP&issuer=Accounting',
            expiresAt: '2026-07-01T00:10:00Z',
          }),
        );
      }
      if (url === '/api/v1/auth/totp/confirm' && init?.method === 'POST') {
        currentTotpEnabled = true;
        return Promise.resolve(response({ enabled: currentTotpEnabled }));
      }
      if (url === '/api/v1/auth/totp/disable' && init?.method === 'POST') {
        currentTotpEnabled = false;
        return Promise.resolve(response({ enabled: currentTotpEnabled }));
      }
      if (url === '/api/v1/audit?page=1&page_size=20') {
        return Promise.resolve(response({ items: [fixtureAuditEvent], page: 1, pageSize: 20, total: 1 }));
      }
      if (url === '/api/v1/exchange-rates') {
        return Promise.resolve(
          response([{ currency: 'CNY', unitsPerUsd: '7.1', source: 'test', updatedAt: '2026-07-01T00:00:00Z' }]),
        );
      }
      if (url === '/api/v1/imports/wacai/preview' && init?.method === 'POST') {
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
}
