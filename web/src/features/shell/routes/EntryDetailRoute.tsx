import { useNavigate, useParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { useBook } from '@/contexts/BookContext';
import { useNotice } from '@/contexts/NoticeContext';
import { EntryDetailView } from '@/features/mobile/EntryDetailView';
import { useShellOutlet } from '@/features/shell/shellOutlet';
import { useAccountsQuery } from '@/hooks/useAccounts';
import { useBookMembersQuery } from '@/hooks/useBookMembers';
import { useCategoriesQuery } from '@/hooks/useCategories';
import {
  useCreateEntryMutation,
  useDeleteEntryMutation,
  useEntryDetail,
  useRecentEntriesQuery,
  useUpdateEntryMutation,
} from '@/hooks/useEntries';
import type { Entry, EntryUpdateInput } from '@/lib/api/ledger';

// EntryDetailRoute resolves one entry from the shared cache and runs entry mutations.
function EntryDetailRoute() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params = useParams();
  const entryId = params.entryId ?? null;
  const { entryEditorOpenSignal } = useShellOutlet();
  const { books, selectedBook } = useBook();
  const { notifyError, notifyStatus, notifyUndo } = useNotice();
  const accounts = useAccountsQuery().data ?? [];
  const categories = useCategoriesQuery(selectedBook?.id).data ?? [];
  const members = useBookMembersQuery(selectedBook?.id).data ?? [];
  const recentEntries = useRecentEntriesQuery(selectedBook?.id).data?.entries ?? [];
  const initialEntry = entryId ? recentEntries.find((entry) => entry.id === entryId) : undefined;
  const { entry, isError, isLoading } = useEntryDetail(selectedBook?.id, entryId, initialEntry);
  const updateEntry = useUpdateEntryMutation();
  const deleteEntry = useDeleteEntryMutation();
  const createEntry = useCreateEntryMutation();

  // recreateEntry re-posts a just-deleted entry to satisfy the delete undo window (P5.5).
  // Backend soft-delete/restore is out of scope, so undo produces a fresh entry with the
  // same fields (a new id), which the audit log records as a create.
  async function recreateEntry(bookId: string, removed: Entry) {
    if (!removed.accountId) {
      notifyError(t('mobile.error.entryRestoreFailed'));
      return;
    }
    try {
      await createEntry.mutateAsync({
        bookId,
        input: {
          type: removed.type,
          accountId: removed.accountId,
          destinationAccountId: removed.destinationAccountId,
          categoryId: removed.categoryId,
          amountCents: removed.amountCents,
          transactionCurrency: removed.transactionCurrency,
          bookReportingCurrency: removed.bookReportingCurrency,
          occurredAt: removed.occurredAt,
          note: removed.note,
          merchant: removed.merchant,
          tags: removed.tags,
        },
      });
      notifyStatus(t('mobile.status.entryRestored'));
    } catch {
      notifyError(t('mobile.error.entryRestoreFailed'));
    }
  }

  // handleUpdateEntry patches the entry against the book that owns it.
  async function handleUpdateEntry(id: string, input: EntryUpdateInput) {
    const bookId = entry?.bookId ?? selectedBook?.id;
    if (!bookId) {
      notifyError(t('mobile.error.entryUpdateFailed'));
      return;
    }
    try {
      await updateEntry.mutateAsync({ bookId, entryId: id, input });
      notifyStatus(t('mobile.status.entryUpdated'));
    } catch {
      notifyError(t('mobile.error.entryUpdateFailed'));
    }
  }

  // handleDeleteEntry removes the entry, returns to the home feed, and offers a 10s undo.
  async function handleDeleteEntry(id: string) {
    const removed = entry;
    const bookId = removed?.bookId ?? selectedBook?.id;
    if (!bookId) {
      notifyError(t('mobile.error.entryDeleteFailed'));
      return;
    }
    try {
      await deleteEntry.mutateAsync({ bookId, entryId: id });
      navigate('/home');
      if (removed) {
        notifyUndo(t('mobile.status.entryDeleted'), () => void recreateEntry(bookId, removed));
      } else {
        notifyStatus(t('mobile.status.entryDeleted'));
      }
    } catch {
      notifyError(t('mobile.error.entryDeleteFailed'));
    }
  }

  return (
    <EntryDetailView
      accounts={accounts}
      books={books}
      categories={categories}
      editorOpenSignal={entryEditorOpenSignal}
      entry={entry}
      error={isError ? t('mobile.entryDetail.error') : undefined}
      isLoading={isLoading}
      isSaving={updateEntry.isPending || deleteEntry.isPending}
      members={members}
      onDeleteEntry={handleDeleteEntry}
      onUpdateEntry={handleUpdateEntry}
    />
  );
}

export default EntryDetailRoute;
