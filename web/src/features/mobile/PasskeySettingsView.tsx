import { KeyRound, Pencil, Trash2 } from 'lucide-react';
import { type FormEvent, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  beginPasskeyRegistration,
  deletePasskey,
  fetchPasskeys,
  finishPasskeyRegistration,
  updatePasskey,
  type PasskeyListItem,
} from '../../lib/api/auth';
import { credentialCreationOptionsFromJSON, isWebAuthnAvailable, publicKeyCredentialToJSON } from '../../lib/webauthn';
import './passkey-settings.css';

type PasskeySettingsViewProps = {
  featureEnabled: boolean;
};

// PasskeySettingsView receives runtime feature state and renders signed-in passkey management controls.
export function PasskeySettingsView({ featureEnabled }: PasskeySettingsViewProps) {
  const { t } = useTranslation();
  const [passkeys, setPasskeys] = useState<PasskeyListItem[]>([]);
  const [labels, setLabels] = useState<Record<string, string>>({});
  const [newLabel, setNewLabel] = useState(t('mobile.me.defaultPasskeyLabel'));
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);

  useEffect(() => {
    if (!featureEnabled) {
      return;
    }

    const controller = new AbortController();
    fetchPasskeys(controller.signal)
      .then((page) => {
        const items = Array.isArray(page.items) ? page.items : [];
        setPasskeys(items);
        setLabels(passkeyLabels(items));
      })
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === 'AbortError') {
          return;
        }
        setError(t('mobile.error.passkeysFailed'));
      });

    return () => controller.abort();
  }, [featureEnabled, t]);

  // handleRegister receives a form submit event and registers a new browser passkey.
  async function handleRegister(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      if (!isWebAuthnAvailable()) {
        throw new Error('WebAuthn unavailable');
      }
      const start = await beginPasskeyRegistration();
      const credential = await navigator.credentials.create(credentialCreationOptionsFromJSON(start.options));
      if (!credential) {
        throw new Error('WebAuthn credential is required');
      }
      const passkey = await finishPasskeyRegistration(start.flowId, newLabel.trim(), publicKeyCredentialToJSON(credential as PublicKeyCredential));
      setPasskeys((current) => [passkey, ...current.filter((item) => item.id !== passkey.id)]);
      setLabels((current) => ({ ...current, [passkey.id]: passkey.label }));
      setNewLabel(t('mobile.me.defaultPasskeyLabel'));
      setStatus(t('mobile.status.passkeyRegistered'));
    } catch {
      setError(t('mobile.error.passkeyRegisterFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleRename receives a passkey id and updates its display label.
  async function handleRename(passkeyId: string) {
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      const nextLabel = labels[passkeyId]?.trim() ?? '';
      const passkey = await updatePasskey(passkeyId, nextLabel);
      setPasskeys((current) => current.map((item) => (item.id === passkey.id ? passkey : item)));
      setLabels((current) => ({ ...current, [passkey.id]: passkey.label }));
      setStatus(t('mobile.status.passkeyRenamed'));
    } catch {
      setError(t('mobile.error.passkeyRenameFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  // handleDelete receives a passkey id and removes it from the account.
  async function handleDelete(passkeyId: string) {
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      await deletePasskey(passkeyId);
      setPasskeys((current) => current.filter((item) => item.id !== passkeyId));
      setLabels((current) => {
        const next = { ...current };
        delete next[passkeyId];
        return next;
      });
      setStatus(t('mobile.status.passkeyDeleted'));
    } catch {
      setError(t('mobile.error.passkeyDeleteFailed'));
    } finally {
      setIsBusy(false);
    }
  }

  if (!featureEnabled) {
    return null;
  }

  return (
    <article className="passkeySettings" aria-label={t('mobile.me.passkeySettings')}>
      <header>
        <KeyRound size={18} />
        <div>
          <strong>{t('mobile.me.passkeySettings')}</strong>
          <span>{t('mobile.me.passkeyCount', { count: passkeys.length })}</span>
        </div>
      </header>

      <form className="passkeyForm" onSubmit={handleRegister}>
        <label className="mobileField">
          <span>{t('mobile.me.passkeyLabel')}</span>
          <input
            type="text"
            value={newLabel}
            maxLength={80}
            required
            onChange={(event) => setNewLabel(event.target.value)}
          />
        </label>
        <button className="mobilePrimaryButton" type="submit" disabled={isBusy || !newLabel.trim()}>
          {t('mobile.me.registerPasskey')}
        </button>
      </form>

      <ul className="passkeyList" aria-label={t('mobile.me.passkeyList')}>
        {passkeys.length ? (
          passkeys.map((passkey) => (
            <li key={passkey.id}>
              <label className="mobileField">
                <span>{t('mobile.me.passkeyLabelFor', { label: passkey.label })}</span>
                <input
                  type="text"
                  value={labels[passkey.id] ?? passkey.label}
                  maxLength={80}
                  onChange={(event) => setLabels((current) => ({ ...current, [passkey.id]: event.target.value }))}
                />
              </label>
              <div className="passkeyMeta">
                <span>{passkey.transports.length ? passkey.transports.join(', ') : t('mobile.me.passkeyTransportFallback')}</span>
                <time dateTime={passkey.createdAt}>{formatPasskeyTime(passkey.createdAt)}</time>
              </div>
              <div className="passkeyActions">
                <button
                  className="mobileSecondaryButton"
                  type="button"
                  disabled={isBusy || !(labels[passkey.id] ?? '').trim()}
                  onClick={() => void handleRename(passkey.id)}
                >
                  <Pencil size={16} />
                  <span>{t('mobile.me.renamePasskey')}</span>
                </button>
                <button className="mobileDangerButton" type="button" disabled={isBusy} onClick={() => void handleDelete(passkey.id)}>
                  <Trash2 size={16} />
                  <span>{t('mobile.me.deletePasskey')}</span>
                </button>
              </div>
            </li>
          ))
        ) : (
          <li className="passkeyEmpty">{t('mobile.me.noPasskeys')}</li>
        )}
      </ul>

      {error ? <p className="mobileInlineError">{error}</p> : null}
      {status ? <p className="mobileInlineStatus">{status}</p> : null}
    </article>
  );
}

// passkeyLabels receives passkey metadata and returns a lookup of editable labels.
function passkeyLabels(passkeys: PasskeyListItem[]): Record<string, string> {
  return Object.fromEntries(passkeys.map((passkey) => [passkey.id, passkey.label]));
}

// formatPasskeyTime receives an ISO timestamp and returns a compact UTC string.
function formatPasskeyTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toISOString().replace('.000Z', 'Z');
}
