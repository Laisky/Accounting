import { KeyRound, Pencil, Trash2 } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { type FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { passkeyListQueryKey, usePasskeysQuery } from '@/hooks/usePasskeys';
import {
  beginPasskeyRegistration,
  deletePasskey,
  finishPasskeyRegistration,
  updatePasskey,
  type PasskeyList,
  type PasskeyListItem,
} from '@/lib/api/auth';
import { credentialCreationOptionsFromJSON, isWebAuthnAvailable, publicKeyCredentialToJSON } from '@/lib/webauthn';
import './passkey-settings.css';

type PasskeySettingsViewProps = {
  featureEnabled: boolean;
};

// PasskeySettingsView receives runtime feature state and renders signed-in passkey management controls.
export function PasskeySettingsView({ featureEnabled }: PasskeySettingsViewProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const passkeysQuery = usePasskeysQuery(featureEnabled);
  const passkeys = passkeysQuery.data?.items ?? [];
  const [labelDrafts, setLabelDrafts] = useState<Record<string, string>>({});
  const [newLabel, setNewLabel] = useState(t('mobile.me.defaultPasskeyLabel'));
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);

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
      const passkey = await finishPasskeyRegistration(
        start.flowId,
        newLabel.trim(),
        publicKeyCredentialToJSON(credential as PublicKeyCredential),
      );
      queryClient.setQueryData<PasskeyList>(passkeyListQueryKey, (current) =>
        passkeyPageWith([passkey, ...(current?.items ?? []).filter((item) => item.id !== passkey.id)], current),
      );
      setLabelDrafts((current) => withoutDraft(current, passkey.id));
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
      const existing = passkeys.find((passkey) => passkey.id === passkeyId);
      const nextLabel = (labelDrafts[passkeyId] ?? existing?.label ?? '').trim();
      const passkey = await updatePasskey(passkeyId, nextLabel);
      queryClient.setQueryData<PasskeyList>(passkeyListQueryKey, (current) =>
        passkeyPageWith(
          (current?.items ?? passkeys).map((item) => (item.id === passkey.id ? passkey : item)),
          current,
        ),
      );
      setLabelDrafts((current) => withoutDraft(current, passkey.id));
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
      queryClient.setQueryData<PasskeyList>(passkeyListQueryKey, (current) =>
        passkeyPageWith(
          (current?.items ?? passkeys).filter((item) => item.id !== passkeyId),
          current,
        ),
      );
      setLabelDrafts((current) => withoutDraft(current, passkeyId));
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
    <article className="passkeySettings settingsPanel" aria-label={t('mobile.me.passkeySettings')}>
      <header>
        <span className="settingsPanelIcon" aria-hidden="true">
          <KeyRound size={18} />
        </span>
        <div>
          <div className="settingsPanelTitle">
            <strong>{t('mobile.me.passkeySettings')}</strong>
            <span className="mePill mePillAccent">{t('mobile.me.passkeyRecommended')}</span>
          </div>
          <span>{t('mobile.me.passkeyCount', { count: passkeys.length })}</span>
        </div>
      </header>
      <p className="settingsPanelHint">{t('mobile.me.passkeyHint')}</p>

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
                  value={labelDrafts[passkey.id] ?? passkey.label}
                  maxLength={80}
                  onChange={(event) => setLabelDrafts((current) => ({ ...current, [passkey.id]: event.target.value }))}
                />
              </label>
              <div className="passkeyMeta">
                <span>
                  {passkey.transports.length ? passkey.transports.join(', ') : t('mobile.me.passkeyTransportFallback')}
                </span>
                <time dateTime={passkey.createdAt}>{formatPasskeyTime(passkey.createdAt)}</time>
              </div>
              <div className="passkeyActions">
                <button
                  className="mobileSecondaryButton"
                  type="button"
                  disabled={isBusy || !(labelDrafts[passkey.id] ?? passkey.label).trim()}
                  onClick={() => void handleRename(passkey.id)}
                >
                  <Pencil size={16} />
                  <span>{t('mobile.me.renamePasskey')}</span>
                </button>
                <button
                  className="mobileDangerButton"
                  type="button"
                  disabled={isBusy}
                  onClick={() => void handleDelete(passkey.id)}
                >
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

      {passkeysQuery.isError ? <p className="mobileInlineError">{t('mobile.error.passkeysFailed')}</p> : null}
      {error ? <p className="mobileInlineError">{error}</p> : null}
      {status ? <p className="mobileInlineStatus">{status}</p> : null}
    </article>
  );
}

// passkeyPageWith receives passkey metadata and returns a normalized passkey list page.
function passkeyPageWith(items: PasskeyListItem[], current?: PasskeyList): PasskeyList {
  return {
    items,
    page: current?.page ?? 1,
    pageSize: current?.pageSize ?? 20,
    total: items.length,
  };
}

// withoutDraft receives label drafts and removes one passkey draft value.
function withoutDraft(drafts: Record<string, string>, passkeyId: string): Record<string, string> {
  const next = { ...drafts };
  delete next[passkeyId];
  return next;
}

// formatPasskeyTime receives an ISO timestamp and returns a compact UTC string.
function formatPasskeyTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toISOString().replace('.000Z', 'Z');
}
