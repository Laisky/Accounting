import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useBook } from '@/contexts/BookContext';
import { useNotice } from '@/contexts/NoticeContext';
import {
  useAccountGroupsQuery,
  useAccountsQuery,
  useCreateAccountGroupMutation,
  useCreateAccountMutation,
} from '@/hooks/useAccounts';
import { useCreateBookMutation } from '@/hooks/useBooks';

// usePrepareStarterAccount bootstraps a first book, account group, and cash account so a new
// user can start recording immediately. It is shared by the header quick action and the
// accounts empty state.
export function usePrepareStarterAccount() {
  const { t } = useTranslation();
  const { selectedBook, displayCurrency, setSelectedBookId } = useBook();
  const accountsQuery = useAccountsQuery();
  const groupsQuery = useAccountGroupsQuery();
  const createBook = useCreateBookMutation();
  const createGroup = useCreateAccountGroupMutation();
  const createAccount = useCreateAccountMutation();
  const { notifyStatus, notifyError } = useNotice();

  const isPending = createBook.isPending || createGroup.isPending || createAccount.isPending;

  const prepare = useCallback(async () => {
    try {
      const accounts = accountsQuery.data ?? [];
      const groups = groupsQuery.data ?? [];
      const book =
        selectedBook ??
        (await createBook.mutateAsync({ name: 'Household', reportingCurrency: displayCurrency || 'USD' }));
      const group = groups[0] ?? (await createGroup.mutateAsync('Everyday'));
      const primary = accounts.find((account) => !selectedBook || account.sharedBookIds?.includes(selectedBook.id));
      if (!primary) {
        await createAccount.mutateAsync({
          groupId: group.id,
          name: 'Cash',
          type: 'cash',
          currency: book.reportingCurrency,
          sharedBookIds: [book.id],
        });
      }
      notifyStatus(t('common.status.accountReady'));
      setSelectedBookId(book.id);
    } catch {
      notifyError(t('mobile.error.accountSetupFailed'));
    }
  }, [
    accountsQuery.data,
    createAccount,
    createBook,
    createGroup,
    displayCurrency,
    groupsQuery.data,
    notifyError,
    notifyStatus,
    selectedBook,
    setSelectedBookId,
    t,
  ]);

  return { prepare, isPending };
}
