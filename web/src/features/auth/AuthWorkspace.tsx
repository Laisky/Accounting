import type { TFunction } from 'i18next';
import { KeyRound, LogIn, Mail, ShieldCheck, UserPlus } from 'lucide-react';
import { type FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  beginPasskeyLogin,
  confirmPasswordReset,
  loginWithPassword,
  registerWithPassword,
  requestEmailVerification,
  requestPasswordReset,
  type AuthActor,
} from '../../lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from '../../lib/api/runtimeConfig';

type AuthMode = 'login' | 'register' | 'recover';
type RecoveryStep = 'request' | 'confirm';
type LoginStep = 'password' | 'totp';

type AuthWorkspaceProps = {
  runtimeConfig: RuntimeConfig | null;
  onAuthenticated: (actor: AuthActor) => void;
};

// AuthWorkspace renders first-class authentication and account recovery workflows.
export function AuthWorkspace({ runtimeConfig, onAuthenticated }: AuthWorkspaceProps) {
  const { t } = useTranslation();
  const config = runtimeConfig ?? emptyRuntimeConfig;
  const [mode, setMode] = useState<AuthMode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTOTPCode] = useState('');
  const [loginStep, setLoginStep] = useState<LoginStep>('password');
  const [resetCode, setResetCode] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [recoveryStep, setRecoveryStep] = useState<RecoveryStep>('request');
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);

  // handleSubmit receives a form submit event and runs the selected authentication workflow.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsBusy(true);
    setError('');
    setStatus('');

    try {
      if (mode === 'login') {
        const result = await loginWithPassword(email, password, loginStep === 'totp' ? totpCode : undefined);
        if (result.kind === 'totpRequired') {
          // Password verified and the account has TOTP enabled; reveal the code step.
          setLoginStep('totp');
          setStatus(t('auth.status.totpRequired'));
          return;
        }
        onAuthenticated({
          userId: result.user.id,
          email: result.user.email,
          status: result.user.status,
        });
        return;
      }

      if (mode === 'register') {
        const result = await registerWithPassword(email, password);
        if (config.auth.emailVerificationRequired) {
          await requestEmailVerification(email);
          setStatus(t('auth.status.verificationRequested'));
        } else {
          setStatus(t('auth.status.registrationComplete'));
        }
        setPassword('');
        if (result.user.status === 'active' && !config.auth.emailVerificationRequired) {
          setMode('login');
        }
        return;
      }

      if (recoveryStep === 'request') {
        await requestPasswordReset(email);
        setRecoveryStep('confirm');
        setStatus(t('auth.status.resetRequested'));
        return;
      }

      await confirmPasswordReset(email, resetCode, newPassword);
      setMode('login');
      setRecoveryStep('request');
      setPassword('');
      setResetCode('');
      setNewPassword('');
      setStatus(t('auth.status.passwordUpdated'));
    } catch {
      setError(authErrorText(t, mode));
    } finally {
      setIsBusy(false);
    }
  }

  // handlePasskeyLogin receives no parameters and starts a passkey login ceremony.
  async function handlePasskeyLogin() {
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      const start = await beginPasskeyLogin();
      setStatus(t('auth.status.passkeyChallengeReady', { flowId: start.flowId.slice(0, 8) }));
    } catch {
      setError(t('auth.error.passkeyUnavailable'));
    } finally {
      setIsBusy(false);
    }
  }

  // changeMode receives a target auth mode and resets any in-progress login challenge and messages.
  function changeMode(next: AuthMode) {
    setMode(next);
    setLoginStep('password');
    setTOTPCode('');
    setError('');
    setStatus('');
  }

  // resetLoginChallenge receives no parameters and clears a pending TOTP step after credentials change.
  function resetLoginChallenge() {
    if (loginStep !== 'password') {
      setLoginStep('password');
      setTOTPCode('');
      setStatus('');
      setError('');
    }
  }

  return (
    <main className="shell authShell">
      <section className="authLayout" aria-label={t('auth.a11y.authentication')}>
        <div className="authBrief">
          <p className="eyebrow">{t('common.accessEyebrow')}</p>
          <h1>{t('auth.heading')}</h1>
          <div className="authMethods" aria-label={t('auth.a11y.availableLoginMethods')}>
            {config.auth.emailLoginEnabled ? <span>{t('auth.methods.emailLogin')}</span> : null}
            {config.auth.emailRegisterEnabled ? <span>{t('auth.methods.registrationOpen')}</span> : <span>{t('auth.methods.registrationClosed')}</span>}
            {config.features.externalSsoEnabled ? <span>{t('auth.methods.externalSso')}</span> : null}
            {config.features.passkeyEnabled ? <span>{t('auth.methods.passkeys')}</span> : null}
            {config.features.totpEnabled ? <span>{t('auth.methods.totp')}</span> : null}
          </div>
        </div>

        <div className="authPanel">
          <div className="authTabs" role="tablist" aria-label={t('auth.a11y.authenticationMode')}>
            <button type="button" className={mode === 'login' ? 'authTabActive' : ''} onClick={() => changeMode('login')}>
              <LogIn size={16} />
              <span>{t('auth.tabs.signIn')}</span>
            </button>
            <button
              type="button"
              className={mode === 'register' ? 'authTabActive' : ''}
              onClick={() => changeMode('register')}
              disabled={!config.auth.emailRegisterEnabled}
            >
              <UserPlus size={16} />
              <span>{t('auth.tabs.register')}</span>
            </button>
            <button
              type="button"
              className={mode === 'recover' ? 'authTabActive' : ''}
              onClick={() => changeMode('recover')}
            >
              <Mail size={16} />
              <span>{t('auth.tabs.recover')}</span>
            </button>
          </div>

          <form className="authForm" onSubmit={handleSubmit}>
            <label>
              <span>{t('auth.fields.email')}</span>
              <input
                type="email"
                value={email}
                autoComplete="email"
                required
                onChange={(event) => {
                  setEmail(event.target.value);
                  resetLoginChallenge();
                }}
              />
            </label>

            {mode !== 'recover' ? (
              <label>
                <span>{t('auth.fields.password')}</span>
                <input
                  type="password"
                  value={password}
                  autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                  required
                  minLength={12}
                  onChange={(event) => {
                    setPassword(event.target.value);
                    resetLoginChallenge();
                  }}
                />
              </label>
            ) : null}

            {mode === 'login' && loginStep === 'totp' ? (
              <label>
                <span>{t('auth.fields.totpCode')}</span>
                <input
                  type="text"
                  value={totpCode}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  required
                  autoFocus
                  onChange={(event) => setTOTPCode(event.target.value)}
                />
              </label>
            ) : null}

            {mode === 'recover' && recoveryStep === 'confirm' ? (
              <>
                <label>
                  <span>{t('auth.fields.resetCode')}</span>
                  <input
                    type="text"
                    value={resetCode}
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    required
                    onChange={(event) => setResetCode(event.target.value)}
                  />
                </label>
                <label>
                  <span>{t('auth.fields.newPassword')}</span>
                  <input
                    type="password"
                    value={newPassword}
                    autoComplete="new-password"
                    required
                    minLength={12}
                    onChange={(event) => setNewPassword(event.target.value)}
                  />
                </label>
              </>
            ) : null}

            {error ? <p className="authError">{error}</p> : null}
            {status ? <p className="authStatus">{status}</p> : null}

            <button className="primaryButton" type="submit" disabled={isBusy || (mode === 'login' && !config.auth.emailLoginEnabled)}>
              <span>{submitLabel(t, mode, recoveryStep, loginStep, isBusy)}</span>
            </button>
          </form>

          {config.features.passkeyEnabled ? (
            <button className="secondaryAuthButton" type="button" disabled={isBusy} onClick={handlePasskeyLogin}>
              <KeyRound size={17} />
              <span>{t('auth.actions.usePasskey')}</span>
            </button>
          ) : null}
          {config.features.externalSsoEnabled && config.sso.startPath ? (
            <a className="secondaryAuthButton" href={config.sso.startPath} aria-disabled={isBusy}>
              <ShieldCheck size={17} />
              <span>{t('auth.actions.useSso')}</span>
            </a>
          ) : null}
        </div>
      </section>
    </main>
  );
}

// submitLabel receives the translator, current auth mode, recovery step, login step, and busy state and returns button copy.
function submitLabel(t: TFunction, mode: AuthMode, recoveryStep: RecoveryStep, loginStep: LoginStep, isBusy: boolean): string {
  if (isBusy) {
    return t('auth.submit.working');
  }
  if (mode === 'register') {
    return t('auth.submit.createAccount');
  }
  if (mode === 'recover') {
    return recoveryStep === 'request' ? t('auth.submit.sendResetEmail') : t('auth.submit.resetPassword');
  }
  if (mode === 'login' && loginStep === 'totp') {
    return t('auth.submit.verifyTotp');
  }

  return t('auth.submit.signInWithEmail');
}

// authErrorText receives the translator and current auth mode and returns a stable non-enumerating error message.
function authErrorText(t: TFunction, mode: AuthMode): string {
  if (mode === 'register') {
    return t('auth.error.registrationFailed');
  }
  if (mode === 'recover') {
    return t('auth.error.recoveryFailed');
  }

  return t('auth.error.signInFailed');
}
