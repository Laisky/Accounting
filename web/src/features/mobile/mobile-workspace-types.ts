import {
  type Account,
  type AccountGroup,
  type BookListItem,
  type BookMember,
  type Category,
  type Entry,
  type ExchangeRate,
} from '@/lib/api/ledger';

export type LedgerSnapshot = {
  groups: AccountGroup[];
  members: BookMember[];
  books: BookListItem[];
  accounts: Account[];
  categories: Category[];
  entries: Entry[];
  rates: ExchangeRate[];
  totalEntries: number;
};

export const emptyLedgerSnapshot: LedgerSnapshot = {
  groups: [],
  members: [],
  books: [],
  accounts: [],
  categories: [],
  entries: [],
  rates: [],
  totalEntries: 0,
};
