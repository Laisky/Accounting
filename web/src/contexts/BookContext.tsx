import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useExchangeRatesQuery } from '@/hooks/useAccounts';
import { useBooksQuery } from '@/hooks/useBooks';
import { useUserProfileQuery } from '@/hooks/useUserProfile';
import type { BookListItem } from '@/lib/api/ledger';
import { buildRateIndex, normalizeCurrencyCode } from '@/lib/money';

type BookContextValue = {
  books: BookListItem[];
  selectedBookId: string;
  selectedBook: BookListItem | undefined;
  setSelectedBookId: (id: string) => void;
  bookCurrency: string;
  displayCurrency: string;
  rateIndex: Map<string, number>;
  canManageCategories: boolean;
  isFoundationLoading: boolean;
};

const BookContext = createContext<BookContextValue | null>(null);

type BookProviderProps = {
  children: ReactNode;
};

// BookProvider owns the selected book plus its derived reporting/display currency and rate index.
export function BookProvider({ children }: BookProviderProps) {
  const booksQuery = useBooksQuery();
  const ratesQuery = useExchangeRatesQuery();
  const profileQuery = useUserProfileQuery();
  const [selectedBookId, setSelectedBookId] = useState('');

  const books = useMemo(() => booksQuery.data ?? [], [booksQuery.data]);

  // Default to the first visible book, and never keep a selection that no longer exists.
  useEffect(() => {
    const first = books[0];
    if (!first) {
      return;
    }
    setSelectedBookId((current) => (current && books.some((book) => book.id === current) ? current : first.id));
  }, [books]);

  const selectedBook = books.find((book) => book.id === selectedBookId) ?? books[0];
  const bookCurrency = selectedBook?.reportingCurrency ?? 'USD';
  const baseCurrency = normalizeCurrencyCode(profileQuery.data?.baseCurrency || 'USD');
  const displayCurrency = baseCurrency || bookCurrency || 'USD';
  const canManageCategories = selectedBook?.role === 'owner' || selectedBook?.role === 'administrator';
  const rateIndex = useMemo(() => buildRateIndex(ratesQuery.data ?? []), [ratesQuery.data]);

  const value = useMemo<BookContextValue>(
    () => ({
      books,
      selectedBookId: selectedBook?.id ?? '',
      selectedBook,
      setSelectedBookId,
      bookCurrency,
      displayCurrency,
      rateIndex,
      canManageCategories,
      isFoundationLoading: booksQuery.isLoading,
    }),
    [books, selectedBook, bookCurrency, displayCurrency, rateIndex, canManageCategories, booksQuery.isLoading],
  );

  return <BookContext value={value}>{children}</BookContext>;
}

// useBook returns the selected book plus derived currency, rate index, and permissions.
export function useBook(): BookContextValue {
  const value = useContext(BookContext);
  if (!value) {
    throw new Error('useBook must be used within BookProvider');
  }

  return value;
}
