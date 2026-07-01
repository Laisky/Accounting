import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';

describe('App', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ balanceCents: 12500, currency: 'USD', entryCount: 3 }),
      }),
    );
  });

  it('renders the main accounting workspace', async () => {
    render(<App />);

    expect(screen.getByText('Accounting workspace')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Import Wacai books into a clean ledger.' })).toBeInTheDocument();
    expect(await screen.findByText('$125.00')).toBeInTheDocument();
  });

  it('stages a Wacai export file for import review', async () => {
    render(<App />);

    const input = screen.getByLabelText('Upload Wacai export file');
    const file = new File(['date,amount\n2026-07-01,12.30'], 'wacai-export.csv', { type: 'text/csv' });

    fireEvent.change(input, { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: 'Stage import' }));

    expect(await screen.findByText('wacai-export.csv')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Import staged' })).toBeInTheDocument();
    expect(screen.getByText('MinIO object')).toBeInTheDocument();
    expect(screen.getByText('PostgreSQL batch')).toBeInTheDocument();
  });
});
