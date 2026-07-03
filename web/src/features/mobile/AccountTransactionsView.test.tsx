import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { type Account, type BookMember, type Category, type Entry } from '../../lib/api/ledger';
import { AccountTransactionsView } from './AccountTransactionsView';
import { TransactionSearchView } from './TransactionSearchView';
import { accountEntries } from './account-transaction-utils';

const account: Account = {
  id: 'account-checking',
  userId: 'user-1',
  groupId: 'group-1',
  name: 'Checking',
  type: 'cash',
  currency: 'USD',
  sharedBookIds: ['book-1'],
  openingBalanceCents: 100000,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const transferTarget: Account = {
  ...account,
  id: 'account-savings',
  name: 'Savings',
};

const categories: Category[] = [
  {
    id: 'cat-home',
    bookId: 'book-1',
    name: 'Housing',
    direction: 'expense',
    sortOrder: 0,
    archived: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'cat-rent',
    bookId: 'book-1',
    parentId: 'cat-home',
    name: 'Rent',
    direction: 'expense',
    sortOrder: 1,
    archived: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

const members: BookMember[] = [
  {
    bookId: 'book-1',
    userId: 'user-1',
    role: 'owner',
    displayName: 'Alex Chen',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

const entries: Entry[] = [
  {
    id: 'entry-income',
    bookId: 'book-1',
    creatorUserId: 'user-1',
    type: 'income',
    accountId: 'account-checking',
    amountCents: 25000,
    transactionCurrency: 'USD',
    accountCurrency: 'USD',
    bookReportingCurrency: 'USD',
    occurredAt: '2026-07-02T09:00:00Z',
    note: 'Salary',
    createdAt: '2026-07-02T09:00:00Z',
    updatedAt: '2026-07-02T09:00:00Z',
  },
  {
    id: 'entry-rent',
    bookId: 'book-1',
    creatorUserId: 'user-1',
    type: 'expense',
    accountId: 'account-checking',
    categoryId: 'cat-rent',
    amountCents: 1200,
    transactionCurrency: 'USD',
    accountCurrency: 'USD',
    bookReportingCurrency: 'USD',
    occurredAt: '2026-07-01T07:51:00Z',
    merchant: 'Landlord',
    createdAt: '2026-07-01T07:51:00Z',
    updatedAt: '2026-07-01T07:51:00Z',
  },
  {
    id: 'entry-transfer-out',
    bookId: 'book-1',
    creatorUserId: 'user-1',
    type: 'transfer',
    accountId: 'account-checking',
    destinationAccountId: 'account-savings',
    amountCents: 5000,
    transactionCurrency: 'USD',
    accountCurrency: 'USD',
    bookReportingCurrency: 'USD',
    occurredAt: '2026-06-15T12:00:00Z',
    note: 'Move to savings',
    createdAt: '2026-06-15T12:00:00Z',
    updatedAt: '2026-06-15T12:00:00Z',
  },
  {
    id: 'entry-transfer-in',
    bookId: 'book-1',
    creatorUserId: 'user-1',
    type: 'transfer',
    accountId: 'account-savings',
    destinationAccountId: 'account-checking',
    amountCents: 7000,
    transactionCurrency: 'USD',
    accountCurrency: 'USD',
    bookReportingCurrency: 'USD',
    occurredAt: '2026-06-10T12:00:00Z',
    note: 'Savings return',
    createdAt: '2026-06-10T12:00:00Z',
    updatedAt: '2026-06-10T12:00:00Z',
  },
];

describe('AccountTransactionsView', () => {
  it('shows running account balance and monthly transaction details', () => {
    render(
      <AccountTransactionsView
        account={account}
        categories={categories}
        entries={entries}
        isLoading={false}
        members={members}
      />,
    );

    expect(screen.getByRole('region', { name: 'Checking transactions' })).toHaveTextContent('$1,258.00');
    expect(screen.getByRole('button', { name: /July/i })).toHaveTextContent('In $250.00 / Out $12.00');
    expect(screen.getByText('Salary')).toBeInTheDocument();
    expect(screen.getByText('Landlord')).toBeInTheDocument();
    expect(screen.getAllByText(/Alex Chen/)).toHaveLength(2);
  });

  it('filters account entries through source and destination transfer accounts', () => {
    expect(accountEntries(account.id, entries).map((entry) => entry.id)).toEqual([
      'entry-income',
      'entry-rent',
      'entry-transfer-out',
      'entry-transfer-in',
    ]);
    expect(accountEntries(transferTarget.id, entries).map((entry) => entry.id)).toEqual([
      'entry-transfer-out',
      'entry-transfer-in',
    ]);
  });
});

describe('TransactionSearchView', () => {
  it('matches account-scoped transactions by parent category, amount, and member', () => {
    render(
      <TransactionSearchView
        accounts={[account, transferTarget]}
        categories={categories}
        entries={[entries[1]]}
        error=""
        isLoading={false}
        members={members}
        onClose={() => undefined}
        title="Search Checking"
      />,
    );

    fireEvent.change(screen.getByRole('textbox', { name: 'Search transactions' }), { target: { value: 'housing' } });
    expect(screen.getByText('Landlord')).toBeInTheDocument();

    fireEvent.change(screen.getByRole('textbox', { name: 'Search transactions' }), { target: { value: '12' } });
    expect(screen.getByText('Landlord')).toBeInTheDocument();

    fireEvent.change(screen.getByRole('textbox', { name: 'Search transactions' }), { target: { value: 'alex' } });
    expect(screen.getByText('Landlord')).toBeInTheDocument();
  });
});
