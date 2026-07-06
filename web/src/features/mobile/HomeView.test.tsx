import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { HomeView } from './HomeView';

describe('HomeView', () => {
  it('shows zero budget values for an empty ledger instead of a fake cent', () => {
    render(
      <HomeView
        accounts={[]}
        bookName="Household"
        categories={[]}
        currencyCode="USD"
        entries={[]}
        rateIndex={new Map()}
        summary={{ balanceCents: 0, currency: 'USD', entryCount: 0 }}
      />,
    );

    const budget = screen.getByRole('region', { name: 'Monthly spending budget' });
    expect(budget).toHaveTextContent('Remaining $0.00');
    expect(within(budget).getByText('Total $0.00')).toBeInTheDocument();
    expect(within(budget).getByText('Spent $0.00')).toBeInTheDocument();
    expect(within(budget).getByText('0%')).toBeInTheDocument();
  });

  it('converts summary and day totals into the profile currency', () => {
    render(
      <HomeView
        accounts={[]}
        bookName="Household"
        categories={[]}
        currencyCode="EUR"
        entries={[
          {
            id: 'entry-1',
            bookId: 'book-1',
            creatorUserId: 'user-1',
            type: 'expense',
            accountId: 'account-1',
            amountCents: 10000,
            transactionCurrency: 'USD',
            accountCurrency: 'USD',
            bookReportingCurrency: 'USD',
            occurredAt: new Date().toISOString(),
            createdAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T00:00:00Z',
          },
        ]}
        rateIndex={
          new Map([
            ['USD', 1],
            ['EUR', 0.9],
          ])
        }
        summary={{ balanceCents: 20000, currency: 'USD', entryCount: 1 }}
      />,
    );

    expect(screen.getByText('Spent €90.00')).toBeInTheDocument();
    expect(screen.getByText(/Income €0.00 Expense €90.00/)).toBeInTheDocument();
  });
});
