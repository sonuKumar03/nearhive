import React from 'react';
import { cn } from '@/lib/utils';
import { SearchX } from 'lucide-react';

export interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: string;
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
    <div className={cn('py-14 px-6 text-center space-y-3.5 select-none', className)}>
      <div className="w-12 h-12 rounded-2xl bg-slate-800/70 border border-slate-700/50 flex items-center justify-center mx-auto shadow-inner text-slate-400">
        {icon || <SearchX className="w-6 h-6" />}
      </div>
      <div className="space-y-1">
        <h4 className="font-semibold text-xs text-slate-200 tracking-tight">{title}</h4>
        {description && (
          <p className="text-[11px] text-slate-400 leading-relaxed max-w-[280px] mx-auto">
            {description}
          </p>
        )}
      </div>
      {action && <div className="pt-1">{action}</div>}
    </div>
  );
};
