import type { ButtonHTMLAttributes, ReactNode, Ref } from 'react';
import './ui.css';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: 'md' | 'sm';
  block?: boolean;
  ref?: Ref<HTMLButtonElement>;
  children: ReactNode;
};

// Button is the shared action primitive: token-styled, 44px minimum target, focus-visible ring.
export function Button({ variant = 'primary', size = 'md', block, className, type, children, ...rest }: ButtonProps) {
  const classes = [
    'ui-button',
    `ui-button-${variant}`,
    size === 'sm' ? 'ui-button-sm' : '',
    block ? 'ui-button-block' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <button type={type ?? 'button'} className={classes} {...rest}>
      {children}
    </button>
  );
}
