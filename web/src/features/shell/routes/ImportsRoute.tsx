import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { useBook } from '@/contexts/BookContext';
import { useNotice } from '@/contexts/NoticeContext';
import { useSession } from '@/contexts/SessionContext';
import { ImportPreviewView } from '@/features/mobile/ImportPreviewView';
import { useShellOutlet } from '@/features/shell/shellOutlet';
import { useBookMembersQuery } from '@/hooks/useBookMembers';
import { useCreateBookMutation } from '@/hooks/useBooks';
import { queryKeys } from '@/hooks/queryKeys';

// ImportsRoute wires the import preview/apply flow to book hooks and refreshes affected caches.
function ImportsRoute() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { actor } = useSession();
  const { books, selectedBookId, setSelectedBookId, bookCurrency, displayCurrency } = useBook();
  const { notifyStatus } = useNotice();
  const { setProcessing } = useShellOutlet();
  const members = useBookMembersQuery(selectedBookId).data ?? [];
  const createBook = useCreateBookMutation();

  // handleCreateBook creates a destination book for an import batch and selects it.
  async function handleCreateBook(name: string): Promise<string> {
    const book = await createBook.mutateAsync({
      name,
      reportingCurrency: bookCurrency || displayCurrency || 'USD',
    });
    setSelectedBookId(book.id);
    return book.id;
  }

  // handleApplied refreshes every cache an applied import can touch.
  function handleApplied() {
    notifyStatus(t('imports.stage.applyComplete'));
    void queryClient.invalidateQueries({ queryKey: ['books', selectedBookId, 'entries'] });
    void queryClient.invalidateQueries({ queryKey: ['books', selectedBookId, 'categories'] });
    void queryClient.invalidateQueries({ queryKey: ['accounts'] });
    void queryClient.invalidateQueries({ queryKey: ['books', 'list'] });
    void queryClient.invalidateQueries({ queryKey: queryKeys.ledger.summary() });
    void queryClient.invalidateQueries({ queryKey: ['reports'] });
  }

  return (
    <ImportPreviewView
      actor={actor}
      books={books}
      members={members}
      onApplied={handleApplied}
      onCreateBook={handleCreateBook}
      onProcessingChange={setProcessing}
      selectedBookId={selectedBookId}
      setSelectedBookId={setSelectedBookId}
    />
  );
}

export default ImportsRoute;
