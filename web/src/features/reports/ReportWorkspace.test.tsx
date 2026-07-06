import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter, Navigate, Route, Routes, useParams } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import { ReportWorkspace } from './ReportWorkspace';
import { isReportTab, reportTabPath } from './reportWorkspaceModel';
import type { Account, BookListItem, BookMember, Category, Entry } from '@/lib/api/ledger';

const fixtures = vi.hoisted(() => {
  const book: BookListItem = {
    id: 'book-1',
    ownerUserId: 'user-1',
    name: 'Household',
    reportingCurrency: 'USD',
    role: 'owner',
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  };
  const account: Account = {
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
  const expenseCategory: Category = {
    id: 'category-expense',
    bookId: 'book-1',
    name: 'Dining',
    direction: 'expense',
    sortOrder: 0,
    archived: false,
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
  };
  const incomeCategory: Category = {
    ...expenseCategory,
    id: 'category-income',
    name: 'Salary',
    direction: 'income',
  };
  const members: BookMember[] = [
    {
      bookId: 'book-1',
      userId: 'user-1',
      role: 'owner',
      displayName: 'Person',
      createdAt: '2026-07-01T00:00:00Z',
      updatedAt: '2026-07-01T00:00:00Z',
    },
    {
      bookId: 'book-1',
      userId: 'user-2',
      role: 'member',
      displayName: 'Roommate',
      createdAt: '2026-07-01T00:00:00Z',
      updatedAt: '2026-07-01T00:00:00Z',
    },
  ];
  const entries: Entry[] = [
    {
      id: 'income-1',
      bookId: 'book-1',
      creatorUserId: 'user-1',
      type: 'income',
      accountId: 'account-1',
      categoryId: 'category-income',
      amountCents: 125000,
      transactionCurrency: 'USD',
      accountCurrency: 'USD',
      bookReportingCurrency: 'USD',
      occurredAt: '2026-07-01T08:00:00Z',
      note: 'Salary',
      createdAt: '2026-07-01T08:00:00Z',
      updatedAt: '2026-07-01T08:00:00Z',
    },
    {
      id: 'expense-1',
      bookId: 'book-1',
      creatorUserId: 'user-1',
      type: 'expense',
      accountId: 'account-1',
      categoryId: 'category-expense',
      amountCents: 4100,
      transactionCurrency: 'USD',
      accountCurrency: 'USD',
      bookReportingCurrency: 'USD',
      occurredAt: '2026-07-02T09:00:00Z',
      note: 'Groceries',
      createdAt: '2026-07-02T09:00:00Z',
      updatedAt: '2026-07-02T09:00:00Z',
    },
    {
      id: 'income-2',
      bookId: 'book-1',
      creatorUserId: 'user-2',
      type: 'income',
      accountId: 'account-1',
      categoryId: 'category-income',
      amountCents: 50000,
      transactionCurrency: 'USD',
      accountCurrency: 'USD',
      bookReportingCurrency: 'USD',
      occurredAt: '2026-07-03T08:00:00Z',
      note: 'Shared refund',
      createdAt: '2026-07-03T08:00:00Z',
      updatedAt: '2026-07-03T08:00:00Z',
    },
    {
      id: 'expense-2',
      bookId: 'book-1',
      creatorUserId: 'user-2',
      type: 'expense',
      accountId: 'account-1',
      categoryId: 'category-expense',
      amountCents: 20000,
      transactionCurrency: 'USD',
      accountCurrency: 'USD',
      bookReportingCurrency: 'USD',
      occurredAt: '2026-07-04T09:00:00Z',
      note: 'Utilities',
      createdAt: '2026-07-04T09:00:00Z',
      updatedAt: '2026-07-04T09:00:00Z',
    },
  ];

  return { account, book, categories: [expenseCategory, incomeCategory], entries, members };
});

vi.mock('@/lib/api/ledger', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api/ledger')>();
  return {
    ...actual,
    fetchAccounts: vi.fn(async () => [fixtures.account]),
    fetchAllEntries: vi.fn(async () => fixtures.entries),
    fetchBooks: vi.fn(async () => [fixtures.book]),
    fetchBookMembers: vi.fn(async () => fixtures.members),
    fetchCategories: vi.fn(async () => fixtures.categories),
    fetchExchangeRates: vi.fn(async () => []),
  };
});

function renderReport(path = '/reports/category') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/reports" element={<Navigate to={reportTabPath('category')} replace />} />
        <Route path="/reports/:dimension" element={<RoutedReportWorkspace />} />
      </Routes>
    </MemoryRouter>,
  );
}

function RoutedReportWorkspace() {
  const { dimension } = useParams();
  if (!isReportTab(dimension)) {
    return <Navigate to={reportTabPath('category')} replace />;
  }

  return <ReportWorkspace activeTab={dimension} />;
}

describe('ReportWorkspace', () => {
  it('shows category expense, income, and balance sections by default', async () => {
    renderReport();

    const panel = await screen.findByRole('tabpanel', { name: 'Category' });
    expect(within(panel).getByRole('heading', { name: 'Category expense' })).toBeInTheDocument();
    expect(within(panel).getByRole('heading', { name: 'Category income' })).toBeInTheDocument();
    expect(within(panel).getByRole('heading', { name: 'Category balance' })).toBeInTheDocument();
    expect(await within(panel).findAllByText('Dining')).not.toHaveLength(0);
    expect(panel).toHaveTextContent('Dining');
    expect(panel).toHaveTextContent('$241.00');
    expect(panel).toHaveTextContent('Salary');
    expect(panel).toHaveTextContent('$1,750.00');
    expect(panel).toHaveTextContent('$1,509.00');
  });

  it('shows subcategory expense, income, and balance sections when all flow is selected', async () => {
    renderReport();

    fireEvent.click(await screen.findByRole('tab', { name: 'Subcategory' }));

    const panel = await screen.findByRole('tabpanel', { name: 'Subcategory' });
    expect(within(panel).getByRole('heading', { name: 'Subcategory expense' })).toBeInTheDocument();
    expect(within(panel).getByRole('heading', { name: 'Subcategory income' })).toBeInTheDocument();
    expect(within(panel).getByRole('heading', { name: 'Subcategory balance' })).toBeInTheDocument();
    expect(await within(panel).findAllByText('Dining')).not.toHaveLength(0);
    expect(panel).toHaveTextContent('$241.00');
    expect(panel).toHaveTextContent('$1,750.00');
    expect(panel).toHaveTextContent('$1,509.00');
  });

  it('shows member expense, income, and balance sections with a generated shared total row', async () => {
    renderReport();

    fireEvent.click(await screen.findByRole('tab', { name: 'Member' }));

    const panel = await screen.findByRole('tabpanel', { name: 'Member' });
    expect(within(panel).getByRole('heading', { name: 'Member expense' })).toBeInTheDocument();
    expect(within(panel).getByRole('heading', { name: 'Member income' })).toBeInTheDocument();
    expect(within(panel).getByRole('heading', { name: 'Member balance' })).toBeInTheDocument();
    expect(panel).toHaveTextContent('Family shared');
    expect(panel).toHaveTextContent('Person');
    expect(panel).toHaveTextContent('Roommate');
    expect(panel).toHaveTextContent('$241.00');
    expect(panel).toHaveTextContent('$1,750.00');
    expect(panel).toHaveTextContent('$1,509.00');
  });

  it('shows income, expense, and balance summaries with a trend chart', async () => {
    renderReport('/reports/trend');

    const panel = await screen.findByRole('tabpanel', { name: 'Trend' });
    expect(within(panel).getByRole('heading', { name: 'Income, expense, and balance trend' })).toBeInTheDocument();
    const summary = within(panel).getByLabelText('Trend summary');
    expect(summary).toHaveTextContent('Income');
    expect(await within(summary).findByText('$1,750.00')).toBeInTheDocument();
    expect(summary).toHaveTextContent('$1,750.00');
    expect(summary).toHaveTextContent('Expense');
    expect(summary).toHaveTextContent('$241.00');
    expect(summary).toHaveTextContent('Balance');
    expect(summary).toHaveTextContent('$1,509.00');
    expect(within(panel).getByRole('img', { name: /Cashflow trend chart/ })).toBeInTheDocument();
  });
});
