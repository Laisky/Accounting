import { useTranslation } from 'react-i18next';
import { Button } from './Button';
import { Sheet } from './Sheet';
import './ui.css';

type ConfirmDialogProps = {
  open: boolean;
  title: string;
  description?: string;
  confirmLabel: string;
  cancelLabel?: string;
  destructive?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

// ConfirmDialog is the shared confirmation modal for destructive/irreversible actions (P5.5).
export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  cancelLabel,
  destructive,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const { t } = useTranslation();
  return (
    <Sheet open={open} title={title} onClose={onCancel} closeLabel={cancelLabel ?? t('common.cancel')}>
      {description ? <p className="ui-confirm-body">{description}</p> : null}
      <div className="ui-confirm-actions">
        <Button variant="secondary" onClick={onCancel}>
          {cancelLabel ?? t('common.cancel')}
        </Button>
        <Button variant={destructive ? 'danger' : 'primary'} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </Sheet>
  );
}
