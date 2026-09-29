import React from 'react';
import { cn } from '@/lib/utils';

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'xs' | 'sm' | 'md' | 'lg';
  isLoading?: boolean;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = 'primary',
      size = 'md',
      isLoading = false,
      leftIcon,
      rightIcon,
      disabled,
      children,
      ...props
    },
    ref
  ) => {
    const baseStyles =
      'inline-flex items-center justify-center font-medium transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-border-focus disabled:opacity-50 disabled:pointer-events-none cursor-pointer select-none rounded-xl tracking-tight';

    const variants = {
      primary:
        'bg-primary hover:bg-primary-hover active:scale-[0.99] text-primary-foreground font-medium shadow-md shadow-glow-primary border border-primary/30',
      secondary:
        'bg-secondary hover:bg-secondary-hover text-secondary-foreground border border-border-default active:scale-[0.99] shadow-sm',
      outline:
        'bg-transparent hover:bg-secondary text-zinc-300 hover:text-white border border-border-default',
      ghost:
        'bg-transparent hover:bg-secondary text-zinc-400 hover:text-zinc-100 border border-transparent',
      danger:
        'bg-destructive-subtle hover:bg-destructive/20 text-destructive border border-destructive/30',
    };

    const sizes = {
      xs: 'h-6 px-2 text-[11px] gap-1 rounded-lg',
      sm: 'h-8 px-2.5 text-xs gap-1.5 rounded-lg',
      md: 'h-9 px-3.5 text-xs gap-2',
      lg: 'h-11 px-5 text-sm gap-2.5',
    };

    return (
      <button
        ref={ref}
        disabled={disabled || isLoading}
        className={cn(baseStyles, variants[variant], sizes[size], className)}
        {...props}
      >
        {isLoading ? (
          <svg className="animate-spin h-3.5 w-3.5 shrink-0" viewBox="0 0 24 24" fill="none">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
          </svg>
        ) : (
          leftIcon && <span className="shrink-0">{leftIcon}</span>
        )}
        {children && <span>{children}</span>}
        {!isLoading && rightIcon && <span className="shrink-0">{rightIcon}</span>}
      </button>
    );
  }
);

Button.displayName = 'Button';
