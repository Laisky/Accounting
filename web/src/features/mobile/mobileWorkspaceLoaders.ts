import { fetchUserProfile } from '@/lib/api/auth';
import {
  emptyLedgerSummary,
  fetchAccountGroups,
  fetchAccounts,
  fetchBookMembers,
  fetchBooks,
  fetchCategories,
  fetchEntries,
  fetchExchangeRates,
  fetchLedgerSummary,
} from '@/lib/api/ledger';
import { normalizeCurrencyCode } from '@/lib/money';

export async function loadMobileFoundation() {
  const [summary, books, groups, accounts, rates, profile] = await Promise.all([
    fetchLedgerSummary(new AbortController().signal).catch(() => emptyLedgerSummary),
    fetchBooks(),
    fetchAccountGroups(),
    fetchAccounts(),
    fetchExchangeRates(),
    fetchUserProfile(),
  ]);

  return {
    accounts,
    baseCurrency: normalizeCurrencyCode(profile.baseCurrency || 'USD'),
    books,
    groups,
    rates,
    summary,
  };
}

export async function loadMobileBookContext(bookId: string) {
  const [categories, entryList, members] = await Promise.all([
    fetchCategories(bookId),
    fetchEntries(bookId),
    fetchBookMembers(bookId),
  ]);

  return {
    categories,
    entries: entryList.entries,
    members,
    totalEntries: entryList.total,
  };
}
