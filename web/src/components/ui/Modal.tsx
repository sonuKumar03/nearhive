'use client';

import React, { useEffect } from 'react';
import { cn } from '@/lib/utils';
import { X } from 'lucide-react';

export interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  subtitle?: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  maxWidth?: 'sm' | 'md' | 'lg' | 'xl';
  className?: string;
}

export const Modal: React.FC<ModalProps> = ({
  isOpen,
  onClose,
  title,
  subtitle,
  icon,
  children,
  footer,
  maxWidth = 'lg',
  className,
}) => {
  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const maxWidths = {
    sm: 'max-w-sm',
    md: 'max-w-md',
    lg: 'max-w-lg',
    xl: 'max-w-xl',
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-in fade-in duration-200"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? 'modal-title' : undefined}
        className={cn(
          'w-full max-h-[90vh] flex flex-col rounded-2xl border border-border-default bg-surface-card/98 text-zinc-100 shadow-modal overflow-hidden',
          maxWidths[maxWidth],
          className
        )}
      >
        {/* Header */}
        {(title || icon) && (
          <div className="flex items-center justify-between border-b border-border-subtle px-6 py-4 shrink-0">
            <div className="flex items-center gap-3">
              {icon && (
                <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary-subtle border border-primary/25 text-primary-text shrink-0">
                  {icon}
                </div>
              )}
              <div>
                {title && (
                  <h2 id="modal-title" className="text-base font-semibold tracking-tight text-white">
                    {title}
                  </h2>
                )}
                {subtitle && <p className="text-xs text-zinc-400 mt-0.5">{subtitle}</p>}
              </div>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="Close dialog"
              className="rounded-lg p-1.5 text-zinc-400 hover:bg-surface-elevated hover:text-white transition-colors cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        )}

        {/* Content Body */}
        <div className="overflow-y-auto px-6 py-5 flex-1 [scrollbar-gutter:stable]">
          {children}
        </div>

        {/* Optional Footer */}
        {footer && (
          <div className="border-t border-border-subtle px-6 py-3.5 bg-surface-base/80 shrink-0">
            {footer}
          </div>
        )}
      </section>
    </div>
  );
};
