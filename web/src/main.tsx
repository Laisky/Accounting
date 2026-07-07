import { StrictMode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router';
import { App } from './App';
import { AppErrorBoundary } from './components/AppErrorBoundary';
import './i18n';
import { queryClient } from './lib/queryClient';
import { installTelemetry } from './lib/telemetry';
import { installVisualViewportHeightVar } from './lib/visualViewport';
import './styles/layers.css';
import './styles/palette.css';
import './styles/tokens.css';
import './styles.css';

installVisualViewportHeightVar();
installTelemetry();

// Register the static-asset service worker in production so the app is installable.
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    void navigator.serviceWorker.register('/sw.js').catch(() => {
      // Service worker registration is best-effort; the app works without it.
    });
  });
}

const root = createRoot(document.getElementById('root') as HTMLElement);
root.render(
  <StrictMode>
    <AppErrorBoundary label="app">
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </AppErrorBoundary>
  </StrictMode>,
);
