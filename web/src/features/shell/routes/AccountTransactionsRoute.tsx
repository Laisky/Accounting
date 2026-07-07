import { useNavigate, useParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { useBook } from '@/contexts/BookContext';
import { AccountTransactionsView } from '@/features/mobile/AccountTransactionsView';
import { useAccountsQuery } from '@/hooks/useAccounts';
import { useBookMembersQuery } from '@/hooks/useBookMembers';
import { useCategoriesQuery } from '@/hooks/useCategories';
import { useAccountEntries } from '@/hooks/useEntries';

// AccountTransactionsRoute resolves the routed account and its entries through the shared cache.
function AccountTransactionsRoute() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params = useParams();
  const accountId = params.accountId ?? null;
  const { selectedBook } = useBook();
  const account = useAccountsQuery().data?.find((item) => item.id === accountId);
  const categories = useCategoriesQuery(selectedBook?.id).data ?? [];
  const members = useBookMembersQuery(selectedBook?.id).data ?? [];
  const { entries, isError, isLoading } = useAccountEntries(selectedBook?.id, accountId);

  return (
    <AccountTransactionsView
      account={account}
      categories={categories}
      entries={entries}
      error={isError ? t('mobile.accountDetail.error') : undefined}
      isLoading={isLoading}
      members={members}
      onOpenEntry={(entryId) => navigate(`/entries/${encodeURIComponent(entryId)}`)}
    />
  );
}

export default AccountTransactionsRoute;
