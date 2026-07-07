import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { useBook } from '@/contexts/BookContext';
import { HomeView } from '@/features/mobile/HomeView';
import { useAccountsQuery, useLedgerSummaryQuery } from '@/hooks/useAccounts';
import { useCategoriesQuery } from '@/hooks/useCategories';
import { useRecentEntriesQuery } from '@/hooks/useEntries';
import { emptyLedgerSummary } from '@/lib/api/ledger';

// HomeRoute fetches the home transaction feed through hooks and renders the presentational HomeView.
function HomeRoute() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { selectedBook, displayCurrency, rateIndex } = useBook();
  const accounts = useAccountsQuery().data ?? [];
  const categories = useCategoriesQuery(selectedBook?.id).data ?? [];
  const summary = useLedgerSummaryQuery().data ?? emptyLedgerSummary;
  const entries = useRecentEntriesQuery(selectedBook?.id).data?.entries ?? [];

  return (
    <HomeView
      accounts={accounts}
      bookName={selectedBook?.name ?? t('mobile.defaultBookName')}
      categories={categories}
      currencyCode={displayCurrency}
      entries={entries}
      onOpenEntry={(entryId) => navigate(`/entries/${encodeURIComponent(entryId)}`)}
      onRecordEntry={() => navigate('/record')}
      rateIndex={rateIndex}
      summary={summary}
    />
  );
}

export default HomeRoute;
