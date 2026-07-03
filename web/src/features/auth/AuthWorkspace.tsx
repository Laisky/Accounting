import type { TFunction } from 'i18next';
import { KeyRound, LogIn, Mail, ShieldCheck, UserPlus } from 'lucide-react';
import { type FormEvent, useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router';
import {
  beginPasskeyLogin,
  confirmEmailVerification,
  confirmPasswordReset,
  finishPasskeyLogin,
  loginWithPassword,
  registerWithPassword,
  requestEmailVerification,
  requestPasswordReset,
  type AuthActor,
} from '../../lib/api/auth';
import { emptyRuntimeConfig, type RuntimeConfig } from '../../lib/api/runtimeConfig';
import { credentialRequestOptionsFromJSON, isWebAuthnAvailable, publicKeyCredentialToJSON } from '../../lib/webauthn';
import { TurnstileWidget } from './TurnstileWidget';
import './auth.css';

type AuthMode = 'login' | 'register' | 'recover';
type RecoveryStep = 'request' | 'confirm';
type LoginStep = 'password' | 'totp';
type VerificationStep = 'credentials' | 'confirm';

type AuthWorkspaceProps = {
  runtimeConfig: RuntimeConfig | null;
  onAuthenticated: (actor: AuthActor) => void;
};

// AuthWorkspace renders first-class authentication and account recovery workflows.
export function AuthWorkspace({ runtimeConfig, onAuthenticated }: AuthWorkspaceProps) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const config = runtimeConfig ?? emptyRuntimeConfig;
  // mode and every sub-step live in the URL so each auth screen has its own address
  // and the browser back button walks the steps. See parseAuthRoute / authPathFor.
  const { mode, loginStep, verificationStep, recoveryStep } = parseAuthRoute(location.pathname);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTOTPCode] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [resetCode, setResetCode] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [isBusy, setIsBusy] = useState(false);
  const [turnstileToken, setTurnstileToken] = useState('');
  const [turnstileResetKey, setTurnstileResetKey] = useState(0);
  const [loginTurnstileRequired, setLoginTurnstileRequired] = useState(false);
  const turnstileRequired = shouldRequireTurnstile(config, mode, verificationStep, loginTurnstileRequired);
  const canSubmit = !isBusy && !(mode === 'login' && !config.auth.emailLoginEnabled) && (!turnstileRequired || Boolean(turnstileToken));

  useEffect(() => {
    // Steps past the first hold only in-memory state (a server-verified password, a
    // pending email challenge). On a cold direct load that state is gone, so send the
    // visitor back to step one instead of a dead-end form.
    if (loginStep === 'totp' && !password) {
      navigate('/login', { replace: true });
    } else if (verificationStep === 'confirm' && !email) {
      navigate('/register', { replace: true });
    } else if (recoveryStep === 'confirm' && !email) {
      navigate('/recover', { replace: true });
    }
  }, [loginStep, verificationStep, recoveryStep, password, email, navigate]);

  // handleSubmit receives a form submit event and runs the selected authentication workflow.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsBusy(true);
    setError('');
    setStatus('');
    const currentTurnstileToken = turnstileRequired ? turnstileToken : undefined;
    if (turnstileRequired && !currentTurnstileToken) {
      setError(t('auth.error.turnstileRequired'));
      setIsBusy(false);
      return;
    }

    try {
      if (mode === 'login') {
        const result = await loginWithPassword(email, password, loginStep === 'totp' ? totpCode : undefined, currentTurnstileToken);
        if (result.kind === 'totpRequired') {
          // Password verified and the account has TOTP enabled; advance to the code step.
          setStatus(t('auth.status.totpRequired'));
          navigate('/login/totp');
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
        if (verificationStep === 'confirm') {
          await confirmEmailVerification(email, verificationCode);
          setVerificationCode('');
          setPassword('');
          setStatus(t('auth.status.emailVerified'));
          navigate('/login');
          return;
        }

        const result = await registerWithPassword(email, password, currentTurnstileToken);
        setPassword('');
        if (config.auth.emailVerificationRequired) {
          await requestEmailVerification(email);
          setVerificationCode('');
          setStatus(t('auth.status.verificationRequested'));
          navigate('/register/verify');
        } else {
          setStatus(t('auth.status.registrationComplete'));
          if (result.user.status === 'active') {
            navigate('/login');
          }
        }
        return;
      }

      if (recoveryStep === 'request') {
        await requestPasswordReset(email);
        setStatus(t('auth.status.resetRequested'));
        navigate('/recover/confirm');
        return;
      }

      await confirmPasswordReset(email, resetCode, newPassword);
      setPassword('');
      setResetCode('');
      setNewPassword('');
      setStatus(t('auth.status.passwordUpdated'));
      navigate('/login');
    } catch {
      if (mode === 'login' && config.features.turnstileEnabled && config.turnstile.loginMode === 'after_failure') {
        setLoginTurnstileRequired(true);
      }
      setError(authErrorText(t, mode, verificationStep));
    } finally {
      if (turnstileRequired) {
        resetTurnstileChallenge();
      }
      setIsBusy(false);
    }
  }

  // handlePasskeyLogin receives no parameters and completes a passkey login ceremony.
  async function handlePasskeyLogin() {
    setIsBusy(true);
    setError('');
    setStatus('');
    try {
      if (!isWebAuthnAvailable()) {
        throw new Error('WebAuthn unavailable');
      }
      const start = await beginPasskeyLogin();
      const credential = await navigator.credentials.get(credentialRequestOptionsFromJSON(start.options));
      if (!credential) {
        throw new Error('WebAuthn credential is required');
      }
      const result = await finishPasskeyLogin(start.flowId, publicKeyCredentialToJSON(credential as PublicKeyCredential));
      onAuthenticated({
        userId: result.user.id,
        email: result.user.email,
        status: result.user.status,
      });
    } catch {
      setError(t('auth.error.passkeyUnavailable'));
    } finally {
      setIsBusy(false);
    }
  }

  // changeMode receives a target auth mode, resets any in-progress challenge, and routes to it.
  function changeMode(next: AuthMode) {
    setTOTPCode('');
    setVerificationCode('');
    setResetCode('');
    setNewPassword('');
    setLoginTurnstileRequired(false);
    resetTurnstileChallenge();
    setError('');
    setStatus('');
    navigate(next === 'login' ? '/login' : `/${next}`);
  }

  // resetLoginChallenge receives no parameters and routes back from a pending TOTP step after credentials change.
  function resetLoginChallenge() {
    if (loginStep !== 'password') {
      setTOTPCode('');
      setStatus('');
      setError('');
      navigate('/login');
    }
  }

  // handleTurnstileToken receives a browser challenge token and stores it for the next protected submit.
  const handleTurnstileToken = useCallback((token: string) => {
    setTurnstileToken(token);
  }, []);

  // handleTurnstileExpire receives no parameters and clears stale Turnstile tokens.
  const handleTurnstileExpire = useCallback(() => {
    setTurnstileToken('');
  }, []);

  // handleTurnstileError receives no parameters and surfaces a generic challenge failure.
  const handleTurnstileError = useCallback(() => {
    setError(t('auth.error.turnstileFailed'));
  }, [t]);

  // resetTurnstileChallenge receives no parameters and requests a fresh Turnstile token.
  function resetTurnstileChallenge() {
    setTurnstileToken('');
    setTurnstileResetKey((value) => value + 1);
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

            {mode !== 'recover' && verificationStep !== 'confirm' ? (
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

            {mode === 'register' && verificationStep === 'confirm' ? (
              <label>
                <span>{t('auth.fields.verificationCode')}</span>
                <input
                  type="text"
                  value={verificationCode}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  required
                  autoFocus
                  onChange={(event) => setVerificationCode(event.target.value)}
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

            {turnstileRequired && config.turnstile.siteKey ? (
              <TurnstileWidget
                siteKey={config.turnstile.siteKey}
                resetKey={turnstileResetKey}
                onToken={handleTurnstileToken}
                onExpire={handleTurnstileExpire}
                onError={handleTurnstileError}
              />
            ) : null}

            {error ? <p className="authError">{error}</p> : null}
            {status ? <p className="authStatus">{status}</p> : null}

            <button className="primaryButton" type="submit" disabled={!canSubmit}>
              <span>{submitLabel(t, mode, recoveryStep, loginStep, verificationStep, isBusy)}</span>
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

// parseAuthRoute maps an unauthenticated URL to the mode and step it represents.
function parseAuthRoute(pathname: string): {
  mode: AuthMode;
  loginStep: LoginStep;
  verificationStep: VerificationStep;
  recoveryStep: RecoveryStep;
} {
  switch (pathname) {
    case '/register':
      return { mode: 'register', loginStep: 'password', verificationStep: 'credentials', recoveryStep: 'request' };
    case '/register/verify':
      return { mode: 'register', loginStep: 'password', verificationStep: 'confirm', recoveryStep: 'request' };
    case '/recover':
      return { mode: 'recover', loginStep: 'password', verificationStep: 'credentials', recoveryStep: 'request' };
    case '/recover/confirm':
      return { mode: 'recover', loginStep: 'password', verificationStep: 'credentials', recoveryStep: 'confirm' };
    case '/login/totp':
      return { mode: 'login', loginStep: 'totp', verificationStep: 'credentials', recoveryStep: 'request' };
    default:
      return { mode: 'login', loginStep: 'password', verificationStep: 'credentials', recoveryStep: 'request' };
  }
}

// submitLabel receives the translator and auth state and returns button copy for the current form step.
function submitLabel(t: TFunction, mode: AuthMode, recoveryStep: RecoveryStep, loginStep: LoginStep, verificationStep: VerificationStep, isBusy: boolean): string {
  if (isBusy) {
    return t('auth.submit.working');
  }
  if (mode === 'register') {
    if (verificationStep === 'confirm') {
      return t('auth.submit.verifyEmail');
    }
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

// authErrorText receives the translator and auth state and returns a stable non-enumerating error message.
function authErrorText(t: TFunction, mode: AuthMode, verificationStep: VerificationStep): string {
  if (mode === 'register') {
    if (verificationStep === 'confirm') {
      return t('auth.error.verificationFailed');
    }
    return t('auth.error.registrationFailed');
  }
  if (mode === 'recover') {
    return t('auth.error.recoveryFailed');
  }

  return t('auth.error.signInFailed');
}

// shouldRequireTurnstile receives runtime auth state and returns whether the current submit needs a challenge token.
function shouldRequireTurnstile(config: RuntimeConfig, mode: AuthMode, verificationStep: VerificationStep, loginTurnstileRequired: boolean): boolean {
  if (!config.features.turnstileEnabled) {
    return false;
  }
  if (mode === 'register') {
    return verificationStep !== 'confirm';
  }
  if (mode === 'login') {
    return config.turnstile.loginMode !== 'after_failure' || loginTurnstileRequired;
  }

  return false;
}
