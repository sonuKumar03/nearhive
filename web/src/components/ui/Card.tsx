import React from 'react';
import { cn } from '@/lib/utils';

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'elevated' | 'glass';
  hoverable?: boolean;
  selected?: boolean;
  padding?: 'none' | 'sm' | 'md' | 'lg';
}

export const Card = React.forwardRef<HTMLDivElement, CardProps>(
  (
    {
      className,
      variant = 'default',
      hoverable = false,
      selected = false,
      padding = 'md',
      children,
      ...props
    },
    ref
  ) => {
    const baseStyles = 'rounded-2xl border transition-all';

    const variants = {
      default: 'bg-slate-900/90 border-slate-800/90 text-slate-100',
      elevated: 'bg-slate-900 border-slate-700/80 shadow-xl text-slate-100',
      glass: 'bg-slate-950/70 backdrop-blur-md border-slate-800/80 text-slate-100',
    };

    const paddings = {
      none: '',
      sm: 'p-2.5',
      md: 'p-3.5',
      lg: 'p-5',
    };

    return (
      <div
        ref={ref}
        className={cn(
          baseStyles,
          variants[variant],
          paddings[padding],
          hoverable && 'hover:border-slate-700 hover:shadow-lg cursor-pointer',
          selected && 'border-amber-500/80 bg-slate-900/95 shadow-md shadow-amber-500/10 ring-1 ring-amber-500/30',
          className
        )}
        {...props}
      >
        {children}
      </div>
    );
  }
);

Card.displayName = 'Card';
