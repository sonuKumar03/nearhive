import React from 'react';
import { cn } from '@/lib/utils';
import { X } from 'lucide-react';

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = 'text', label, error, leftIcon, rightIcon, id, disabled, ...props }, ref) => {
    return (
      <div className="w-full space-y-1">
        {label && (
          <label htmlFor={id} className="block text-xs font-semibold uppercase tracking-wider text-zinc-400">
            {label}
          </label>
        )}
        <div className="relative flex items-center">
          {leftIcon && (
            <div className="absolute left-3 text-zinc-400 pointer-events-none flex items-center">
              {leftIcon}
            </div>
          )}
          <input
            id={id}
            ref={ref}
            type={type}
            disabled={disabled}
            className={cn(
              'w-full rounded-xl border border-border-default bg-surface-card/90 px-3.5 py-2.5 text-xs text-zinc-100 placeholder:text-zinc-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50 transition-all shadow-inner disabled:opacity-50 disabled:cursor-not-allowed',
              leftIcon && 'pl-9',
              rightIcon && 'pr-9',
              error && 'border-destructive focus:border-destructive focus:ring-destructive/30',
              className
            )}
            {...props}
          />
          {rightIcon && (
            <div className="absolute right-3 text-zinc-400 flex items-center">
              {rightIcon}
            </div>
          )}
        </div>
        {error && <p className="text-[11px] text-destructive-text">{error}</p>}
      </div>
    );
  }
);

Input.displayName = 'Input';

export interface SearchInputProps extends Omit<InputProps, 'rightIcon'> {
  onClear?: () => void;
}

export const SearchInput = React.forwardRef<HTMLInputElement, SearchInputProps>(
  ({ onClear, value, ...props }, ref) => {
    const hasValue = Boolean(value && String(value).length > 0);

    return (
      <Input
        ref={ref}
        value={value}
        rightIcon={
          hasValue && onClear ? (
            <button
              type="button"
              onClick={onClear}
              aria-label="Clear input"
              className="p-0.5 rounded text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors cursor-pointer"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          ) : undefined
        }
        {...props}
      />
    );
  }
);

SearchInput.displayName = 'SearchInput';
