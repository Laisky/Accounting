import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { useBook } from '@/contexts/BookContext';
import { useNotice } from '@/contexts/NoticeContext';
import { useSession } from '@/contexts/SessionContext';
import { MeView } from '@/features/mobile/MeView';
import { useShellChrome } from '@/features/shell/useShellChrome';
import { useUpdateUserProfileMutation } from '@/hooks/useUserProfile';

// MeRoute wires the account/settings tab to session, book, and profile hooks.
function MeRoute() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { actor, onLogout, runtimeConfig } = useSession();
  const { displayCurrency } = useBook();
  const { meSection } = useShellChrome();
  const { notifyError, notifyStatus } = useNotice();
  const updateProfile = useUpdateUserProfileMutation();
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  // handleLogout tracks the pending state while the session is cleared.
  async function handleLogout() {
    setIsLoggingOut(true);
    try {
      await onLogout();
    } finally {
      setIsLoggingOut(false);
    }
  }

  // handleUpdateBaseCurrency persists the user's display currency preference.
  async function handleUpdateBaseCurrency(currency: string) {
    try {
      await updateProfile.mutateAsync({ baseCurrency: currency });
      notifyStatus(t('mobile.status.profileCurrencyUpdated'));
    } catch {
      notifyError(t('mobile.error.profileCurrencyFailed'));
    }
  }

  return (
    <MeView
      actor={actor}
      baseCurrency={displayCurrency}
      isLoggingOut={isLoggingOut}
      isProfileSaving={updateProfile.isPending}
      meSection={meSection}
      onBack={() => navigate('/me')}
      onLogout={handleLogout}
      onOpenImports={() => navigate('/imports')}
      onOpenProfile={() => navigate('/me/profile')}
      onOpenSecurity={() => navigate('/me/security')}
      onUpdateBaseCurrency={handleUpdateBaseCurrency}
      runtimeConfig={runtimeConfig}
    />
  );
}

export default MeRoute;
