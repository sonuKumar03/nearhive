import React from 'react';
import { cn } from '@/lib/utils';

export interface SegmentedOption<T extends string = string> {
  value: T;
  label: string;
  icon?: React.ReactNode;
  count?: number | string;
}

export interface SegmentedControlProps<T extends string = string> {
  options: SegmentedOption<T>[];
  value: T;
  onChange: (value: T) => void;
  className?: string;
  size?: 'sm' | 'md';
}

export function SegmentedControl<T extends string = string>({
  options,
  value,
  onChange,
  className,
  size = 'md',
}: SegmentedControlProps<T>) {
  const heightStyles = size === 'sm' ? 'h-7 text-[11px]' : 'h-8 text-xs';

  return (
    <div
      role="tablist"
      className={cn(
        'flex items-center gap-1 p-1 bg-slate-950 rounded-xl border border-slate-800 shadow-inner select-none',
        className
      )}
    >
      {options.map((option) => {
        const isSelected = option.value === value;
        return (
          <button
            key={option.value}
            role="tab"
            type="button"
            aria-selected={isSelected}
            onClick={() => onChange(option.value)}
            className={cn(
              'flex-1 px-3 rounded-lg transition-colors flex items-center justify-center gap-1.5 cursor-pointer font-medium border box-border',
              heightStyles,
              isSelected
                ? 'bg-slate-800 text-amber-400 font-semibold shadow-sm border-slate-700/60'
                : 'text-slate-400 border-transparent hover:text-slate-200 hover:bg-slate-900/40'
            )}
          >
            {option.icon && <span className="shrink-0">{option.icon}</span>}
            <span className="truncate">{option.label}</span>
            {option.count !== undefined && (
              <span
                className={cn(
                  'text-[10px] font-mono px-1.5 py-0.5 rounded-full border transition-colors shrink-0',
                  isSelected
                    ? 'bg-amber-400/15 text-amber-300 border-amber-400/30'
                    : 'bg-slate-900 text-slate-500 border-slate-800'
                )}
              >
                {option.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
