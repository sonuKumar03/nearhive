import React from 'react';
import { cn } from '@/lib/utils';

export interface SliderPreset {
  label: string;
  value: number;
  desc?: string;
}

export interface SliderProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  label?: string;
  badgeText?: string;
  presets?: SliderPreset[];
  scaleLabels?: [string, string, string];
  id?: string;
  className?: string;
}

export const Slider: React.FC<SliderProps> = ({
  value,
  onChange,
  min = 1,
  max = 30,
  step = 1,
  label,
  badgeText,
  presets,
  scaleLabels,
  id = 'range-slider',
  className,
}) => {
  return (
    <div className={cn('space-y-2.5 w-full', className)}>
      {(label || badgeText) && (
        <div className="flex items-center justify-between">
          {label && (
            <label htmlFor={id} className="text-xs font-semibold uppercase tracking-wider text-slate-400">
              {label}
            </label>
          )}
          {badgeText && (
            <span className="text-xs font-mono font-semibold text-amber-400 bg-amber-500/10 px-2.5 py-0.5 rounded-full border border-amber-500/30">
              {badgeText}
            </span>
          )}
        </div>
      )}

      {presets && presets.length > 0 && (
        <div className="grid grid-cols-3 gap-2">
          {presets.map((preset) => {
            const isActive = value === preset.value;
            return (
              <button
                key={preset.value}
                type="button"
                onClick={() => onChange(preset.value)}
                className={cn(
                  'px-3 py-1.5 rounded-xl text-xs font-medium border text-center transition-all cursor-pointer',
                  isActive
                    ? 'border-amber-500/80 bg-amber-500/15 text-amber-300 font-semibold shadow-sm'
                    : 'border-slate-800 bg-slate-800/40 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
                )}
              >
                <div>{preset.label}</div>
                {preset.desc && <div className="text-[10px] opacity-75">{preset.desc}</div>}
              </button>
            );
          })}
        </div>
      )}

      <input
        id={id}
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full accent-amber-500 cursor-pointer h-1.5 bg-slate-800 rounded-lg mt-1"
      />

      {scaleLabels && (
        <div className="flex justify-between text-[11px] text-slate-500 font-medium">
          <span>{scaleLabels[0]}</span>
          <span>{scaleLabels[1]}</span>
          <span>{scaleLabels[2]}</span>
        </div>
      )}
    </div>
  );
};
