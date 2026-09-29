import React from 'react';
import { cn } from '@/lib/utils';

export type BadgeVariant =
  | 'primary'
  | 'secondary'
  | 'success'
  | 'warning'
  | 'destructive'
  | 'info'
  // Color aliases mapped to semantic roles
  | 'indigo'
  | 'violet'
  | 'cyan'
  | 'sky'
  | 'emerald'
  | 'amber'
  | 'rose'
  | 'slate';

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
  size?: 'sm' | 'md';
  pulsing?: boolean;
  icon?: React.ReactNode;
}

export const Badge: React.FC<BadgeProps> = ({
  className,
  variant = 'secondary',
  size = 'sm',
  pulsing = false,
  icon,
  children,
  ...props
}) => {
  const baseStyles = 'inline-flex items-center font-medium rounded-full border transition-colors select-none tracking-tight';

  const variants: Record<BadgeVariant, string> = {
    // Semantic Roles
    primary: 'bg-primary-subtle text-primary-text border-primary/30',
    secondary: 'bg-secondary text-secondary-foreground border-border-default',
    success: 'bg-success-subtle text-success-text border-success/30',
    warning: 'bg-warning-subtle text-warning-text border-warning/30',
    destructive: 'bg-destructive-subtle text-destructive-text border-destructive/30',
    info: 'bg-info-subtle text-info-text border-info/30',

    // Visual Aliases mapped to semantic tokens
    indigo: 'bg-primary-subtle text-primary-text border-primary/30',
    violet: 'bg-primary-subtle text-primary-text border-primary/30',
    cyan: 'bg-info-subtle text-info-text border-info/30',
    sky: 'bg-info-subtle text-info-text border-info/30',
    emerald: 'bg-success-subtle text-success-text border-success/30',
    amber: 'bg-warning-subtle text-warning-text border-warning/30',
    rose: 'bg-destructive-subtle text-destructive-text border-destructive/30',
    slate: 'bg-secondary text-secondary-foreground border-border-default',
  };

  const pulseDotColors: Record<BadgeVariant, string> = {
    primary: 'bg-primary',
    secondary: 'bg-zinc-400',
    success: 'bg-success',
    warning: 'bg-warning',
    destructive: 'bg-destructive',
    info: 'bg-info',
    indigo: 'bg-primary',
    violet: 'bg-primary',
    cyan: 'bg-info',
    sky: 'bg-info',
    emerald: 'bg-success',
    amber: 'bg-warning',
    rose: 'bg-destructive',
    slate: 'bg-zinc-400',
  };

  const sizes = {
    sm: 'text-[11px] px-2 py-0.5 gap-1',
    md: 'text-xs px-2.5 py-1 gap-1.5',
  };

  return (
    <span className={cn(baseStyles, variants[variant], sizes[size], className)} {...props}>
      {pulsing && (
        <span className="relative flex h-1.5 w-1.5 shrink-0">
          <span className={cn('animate-ping absolute inline-flex h-full w-full rounded-full opacity-75', pulseDotColors[variant])} />
          <span className={cn('relative inline-flex rounded-full h-1.5 w-1.5', pulseDotColors[variant])} />
        </span>
      )}
      {!pulsing && icon && <span className="shrink-0">{icon}</span>}
      {children}
    </span>
  );
};
