import { apiRequest } from '@/lib/apiClient';
import type { components } from '@/lib/api/generated/schema';

type Schemas = components['schemas'];

export type LedgerSummary = Schemas['LedgerSummary'];
export type BookListItem = Schemas['BookListItem'];
export type BookMember = Schemas['BookMember'];
export type AccountGroup = Schemas['AccountGroup'];
export type Account = Schemas['Account'];
export type Category = Schemas['Category'];
export type Entry = Schemas['Entry'];
export type ExchangeRate = Schemas['ExchangeRate'];
export type EntryList = Schemas['EntryList'];
export type EntryUpdateInput = Schemas['EntryUpdateRequest'];
export type CategoryCreateInput = Schemas['CategoryCreateRequest'];
export type CategoryUpdateInput = Schemas['CategoryUpdateRequest'];
export type AccountGroupUpdateInput = Schemas['AccountGroupUpdateRequest'];

export const emptyLedgerSummary: LedgerSummary = {
  balanceCents: 0,
  currency: 'USD',
  entryCount: 0,
};

// fetchLedgerSummary receives an AbortSignal, loads the ledger summary API, and returns parsed summary data.
export async function fetchLedgerSummary(signal: AbortSignal): Promise<LedgerSummary> {
  return apiRequest<LedgerSummary>('/api/v1/ledger/summary', { signal });
}

// fetchBooks receives no parameters, loads visible books, and returns role-aware book metadata.
export async function fetchBooks(): Promise<BookListItem[]> {
  const list = await apiRequest<Schemas['BookList']>('/api/v1/books?page=1&page_size=50');
  return list.items;
}

// fetchExchangeRates receives no parameters and returns supported USD-relative exchange rates.
export async function fetchExchangeRates(): Promise<ExchangeRate[]> {
  return apiRequest<ExchangeRate[]>('/api/v1/exchange-rates');
}

// createBook receives book settings and returns the created role-aware book.
export async function createBook(name: string, reportingCurrency: string): Promise<BookListItem> {
  return apiRequest<BookListItem>('/api/v1/books', { method: 'POST', body: { name, reportingCurrency } });
}

// updateBook receives book settings and returns the updated role-aware book.
export async function updateBook(
  bookId: string,
  input: { name?: string; reportingCurrency?: string },
): Promise<BookListItem> {
  return apiRequest<BookListItem>(`/api/v1/books/${encodeURIComponent(bookId)}`, { method: 'PATCH', body: input });
}

// fetchBookMembers receives a book id and returns explicit members for that book.
export async function fetchBookMembers(bookId: string): Promise<BookMember[]> {
  const list = await apiRequest<Schemas['BookMemberList']>(
    `/api/v1/books/${encodeURIComponent(bookId)}/members?page=1&page_size=50`,
  );
  return list.items;
}

// fetchAccountGroups receives no parameters and returns account groups owned by the actor.
export async function fetchAccountGroups(): Promise<AccountGroup[]> {
  const list = await apiRequest<Schemas['AccountGroupList']>('/api/v1/accounts/groups?page=1&page_size=50');
  return list.items;
}

// createAccountGroup receives a name and returns the created account group.
export async function createAccountGroup(name: string): Promise<AccountGroup> {
  return apiRequest<AccountGroup>('/api/v1/accounts/groups', { method: 'POST', body: { name, sortOrder: 0 } });
}

// updateAccountGroup receives group identity and patch fields, then returns the updated account group.
export async function updateAccountGroup(groupId: string, input: AccountGroupUpdateInput): Promise<AccountGroup> {
  return apiRequest<AccountGroup>(`/api/v1/accounts/groups/${encodeURIComponent(groupId)}`, {
    method: 'PATCH',
    body: input,
  });
}

// fetchAccounts receives no parameters and returns accounts owned by the actor.
export async function fetchAccounts(): Promise<Account[]> {
  const list = await apiRequest<Schemas['AccountList']>('/api/v1/accounts?page=1&page_size=50');
  return list.items;
}

// createAccount receives account settings and returns the created personal account.
export async function createAccount(input: Schemas['AccountCreateRequest']): Promise<Account> {
  return apiRequest<Account>('/api/v1/accounts', {
    method: 'POST',
    body: { ...input, openingBalanceCents: input.openingBalanceCents ?? 0 },
  });
}

// fetchCategories receives a book id and returns categories visible in that book.
export async function fetchCategories(bookId: string): Promise<Category[]> {
  const list = await apiRequest<Schemas['CategoryList']>(
    `/api/v1/books/${encodeURIComponent(bookId)}/categories?page=1&page_size=50`,
  );
  return list.items;
}

// createCategory receives category settings and returns the created category.
export async function createCategory(bookId: string, input: CategoryCreateInput): Promise<Category> {
  return apiRequest<Category>(`/api/v1/books/${encodeURIComponent(bookId)}/categories`, {
    method: 'POST',
    body: { ...input, sortOrder: input.sortOrder ?? 0 },
  });
}

// updateCategory receives category patch fields and returns the updated category.
export async function updateCategory(
  bookId: string,
  categoryId: string,
  input: CategoryUpdateInput,
): Promise<Category> {
  return apiRequest<Category>(
    `/api/v1/books/${encodeURIComponent(bookId)}/categories/${encodeURIComponent(categoryId)}`,
    {
      method: 'PATCH',
      body: input,
    },
  );
}

// fetchEntries receives a book id and returns the first page of entries for transaction review.
export async function fetchEntries(bookId: string): Promise<EntryList> {
  return apiRequest<EntryList>(`/api/v1/books/${encodeURIComponent(bookId)}/entries?page=1&page_size=20`);
}

// fetchAllEntries receives a book id and loads every entry page for complete reporting.
export async function fetchAllEntries(bookId: string): Promise<Entry[]> {
  const pageSize = 100;
  const entries: Entry[] = [];
  let page = 1;
  let total: number;

  do {
    const list = await apiRequest<EntryList>(
      `/api/v1/books/${encodeURIComponent(bookId)}/entries?page=${page}&page_size=${pageSize}`,
    );
    entries.push(...list.entries);
    total = list.total;
    page += 1;
  } while (entries.length < total);

  return entries;
}

// createEntry receives entry details and returns the stored book entry.
export async function createEntry(bookId: string, input: Schemas['EntryCreateRequest']): Promise<Entry> {
  return apiRequest<Entry>(`/api/v1/books/${encodeURIComponent(bookId)}/entries`, { method: 'POST', body: input });
}

// updateEntry receives entry patch fields and returns the updated book entry.
export async function updateEntry(bookId: string, entryId: string, input: EntryUpdateInput): Promise<Entry> {
  return apiRequest<Entry>(`/api/v1/books/${encodeURIComponent(bookId)}/entries/${encodeURIComponent(entryId)}`, {
    method: 'PATCH',
    body: input,
  });
}

// deleteEntry receives entry identity and removes the matching book entry.
export async function deleteEntry(bookId: string, entryId: string): Promise<void> {
  await apiRequest<void>(`/api/v1/books/${encodeURIComponent(bookId)}/entries/${encodeURIComponent(entryId)}`, {
    method: 'DELETE',
  });
}
