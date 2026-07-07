export type PageQuery = {
  page?: number;
  pageSize?: number;
};

export type EntryQuery = PageQuery & {
  accountId?: string;
  query?: string;
  revision?: number;
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
    totpStatus: () => ['auth', 'totpStatus'] as const,
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
    // all is the single canonical full-ledger query for a book. Reports, search,
    // account-detail, and entry-detail all read from this one cache entry and filter
    // client-side, so a book never triggers more than one full-ledger download.
    all: (bookId: string) => ['books', bookId, 'entries', 'all'] as const,
    detail: (bookId: string, entryId: string, revision = 0) => ['books', bookId, 'entries', entryId, revision] as const,
    list: (bookId: string, filters: EntryQuery = {}) => ['books', bookId, 'entries', filters] as const,
    // recent is the light first-page query that feeds the home and record views.
    recent: (bookId: string, pageSize = 20) => ['books', bookId, 'entries', 'recent', pageSize] as const,
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
  reports: {
    bookData: (bookId: string, revision = 0) => ['reports', 'bookData', bookId, revision] as const,
    foundation: (revision = 0) => ['reports', 'foundation', revision] as const,
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
