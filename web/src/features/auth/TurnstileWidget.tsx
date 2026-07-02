import { useEffect, useRef } from 'react';

type TurnstileAPI = {
  render: (
    container: HTMLElement,
    options: {
      sitekey: string;
      callback: (token: string) => void;
      'expired-callback': () => void;
      'error-callback': () => void;
    },
  ) => string;
  remove?: (widgetId: string) => void;
  reset?: (widgetId: string) => void;
};

declare global {
  // Window includes Cloudflare Turnstile after the explicit-render script loads.
  interface Window {
    turnstile?: TurnstileAPI;
  }
}

const turnstileScriptID = 'accounting-turnstile-script';
let turnstileLoadPromise: Promise<TurnstileAPI> | null = null;

type TurnstileWidgetProps = {
  siteKey: string;
  resetKey: number;
  onToken: (token: string) => void;
  onExpire: () => void;
  onError: () => void;
};

// TurnstileWidget renders Cloudflare Turnstile and reports challenge lifecycle events to the auth form.
export function TurnstileWidget({ siteKey, resetKey, onToken, onExpire, onError }: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const widgetIDRef = useRef<string | null>(null);

  useEffect(() => {
    let isCancelled = false;
    onExpire();

    loadTurnstile()
      .then((turnstile) => {
        if (isCancelled || !containerRef.current) {
          return;
        }
        if (widgetIDRef.current && turnstile.remove) {
          turnstile.remove(widgetIDRef.current);
          widgetIDRef.current = null;
        }
        containerRef.current.replaceChildren();
        widgetIDRef.current = turnstile.render(containerRef.current, {
          sitekey: siteKey,
          callback: (token) => {
            onToken(token);
          },
          'expired-callback': () => {
            onExpire();
          },
          'error-callback': () => {
            onExpire();
            onError();
          },
        });
      })
      .catch(() => {
        if (!isCancelled) {
          onExpire();
          onError();
        }
      });

    return () => {
      isCancelled = true;
      if (widgetIDRef.current && window.turnstile?.remove) {
        window.turnstile.remove(widgetIDRef.current);
        widgetIDRef.current = null;
      }
    };
  }, [onError, onExpire, onToken, resetKey, siteKey]);

  return (
    <div className="turnstileBox" aria-label="Turnstile challenge">
      <div ref={containerRef} />
    </div>
  );
}

// loadTurnstile receives no parameters and resolves the Cloudflare Turnstile browser API.
function loadTurnstile(): Promise<TurnstileAPI> {
  if (window.turnstile) {
    return Promise.resolve(window.turnstile);
  }
  if (turnstileLoadPromise) {
    return turnstileLoadPromise;
  }

  turnstileLoadPromise = new Promise((resolve, reject) => {
    const existing = document.getElementById(turnstileScriptID) as HTMLScriptElement | null;
    if (existing) {
      existing.addEventListener('load', () => resolveTurnstile(resolve, reject), { once: true });
      existing.addEventListener('error', () => reject(new Error('turnstile script failed')), { once: true });
      return;
    }

    const script = document.createElement('script');
    script.id = turnstileScriptID;
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
    script.async = true;
    script.defer = true;
    script.addEventListener('load', () => resolveTurnstile(resolve, reject), { once: true });
    script.addEventListener('error', () => reject(new Error('turnstile script failed')), { once: true });
    document.head.append(script);
  });

  return turnstileLoadPromise;
}

// resolveTurnstile receives promise callbacks and resolves only after the global Turnstile API exists.
function resolveTurnstile(resolve: (turnstile: TurnstileAPI) => void, reject: (reason?: unknown) => void): void {
  if (window.turnstile) {
    resolve(window.turnstile);
    return;
  }
  reject(new Error('turnstile unavailable'));
}
