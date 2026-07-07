import { useId, type ReactElement } from 'react';
import './ui.css';

type FormFieldProps = {
  label: string;
  hint?: string;
  error?: string;
  // render receives the wiring ids so the control links to its label, hint, and error.
  children: (field: { id: string; describedBy?: string; invalid: boolean }) => ReactElement;
};

// FormField is the shared labeled-control primitive: it owns label/hint/error wiring and
// aria-describedby so every feature form gets consistent, accessible field semantics.
export function FormField({ label, hint, error, children }: FormFieldProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const describedBy = [hint ? hintId : '', error ? errorId : ''].filter(Boolean).join(' ') || undefined;

  return (
    <div className="ui-field">
      <label className="ui-field-label" htmlFor={id}>
        {label}
      </label>
      {children({ id, describedBy, invalid: Boolean(error) })}
      {hint ? (
        <span className="ui-field-hint" id={hintId}>
          {hint}
        </span>
      ) : null}
      {error ? (
        <span className="ui-field-error" id={errorId} role="alert">
          {error}
        </span>
      ) : null}
    </div>
  );
}
