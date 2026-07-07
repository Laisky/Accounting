import { Component, type ErrorInfo, type ReactNode } from 'react';
import { reportClientError } from '@/lib/telemetry';
import './app-error-boundary.css';

type AppErrorBoundaryProps = {
  children: ReactNode;
  // label distinguishes nested boundaries (e.g. "app" vs "route") in the fallback copy.
  label?: string;
};

type AppErrorBoundaryState = {
  eventId: string | null;
};

// AppErrorBoundary contains a render exception, reports a sanitized telemetry event, and
// shows a recoverable fallback with a copyable event id. The fallback text is intentionally
// i18n-independent so it still renders if the i18n layer is part of the failure.
export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { eventId: null };

  static getDerivedStateFromError(): Partial<AppErrorBoundaryState> {
    return { eventId: 'pending' };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    const eventId = reportClientError({ error, componentStack: info.componentStack ?? '' });
    this.setState({ eventId });
  }

  private reset = (): void => {
    this.setState({ eventId: null });
  };

  private copyEventId = (): void => {
    const { eventId } = this.state;
    if (eventId && navigator.clipboard) {
      void navigator.clipboard.writeText(eventId).catch(() => {});
    }
  };

  render(): ReactNode {
    const { eventId } = this.state;
    if (!eventId) {
      return this.props.children;
    }

    return (
      <div className="appErrorBoundary" role="alert">
        <h1>Something went wrong</h1>
        <p>This view hit an unexpected error. You can retry, or reload the page.</p>
        <p className="appErrorBoundaryId">
          Reference: <code>{eventId}</code>
          <button type="button" onClick={this.copyEventId}>
            Copy
          </button>
        </p>
        <div className="appErrorBoundaryActions">
          <button type="button" onClick={this.reset}>
            Try again
          </button>
          <button type="button" onClick={() => window.location.reload()}>
            Reload
          </button>
        </div>
      </div>
    );
  }
}
