import { Check, ChevronRight, Copy, FileSpreadsheet, History, LogOut } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LanguageSelector } from '../../components/LanguageSelector';
import { type AuditEvent } from '../../lib/api/audit';
import { type AuthActor } from '../../lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { copyToClipboard } from '../../lib/clipboard';
import { PasskeySettingsView } from './PasskeySettingsView';
import { TotpSettingsView } from './TotpSettingsView';
import './me-view.css';

type MeViewProps = {
  actor: AuthActor;
  activityEvents: AuditEvent[];
  isActivityLoading: boolean;
  isLoggingOut: boolean;
  onLoadActivity: () => void;
  onLogout: () => void;
  onOpenImports: () => void;
  runtimeConfig: RuntimeConfig | null;
};

// MeView renders the personal account tab as a native-grouped settings index:
// an identity banner followed by labeled Security / Activity / Preferences / Data
// groups and an isolated sign-out.
export function MeView({
  actor,
  activityEvents,
  isActivityLoading,
  isLoggingOut,
  onLoadActivity,
  onLogout,
  onOpenImports,
  runtimeConfig,
}: MeViewProps) {
  const { t } = useTranslation();
  const config = runtimeConfig ?? emptyRuntimeConfig;
  const [uidCopied, setUidCopied] = useState(false);
  const monogram = actor.email.trim().charAt(0).toUpperCase() || '?';
  const statusIsActive = actor.status.toLowerCase() === 'active';
  const securityAvailable = config.features.passkeyEnabled || config.features.totpEnabled;

  // handleCopyUid receives no parameters and copies the account UID to the clipboard, showing brief confirmation.
  async function handleCopyUid() {
    const copied = await copyToClipboard(actor.userId);
    if (!copied) {
      return;
    }
    setUidCopied(true);
    window.setTimeout(() => setUidCopied(false), 2000);
  }

  return (
    <section className="meScreen" aria-label={t('mobile.nav.me')}>
      <header className="meIdentity">
        <span className="meAvatar" aria-hidden="true">{monogram}</span>
        <div className="meIdentityMain">
          <strong className="meEmail" dir="auto">{actor.email}</strong>
          <span className={`mePill ${statusIsActive ? 'mePillPositive' : 'mePillNeutral'}`}>
            <span className="mePillDot" aria-hidden="true" />
            {actor.status}
          </span>
        </div>
        <div className="meUid">
          <div className="meUidText">
            <span>{t('mobile.me.accountUid')}</span>
            <code dir="ltr">{actor.userId}</code>
          </div>
          <button type="button" className="meCopyButton" aria-label={t('mobile.me.copyUid')} onClick={handleCopyUid}>
            {uidCopied ? <Check size={15} aria-hidden="true" /> : <Copy size={15} aria-hidden="true" />}
            <span role="status" aria-live="polite">{uidCopied ? t('mobile.me.uidCopied') : t('mobile.me.copy')}</span>
          </button>
        </div>
      </header>

      <section className="meGroup">
        <h2 className="meGroupHeader">{t('mobile.me.groupSecurity')}</h2>
        <PasskeySettingsView featureEnabled={config.features.passkeyEnabled} />
        <TotpSettingsView featureEnabled={config.features.totpEnabled} />
        {!securityAvailable ? <p className="meEmpty">{t('mobile.me.emailPasswordOnly')}</p> : null}
      </section>

      <section className="meGroup">
        <h2 className="meGroupHeader">{t('mobile.me.groupActivity')}</h2>
        <div className="meCard meActivityCard">
          <button className="mobileSecondaryButton" type="button" disabled={isActivityLoading} onClick={onLoadActivity}>
            <History size={16} aria-hidden="true" />
            {t('mobile.me.loadActivity')}
          </button>
          {activityEvents.length ? (
            <ul className="meActivityList">
              {activityEvents.slice(0, 8).map((event) => (
                <li key={event.id}>
                  <span className="meActivityAction">{event.action.replace('.', ' / ')}</span>
                  <time dateTime={event.createdAt}>{formatAuditTime(event.createdAt)}</time>
                </li>
              ))}
            </ul>
          ) : (
            <p className="meEmpty">{t('mobile.me.activityEmpty')}</p>
          )}
        </div>
      </section>

      <section className="meGroup">
        <h2 className="meGroupHeader">{t('mobile.me.groupPreferences')}</h2>
        <LanguageSelector className="meLanguageRow" />
      </section>

      <section className="meGroup">
        <h2 className="meGroupHeader">{t('mobile.me.groupData')}</h2>
        <button type="button" className="meNavRow" aria-label={t('mobile.nav.imports')} onClick={onOpenImports}>
          <span className="meNavIcon" aria-hidden="true">
            <FileSpreadsheet size={18} />
          </span>
          <span className="meNavLabel">
            <b>{t('mobile.nav.imports')}</b>
            <small>{t('mobile.me.importHint')}</small>
          </span>
          <ChevronRight className="meNavChevron" size={18} aria-hidden="true" />
        </button>
      </section>

      <div className="meSignOut">
        <button className="meSignOutButton" type="button" disabled={isLoggingOut} onClick={onLogout}>
          <LogOut size={17} aria-hidden="true" />
          {t('common.signOut')}
        </button>
      </div>
    </section>
  );
}

// formatAuditTime receives an ISO timestamp and returns a compact UTC string.
function formatAuditTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toISOString().replace('.000Z', 'Z');
}
