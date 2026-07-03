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
        summary={{ balanceCents: 0, currency: 'USD', entryCount: 0 }}
      />,
    );

    const budget = screen.getByRole('region', { name: 'Monthly spending budget' });
    expect(budget).toHaveTextContent('Remaining $0.00');
    expect(within(budget).getByText('Total $0.00')).toBeInTheDocument();
    expect(within(budget).getByText('Spent $0.00')).toBeInTheDocument();
    expect(within(budget).getByText('0%')).toBeInTheDocument();
  });
});
