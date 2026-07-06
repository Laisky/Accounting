import {
  Check,
  ChevronLeft,
  ChevronRight,
  Copy,
  FileSpreadsheet,
  History,
  LockKeyhole,
  LogOut,
  ShieldCheck,
  UserRound,
} from 'lucide-react';
import { type FormEvent, type ReactNode, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LanguageSelector } from '@/components/LanguageSelector';
import { type AuditEvent } from '@/lib/api/audit';
import { confirmPasswordReset, requestPasswordReset, type AuthActor } from '@/lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from '@/lib/api/runtimeConfig';
import { copyToClipboard } from '@/lib/clipboard';
import { supportedCurrencies } from '@/lib/money';
import { PasskeySettingsView } from './PasskeySettingsView';
import { TotpSettingsView } from './TotpSettingsView';
import { type MeSection } from './mobile-workspace-utils';
import './me-view.css';

type MeViewProps = {
  actor: AuthActor;
  activityEvents: AuditEvent[];
  baseCurrency: string;
  isActivityLoading: boolean;
  isLoggingOut: boolean;
  isProfileSaving: boolean;
  meSection: MeSection;
  onBack: () => void;
  onLoadActivity: () => void;
  onLogout: () => void;
  onOpenImports: () => void;
  onOpenProfile: () => void;
  onOpenSecurity: () => void;
  onUpdateBaseCurrency: (currency: string) => void;
  runtimeConfig: RuntimeConfig | null;
};

// MeView renders the personal account tab as a compact settings index with drill-in Profile and Security pages.
export function MeView({
  actor,
  activityEvents,
  baseCurrency,
  isActivityLoading,
  isLoggingOut,
  isProfileSaving,
  meSection,
  onBack,
  onLoadActivity,
  onLogout,
  onOpenImports,
  onOpenProfile,
  onOpenSecurity,
  onUpdateBaseCurrency,
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

  if (meSection === 'security') {
    return (
      <section className="meScreen" aria-label={t('mobile.me.securitySettings')}>
        <MeSubpageHeader title={t('mobile.me.securitySettings')} onBack={onBack} />

        <section className="meGroup">
          <h2 className="meGroupHeader">{t('mobile.me.groupSecurity')}</h2>
          <PasswordSettingsView email={actor.email} />
          <PasskeySettingsView featureEnabled={config.features.passkeyEnabled} />
          <TotpSettingsView featureEnabled={config.features.totpEnabled} />
          {!securityAvailable ? <p className="meEmpty">{t('mobile.me.emailPasswordOnly')}</p> : null}
        </section>
      </section>
    );
  }

  if (meSection === 'profile') {
    return (
      <section className="meScreen" aria-label={t('mobile.me.profileSettings')}>
        <MeSubpageHeader title={t('mobile.me.profileSettings')} onBack={onBack} />
        <MeIdentity
          actor={actor}
          monogram={monogram}
          statusIsActive={statusIsActive}
          uidCopied={uidCopied}
          onCopyUid={handleCopyUid}
        />

        <section className="meGroup">
          <h2 className="meGroupHeader">{t('mobile.me.groupActivity')}</h2>
          <div className="meCard meActivityCard">
            <button
              className="mobileSecondaryButton"
              type="button"
              disabled={isActivityLoading}
              onClick={onLoadActivity}
            >
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
          <label className="mePreferenceRow">
            <span>
              <b>{t('mobile.me.primaryCurrency')}</b>
              <small>{t('mobile.me.primaryCurrencyHint')}</small>
            </span>
            <select
              value={baseCurrency}
              disabled={isProfileSaving}
              onChange={(event) => onUpdateBaseCurrency(event.target.value)}
            >
              {supportedCurrencies.map((currency) => (
                <option key={currency} value={currency}>
                  {currency}
                </option>
              ))}
            </select>
          </label>
          <LanguageSelector className="meLanguageRow" />
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

  return (
    <section className="meScreen meScreenCompact" aria-label={t('mobile.nav.me')}>
      <nav className="meIndex" aria-label={t('mobile.nav.me')}>
        <MeIndexButton
          icon={<UserRound size={19} />}
          title={t('mobile.me.profileSettings')}
          hint={t('mobile.me.profileHint')}
          onClick={onOpenProfile}
        />
        <MeIndexButton
          icon={<ShieldCheck size={19} />}
          title={t('mobile.me.securitySettings')}
          hint={t('mobile.me.securityHint')}
          onClick={onOpenSecurity}
        />
        <MeIndexButton
          icon={<FileSpreadsheet size={19} />}
          title={t('mobile.me.importSettings')}
          hint={t('mobile.me.importHint')}
          onClick={onOpenImports}
        />
      </nav>
    </section>
  );
}

// MeSubpageHeader receives a subpage title and returns the local back navigation row.
function MeSubpageHeader({ title, onBack }: { title: string; onBack: () => void }) {
  const { t } = useTranslation();

  return (
    <header className="meSubpageHeader">
      <button type="button" className="meBackButton" aria-label={t('mobile.me.backToMe')} onClick={onBack}>
        <ChevronLeft size={20} aria-hidden="true" />
      </button>
      <h1>{title}</h1>
    </header>
  );
}

// MeIndexButton receives one settings destination and returns a compact index row.
function MeIndexButton({
  hint,
  icon,
  onClick,
  title,
}: {
  hint: string;
  icon: ReactNode;
  onClick: () => void;
  title: string;
}) {
  return (
    <button type="button" className="meNavRow meIndexButton" onClick={onClick}>
      <span className="meNavIcon" aria-hidden="true">
        {icon}
      </span>
      <span className="meNavLabel">
        <b>{title}</b>
        <small>{hint}</small>
      </span>
      <ChevronRight className="meNavChevron" size={18} aria-hidden="true" />
    </button>
  );
}

// MeIdentity receives actor metadata and returns the profile identity card.
function MeIdentity({
  actor,
  monogram,
  onCopyUid,
  statusIsActive,
  uidCopied,
}: {
  actor: AuthActor;
  monogram: string;
  onCopyUid: () => void;
  statusIsActive: boolean;
  uidCopied: boolean;
}) {
  const { t } = useTranslation();

  return (
    <header className="meIdentity">
      <span className="meAvatar" aria-hidden="true">
        {monogram}
      </span>
      <div className="meIdentityMain">
        <strong className="meEmail" dir="auto">
          {actor.email}
        </strong>
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
        <button type="button" className="meCopyButton" aria-label={t('mobile.me.copyUid')} onClick={onCopyUid}>
          {uidCopied ? <Check size={15} aria-hidden="true" /> : <Copy size={15} aria-hidden="true" />}
          <span role="status" aria-live="polite">
            {uidCopied ? t('mobile.me.uidCopied') : t('mobile.me.copy')}
          </span>
        </button>
      </div>
    </header>
  );
}

// PasswordSettingsView receives the signed-in email and renders password reset controls.
function PasswordSettingsView({ email }: { email: string }) {
  const { t } = useTranslation();
  const [resetCode, setResetCode] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);
  const canConfirm = Boolean(resetCode.trim()) && Boolean(newPassword.trim()) && !isBusy;

  // handleRequestReset receives no parameters and requests a password reset code for the signed-in account.
  async function handleRequestReset() {
    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      await requestPasswordReset(email);
      setStatus(t('auth.status.resetRequested'));
    } catch {
      setError(t('auth.error.recoveryFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleConfirmReset receives a form submit event and confirms the new password with the reset code.
  async function handleConfirmReset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canConfirm) {
      return;
    }

    setIsBusy(true);
    setStatus('');
    setError('');
    try {
      await confirmPasswordReset(email, resetCode.trim(), newPassword);
      setResetCode('');
      setNewPassword('');
      setStatus(t('auth.status.passwordUpdated'));
    } catch {
      setError(t('auth.error.recoveryFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  return (
    <article className="settingsPanel passwordSettings" aria-label={t('mobile.me.passwordSettings')}>
      <header>
        <span className="settingsPanelIcon" aria-hidden="true">
          <LockKeyhole size={18} />
        </span>
        <div>
          <div className="settingsPanelTitle">
            <strong>{t('mobile.me.passwordSettings')}</strong>
          </div>
          <span>{t('mobile.me.passwordHint')}</span>
        </div>
      </header>
      <button className="mobileSecondaryButton" type="button" disabled={isBusy} onClick={handleRequestReset}>
        {t('auth.submit.sendResetEmail')}
      </button>
      <form className="passwordResetForm" onSubmit={handleConfirmReset}>
        <label className="mobileField">
          <span>{t('auth.fields.resetCode')}</span>
          <input
            type="text"
            value={resetCode}
            inputMode="numeric"
            autoComplete="one-time-code"
            onChange={(event) => setResetCode(event.target.value)}
          />
        </label>
        <label className="mobileField">
          <span>{t('auth.fields.newPassword')}</span>
          <input
            type="password"
            value={newPassword}
            autoComplete="new-password"
            onChange={(event) => setNewPassword(event.target.value)}
          />
        </label>
        <button className="mobilePrimaryButton" type="submit" disabled={!canConfirm}>
          {t('auth.submit.resetPassword')}
        </button>
      </form>
      {error ? <p className="mobileInlineError">{error}</p> : null}
      {status ? <p className="mobileInlineStatus">{status}</p> : null}
    </article>
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
