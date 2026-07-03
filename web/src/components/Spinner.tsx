import { Loader2 } from 'lucide-react';
import './loading.css';

type SpinnerProps = {
  size?: number;
  className?: string;
};

// Spinner receives an optional size and class name and returns a decorative rotating loading glyph.
export function Spinner({ size = 18, className }: SpinnerProps) {
  return <Loader2 size={size} className={`spinner${className ? ` ${className}` : ''}`} aria-hidden="true" focusable={false} />;
}
