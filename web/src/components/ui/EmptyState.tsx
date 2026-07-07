import type { ReactNode } from 'react';
import { Button } from './Button';
import './ui.css';

type EmptyStateProps = {
  icon?: ReactNode;
  title: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
  actionDisabled?: boolean;
};

// EmptyState explains a zero-data view and offers one primary action (P5.1).
export function EmptyState({ icon, title, description, actionLabel, onAction, actionDisabled }: EmptyStateProps) {
  return (
    <div className="ui-empty" role="status">
      {icon ? (
        <span className="ui-empty-icon" aria-hidden="true">
          {icon}
        </span>
      ) : null}
      <p className="ui-empty-title">{title}</p>
      {description ? <p className="ui-empty-body">{description}</p> : null}
      {actionLabel && onAction ? (
        <Button variant="primary" onClick={onAction} disabled={actionDisabled}>
          {actionLabel}
        </Button>
      ) : null}
    </div>
  );
}
