import { BarChart3, CircleUserRound, House, Plus, WalletCards } from 'lucide-react';
import { type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { type MobileTab } from './mobile-workspace-utils';

type MobileBottomNavProps = {
  activeTab: MobileTab;
  onOpenTab: (tab: MobileTab) => void;
};

const bottomNavItems: Array<{ id: Exclude<MobileTab, 'imports'>; icon: ReactNode }> = [
  { id: 'accounts', icon: <WalletCards size={22} /> },
  { id: 'record', icon: <Plus size={24} /> },
  { id: 'home', icon: <House size={22} /> },
  { id: 'reports', icon: <BarChart3 size={22} /> },
  { id: 'me', icon: <CircleUserRound size={22} /> },
];

// MobileBottomNav receives the active tab and returns the authenticated mobile navigation.
export function MobileBottomNav({ activeTab, onOpenTab }: MobileBottomNavProps) {
  const { t } = useTranslation();

  return (
    <nav className="bottomNav" aria-label={t('mobile.a11y.mainNavigation')}>
      <div className="navBrand">
        <WalletCards size={22} aria-hidden="true" />
        <span>{t('mobile.brand')}</span>
      </div>
      {bottomNavItems.map((item) => {
        // The imports view is reached from the Me tab, so keep Me highlighted while it is open.
        const isActive = activeTab === item.id || (item.id === 'me' && activeTab === 'imports');
        return (
          <button
            key={item.id}
            type="button"
            className={isActive ? 'bottomNavActive' : undefined}
            aria-current={isActive ? 'page' : undefined}
            onClick={() => onOpenTab(item.id)}
          >
            <span>{item.icon}</span>
            <b>{t(`mobile.nav.${item.id}`)}</b>
          </button>
        );
      })}
    </nav>
  );
}
