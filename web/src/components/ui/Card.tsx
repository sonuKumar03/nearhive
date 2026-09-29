import React from 'react';
import { cn } from '@/lib/utils';

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  hoverable?: boolean;
  selected?: boolean;
  children: React.ReactNode;
}

export const Card = React.forwardRef<HTMLDivElement, CardProps>(
  ({ className, hoverable = false, selected = false, children, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          'rounded-2xl border border-border-default bg-surface-card/90 backdrop-blur-md transition-all text-zinc-100 shadow-sm',
          hoverable &&
            'hover:border-border-strong hover:bg-surface-overlay hover:shadow-xl hover:shadow-black/60 cursor-pointer',
          selected &&
            'ring-1 ring-primary border-primary bg-surface-overlay shadow-xl shadow-primary/15',
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
