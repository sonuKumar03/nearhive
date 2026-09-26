import { CompanySearchResult } from '@/types';
import { Building2, MapPin, Users } from 'lucide-react';

interface CompanyCardProps {
  company: CompanySearchResult;
  onClick: () => void;
}

export default function CompanyCard({ company, onClick }: CompanyCardProps) {
  const conf = Math.round(company.confidence * 100);
  const confBadgeColor =
    conf >= 80
      ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30'
      : conf >= 60
      ? 'text-amber-400 bg-amber-500/10 border-amber-500/30'
      : 'text-slate-400 bg-slate-500/10 border-slate-500/30';

  const distanceKm = (company.distance_meters / 1000).toFixed(1);

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onClick();
        }
      }}
      aria-label={`${company.name}, ${distanceKm} km away, ${conf}% verified`}
      className="p-3 rounded-xl bg-slate-950/60 hover:bg-slate-850/80 focus:bg-slate-800/80 focus:outline-none focus:ring-1 focus:ring-amber-500/50 border border-slate-800/80 hover:border-amber-500/30 transition-all cursor-pointer group space-y-2"
    >
      <div className="flex items-start justify-between gap-2">
        <div>
          <h4 className="font-semibold text-xs text-slate-100 group-hover:text-amber-300 transition-colors flex items-center gap-1.5">
            <Building2 className="w-3.5 h-3.5 text-amber-400" />
            {company.name}
          </h4>
          <div className="flex items-center gap-2 mt-1">
            {company.industry && (
              <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-slate-800 text-slate-300">
                {company.industry}
              </span>
            )}
            {company.employee_count && (
              <span className="text-[10px] text-slate-300 font-mono flex items-center gap-1">
                <Users className="w-3 h-3 text-slate-400" />
                {company.employee_count}
              </span>
            )}
          </div>
        </div>
        <span className={`text-[10px] font-mono font-semibold px-2 py-0.5 rounded-full border ${confBadgeColor}`}>
          {conf}%
        </span>
      </div>

      <p className="text-[11px] text-slate-300 leading-relaxed flex items-center gap-1.5">
        <MapPin className="w-3 h-3 text-slate-400 shrink-0" />
        <span className="truncate">{company.address || 'Address registered'}</span>
      </p>

      <div className="flex items-center justify-between pt-1 border-t border-slate-800/50 text-[11px]">
        <span className="text-slate-400 font-mono text-[10px]">{distanceKm} km away</span>
        <span className="text-amber-400 group-hover:text-amber-300 font-medium text-[11px] flex items-center gap-1">
          Details ›
        </span>
      </div>
    </div>
  );
}
