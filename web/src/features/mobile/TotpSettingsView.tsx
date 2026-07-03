import { Check, Copy, ShieldCheck } from 'lucide-react';
import { type FormEvent, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { confirmTotp, disableTotp, fetchTotpStatus, setupTotp } from '../../lib/api/auth';
import { copyToClipboard } from '../../lib/clipboard';
import './totp-settings.css';

type TotpSettingsViewProps = {
  featureEnabled: boolean;
};

// TotpSettingsView receives runtime feature state and renders signed-in TOTP setup and disable controls.
export function TotpSettingsView({ featureEnabled }: TotpSettingsViewProps) {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState(false);
  const [setupURI, setSetupURI] = useState('');
  const [setupCode, setSetupCode] = useState('');
  const [disableCode, setDisableCode] = useState('');
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);
  const [keyCopied, setKeyCopied] = useState(false);

  // handleCopySetupKey receives no parameters and copies the authenticator secret to the clipboard
  // (the camera-less enrollment path) with brief confirmation.
  async function handleCopySetupKey() {
    const copied = await copyToClipboard(extractTotpSecret(setupURI));
    if (!copied) {
      return;
    }
    setKeyCopied(true);
    window.setTimeout(() => setKeyCopied(false), 2000);
  }

  useEffect(() => {
    if (!featureEnabled) {
      return;
    }

    const controller = new AbortController();
    fetchTotpStatus(controller.signal)
      .then((nextStatus) => setEnabled(nextStatus.enabled))
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === 'AbortError') {
          return;
        }
        setError(t('mobile.error.totpStatusFailed'));
      });

    return () => controller.abort();
  }, [featureEnabled, t]);

  // handleStartSetup receives no parameters and starts a pending TOTP enrollment.
  async function handleStartSetup() {
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      const setup = await setupTotp();
      setSetupURI(setup.otpauth);
      setSetupCode('');
      setStatus(t('mobile.status.totpSetupStarted'));
    } catch {
      setError(t('mobile.error.totpSetupFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleConfirm receives a form submit event and confirms the pending TOTP enrollment.
  async function handleConfirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      const nextStatus = await confirmTotp(setupCode);
      setEnabled(nextStatus.enabled);
      setSetupURI('');
      setSetupCode('');
      setStatus(t('mobile.status.totpEnabled'));
    } catch {
      setError(t('mobile.error.totpConfirmFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleDisable receives a form submit event and disables TOTP for the signed-in account.
  async function handleDisable(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      const nextStatus = await disableTotp(disableCode);
      setEnabled(nextStatus.enabled);
      setDisableCode('');
      setStatus(t('mobile.status.totpDisabled'));
    } catch {
      setError(t('mobile.error.totpDisableFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  if (!featureEnabled) {
    return null;
  }

  return (
    <article className="totpSettings settingsPanel" aria-label={t('mobile.me.totpSettings')}>
      <header>
        <span className="settingsPanelIcon" aria-hidden="true">
          <ShieldCheck size={18} />
        </span>
        <div>
          <div className="settingsPanelTitle">
            <strong>{t('mobile.me.totpSettings')}</strong>
          </div>
          <span className={`totpStatusLine ${enabled ? 'isOn' : 'isOff'}`}>
            {enabled ? t('mobile.me.totpEnabled') : t('mobile.me.totpDisabled')}
          </span>
        </div>
      </header>
      <p className="settingsPanelHint">{t('mobile.me.totpHint')}</p>

      {!enabled ? (
        <>
          <button className="mobileSecondaryButton" type="button" disabled={isBusy} onClick={handleStartSetup}>
            {t('mobile.me.startTotpSetup')}
          </button>
          {setupURI ? (
            <form className="totpForm" onSubmit={handleConfirm}>
              <div className="totpSetupKey">
                <label className="mobileField">
                  <span>{t('mobile.me.totpSetupUri')}</span>
                  <textarea readOnly value={setupURI} aria-label={t('mobile.me.totpSetupUri')} />
                </label>
                <button type="button" className="mobileSecondaryButton totpCopyKey" onClick={handleCopySetupKey}>
                  {keyCopied ? <Check size={15} aria-hidden="true" /> : <Copy size={15} aria-hidden="true" />}
                  <span role="status" aria-live="polite">
                    {keyCopied ? t('mobile.me.setupKeyCopied') : t('mobile.me.copySetupKey')}
                  </span>
                </button>
              </div>
              <label className="mobileField">
                <span>{t('mobile.me.totpCode')}</span>
                <input
                  type="text"
                  value={setupCode}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  required
                  onChange={(event) => setSetupCode(event.target.value)}
                />
              </label>
              <button className="mobilePrimaryButton" type="submit" disabled={isBusy || !setupCode.trim()}>
                {t('mobile.me.confirmTotp')}
              </button>
            </form>
          ) : null}
        </>
      ) : (
        <form className="totpForm" onSubmit={handleDisable}>
          <label className="mobileField">
            <span>{t('mobile.me.totpCode')}</span>
            <input
              type="text"
              value={disableCode}
              inputMode="numeric"
              autoComplete="one-time-code"
              required
              onChange={(event) => setDisableCode(event.target.value)}
            />
          </label>
          <button className="mobileDangerButton" type="submit" disabled={isBusy || !disableCode.trim()}>
            {t('mobile.me.disableTotp')}
          </button>
        </form>
      )}

      {error ? <p className="mobileInlineError">{error}</p> : null}
      {status ? <p className="mobileInlineStatus">{status}</p> : null}
    </article>
  );
}

// extractTotpSecret receives an otpauth URI and returns its base32 secret, falling back
// to the full URI when the secret cannot be parsed.
function extractTotpSecret(otpauth: string): string {
  try {
    return new URL(otpauth).searchParams.get('secret') ?? otpauth;
  } catch {
    return otpauth;
  }
}
