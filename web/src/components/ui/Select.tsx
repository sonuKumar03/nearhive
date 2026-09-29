import React from 'react';
import { cn } from '@/lib/utils';
import { ChevronDown } from 'lucide-react';

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  options: SelectOption[];
  label?: string;
  error?: string;
}

export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, options, label, error, disabled, id, ...props }, ref) => {
    return (
      <div className="w-full space-y-1">
        {label && (
          <label htmlFor={id} className="block text-xs font-semibold uppercase tracking-wider text-zinc-400">
            {label}
          </label>
        )}
        <div className="relative">
          <select
            id={id}
            ref={ref}
            disabled={disabled}
            className={cn(
              'w-full appearance-none rounded-xl border border-border-default bg-surface-card/90 px-3.5 py-2.5 text-xs text-zinc-100 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50 pr-10 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-inner',
              error && 'border-destructive focus:border-destructive focus:ring-destructive/30',
              className
            )}
            {...props}
          >
            {options.map((opt) => (
              <option key={opt.value} value={opt.value} className="bg-surface-elevated text-zinc-100 py-1">
                {opt.label}
              </option>
            ))}
          </select>
          <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-zinc-400">
            <ChevronDown className="w-4 h-4" />
          </div>
        </div>
        {error && <p className="text-[11px] text-destructive-text">{error}</p>}
      </div>
    );
  }
);

Select.displayName = 'Select';
