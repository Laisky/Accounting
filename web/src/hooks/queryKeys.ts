export type PageQuery = {
  page?: number;
  pageSize?: number;
};

export type EntryQuery = PageQuery & {
  accountId?: string;
  query?: string;
};

// queryKeys defines stable TanStack Query keys for server-owned Accounting data.
export const queryKeys = {
  accounts: {
    groups: (page: PageQuery = {}) => ['accounts', 'groups', page] as const,
    list: (page: PageQuery = {}) => ['accounts', 'list', page] as const,
  },
  audit: {
    list: (page: PageQuery = {}) => ['audit', 'list', page] as const,
  },
  auth: {
    passkeys: (page: PageQuery = {}) => ['auth', 'passkeys', page] as const,
    session: () => ['auth', 'session'] as const,
  },
  books: {
    detail: (bookId: string) => ['books', 'detail', bookId] as const,
    list: (page: PageQuery = {}) => ['books', 'list', page] as const,
    members: (bookId: string, page: PageQuery = {}) => ['books', bookId, 'members', page] as const,
  },
  categories: {
    list: (bookId: string, page: PageQuery = {}) => ['books', bookId, 'categories', page] as const,
  },
  entries: {
    detail: (bookId: string, entryId: string) => ['books', bookId, 'entries', entryId] as const,
    list: (bookId: string, filters: EntryQuery = {}) => ['books', bookId, 'entries', filters] as const,
  },
  exchangeRates: {
    list: () => ['exchangeRates', 'list'] as const,
  },
  imports: {
    batch: (batchId: string) => ['imports', 'batch', batchId] as const,
  },
  ledger: {
    summary: () => ['ledger', 'summary'] as const,
  },
  runtimeConfig: {
    public: () => ['runtimeConfig', 'public'] as const,
  },
  user: {
    me: () => ['user', 'me'] as const,
  },
  workspace: {
    bookContext: (bookId: string, revision = 0) => ['workspace', 'bookContext', bookId, revision] as const,
    foundation: (revision = 0) => ['workspace', 'foundation', revision] as const,
  },
};
