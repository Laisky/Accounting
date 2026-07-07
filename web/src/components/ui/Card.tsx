import type { HTMLAttributes, ReactNode } from 'react';
import './ui.css';

type CardProps = HTMLAttributes<HTMLDivElement> & {
  as?: 'div' | 'section' | 'article';
  children: ReactNode;
};

// Card is the shared raised-surface container primitive.
export function Card({ as: Tag = 'div', className, children, ...rest }: CardProps) {
  return (
    <Tag className={['ui-card', className ?? ''].filter(Boolean).join(' ')} {...rest}>
      {children}
    </Tag>
  );
}
