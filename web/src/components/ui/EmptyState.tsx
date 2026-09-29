import React from 'react';
import { cn } from '@/lib/utils';

export interface EmptyStateProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  action?: React.ReactNode;
  className?: string;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  icon,
  title,
  description,
  action,
  className,
}) => {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center p-8 text-center rounded-2xl border border-dashed border-border-default bg-surface-card/50',
        className
      )}
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-subtle border border-primary/20 text-primary-text mb-3.5 shadow-sm">
        {icon}
      </div>
      <h4 className="text-sm font-semibold text-zinc-200 mb-1">{title}</h4>
      <p className="text-xs text-zinc-400 max-w-xs leading-relaxed mb-4">{description}</p>
      {action && <div>{action}</div>}
    </div>
  );
};
