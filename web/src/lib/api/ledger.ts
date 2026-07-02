export type LedgerSummary = {
  bookId?: string;
  bookName?: string;
  balanceCents: number;
  currency: string;
  entryCount: number;
};

export type BookListItem = {
  id: string;
  ownerUserId: string;
  name: string;
  reportingCurrency: string;
  role: string;
  createdAt: string;
  updatedAt: string;
};

export type AccountGroup = {
  id: string;
  userId: string;
  name: string;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};

export type Account = {
  id: string;
  userId: string;
  groupId: string;
  name: string;
  type: string;
  currency: string;
  sharedBookIds?: string[];
  openingBalanceCents: number;
  createdAt: string;
  updatedAt: string;
};

export type Category = {
  id: string;
  bookId: string;
  parentId?: string;
  name: string;
  direction: string;
  sortOrder: number;
  archived: boolean;
  rawSourceName?: string;
  createdAt: string;
  updatedAt: string;
};

export type Entry = {
  id: string;
  bookId: string;
  creatorUserId: string;
  type: string;
  accountId?: string;
  categoryId?: string;
  amountCents: number;
  transactionCurrency: string;
  accountCurrency: string;
  bookReportingCurrency: string;
  exchangeRate?: string;
  occurredAt: string;
  note?: string;
  merchant?: string;
  tags?: string[];
  createdAt: string;
  updatedAt: string;
};

export type ExchangeRate = {
  currency: string;
  unitsPerUsd: string;
  source: string;
  updatedAt: string;
};

export type EntryList = {
  entries: Entry[];
  page: number;
  pageSize: number;
  total: number;
};

type PaginatedList<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
};

export const emptyLedgerSummary: LedgerSummary = {
  balanceCents: 0,
  currency: 'USD',
  entryCount: 0,
};

// fetchLedgerSummary receives an AbortSignal, loads the ledger summary API, and returns parsed summary data.
export async function fetchLedgerSummary(signal: AbortSignal): Promise<LedgerSummary> {
  const response = await fetch('/api/ledger/summary', { signal });
  if (!response.ok) {
    throw new Error(`summary request failed: ${response.status}`);
  }

  return response.json() as Promise<LedgerSummary>;
}

// fetchBooks receives no parameters, loads visible books, and returns role-aware book metadata.
export async function fetchBooks(): Promise<BookListItem[]> {
  const response = await fetch('/api/books?page=1&page_size=50');
  if (!response.ok) {
    throw new Error(`books request failed: ${response.status}`);
  }

  const list = await response.json() as PaginatedList<BookListItem>;
  return list.items;
}

// fetchExchangeRates receives no parameters and returns supported USD-relative exchange rates.
export async function fetchExchangeRates(): Promise<ExchangeRate[]> {
  const response = await fetch('/api/exchange-rates');
  if (!response.ok) {
    throw new Error(`exchange rates request failed: ${response.status}`);
  }

  return response.json() as Promise<ExchangeRate[]>;
}

// createBook receives book settings and returns the created role-aware book.
export async function createBook(name: string, reportingCurrency: string): Promise<BookListItem> {
  const response = await fetch('/api/books', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, reportingCurrency }),
  });
  if (!response.ok) {
    throw new Error(`book create failed: ${response.status}`);
  }

  return response.json() as Promise<BookListItem>;
}

// updateBook receives book settings and returns the updated role-aware book.
export async function updateBook(bookId: string, input: { name?: string; reportingCurrency?: string }): Promise<BookListItem> {
  const response = await fetch(`/api/books/${encodeURIComponent(bookId)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new Error(`book update failed: ${response.status}`);
  }

  return response.json() as Promise<BookListItem>;
}

// fetchAccountGroups receives no parameters and returns account groups owned by the actor.
export async function fetchAccountGroups(): Promise<AccountGroup[]> {
  const response = await fetch('/api/accounts/groups?page=1&page_size=50');
  if (!response.ok) {
    throw new Error(`account groups request failed: ${response.status}`);
  }

  const list = await response.json() as PaginatedList<AccountGroup>;
  return list.items;
}

// createAccountGroup receives a name and returns the created account group.
export async function createAccountGroup(name: string): Promise<AccountGroup> {
  const response = await fetch('/api/accounts/groups', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, sortOrder: 0 }),
  });
  if (!response.ok) {
    throw new Error(`account group create failed: ${response.status}`);
  }

  return response.json() as Promise<AccountGroup>;
}

// fetchAccounts receives no parameters and returns accounts owned by the actor.
export async function fetchAccounts(): Promise<Account[]> {
  const response = await fetch('/api/accounts?page=1&page_size=50');
  if (!response.ok) {
    throw new Error(`accounts request failed: ${response.status}`);
  }

  const list = await response.json() as PaginatedList<Account>;
  return list.items;
}

// createAccount receives account settings and returns the created personal account.
export async function createAccount(input: {
  groupId: string;
  name: string;
  type: string;
  currency: string;
  sharedBookIds: string[];
}): Promise<Account> {
  const response = await fetch('/api/accounts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...input, openingBalanceCents: 0 }),
  });
  if (!response.ok) {
    throw new Error(`account create failed: ${response.status}`);
  }

  return response.json() as Promise<Account>;
}

// fetchCategories receives a book id and returns categories visible in that book.
export async function fetchCategories(bookId: string): Promise<Category[]> {
  const response = await fetch(`/api/books/${encodeURIComponent(bookId)}/categories?page=1&page_size=50`);
  if (!response.ok) {
    throw new Error(`categories request failed: ${response.status}`);
  }

  const list = await response.json() as PaginatedList<Category>;
  return list.items;
}

// createCategory receives category settings and returns the created category.
export async function createCategory(bookId: string, name: string, direction: string): Promise<Category> {
  const response = await fetch(`/api/books/${encodeURIComponent(bookId)}/categories`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, direction, sortOrder: 0 }),
  });
  if (!response.ok) {
    throw new Error(`category create failed: ${response.status}`);
  }

  return response.json() as Promise<Category>;
}

// fetchEntries receives a book id and returns the first page of entries for transaction review.
export async function fetchEntries(bookId: string): Promise<EntryList> {
  const response = await fetch(`/api/books/${encodeURIComponent(bookId)}/entries?page=1&page_size=20`);
  if (!response.ok) {
    throw new Error(`entries request failed: ${response.status}`);
  }

  return response.json() as Promise<EntryList>;
}

// fetchAllEntries receives a book id and loads every entry page for complete reporting.
export async function fetchAllEntries(bookId: string): Promise<Entry[]> {
  const pageSize = 100;
  const entries: Entry[] = [];
  let page = 1;
  let total = 0;

  do {
    const response = await fetch(`/api/books/${encodeURIComponent(bookId)}/entries?page=${page}&page_size=${pageSize}`);
    if (!response.ok) {
      throw new Error(`entries request failed: ${response.status}`);
    }

    const list = await response.json() as EntryList;
    entries.push(...list.entries);
    total = list.total;
    page += 1;
  } while (entries.length < total);

  return entries;
}

// createEntry receives entry details and returns the stored book entry.
export async function createEntry(bookId: string, input: {
  type: string;
  accountId: string;
  categoryId?: string;
  amountCents: number;
  transactionCurrency: string;
  occurredAt: string;
  note?: string;
}): Promise<Entry> {
  const response = await fetch(`/api/books/${encodeURIComponent(bookId)}/entries`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new Error(`entry create failed: ${response.status}`);
  }

  return response.json() as Promise<Entry>;
}
