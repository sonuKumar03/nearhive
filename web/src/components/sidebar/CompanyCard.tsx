import { CompanySearchResult, PresenceType, WorkArrangement } from '@/types';
import { Building2, MapPin, Users, Briefcase, CheckCircle2, AlertCircle, HelpCircle } from 'lucide-react';

interface CompanyCardProps {
  company: CompanySearchResult;
  onClick: () => void;
}

function getPresenceBadge(presence?: PresenceType) {
  switch (presence) {
    case 'confirmed_office':
      return {
        label: 'Confirmed Office',
        className: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30',
        Icon: CheckCircle2,
      };
    case 'probable_office':
      return {
        label: 'Probable Office',
        className: 'text-amber-400 bg-amber-500/10 border-amber-500/30',
        Icon: AlertCircle,
      };
    case 'job_location_only':
      return {
        label: 'Job Location',
        className: 'text-sky-400 bg-sky-500/10 border-sky-500/30',
        Icon: MapPin,
      };
    default:
      return null;
  }
}

function formatArrangement(arr: WorkArrangement): { text: string; ariaLabel: string } {
  switch (arr) {
    case 'in_office':
      return { text: 'In-Office', ariaLabel: 'Work arrangement: In-Office' };
    case 'hybrid':
      return { text: 'Hybrid', ariaLabel: 'Work arrangement: Hybrid' };
    case 'remote':
      return { text: 'Remote', ariaLabel: 'Work arrangement: Remote' };
    case 'unknown':
    default:
      return { text: 'Unknown', ariaLabel: 'Work arrangement: Unknown' };
  }
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
  const presence = getPresenceBadge(company.presence_type);
  const arrangements = company.arrangements || [];
  const techJobCount = company.recent_technical_job_count;

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
      aria-label={`${company.name}, ${distanceKm} km away, ${conf}% verified${
        company.presence_type ? `, ${company.presence_type}` : ''
      }`}
      className="p-3 rounded-xl bg-slate-950/60 hover:bg-slate-850/80 focus:bg-slate-800/80 focus:outline-none focus:ring-1 focus:ring-amber-500/50 border border-slate-800/80 hover:border-amber-500/30 transition-all cursor-pointer group space-y-2"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="space-y-1">
          <h4 className="font-semibold text-xs text-slate-100 group-hover:text-amber-300 transition-colors flex items-center gap-1.5">
            <Building2 className="w-3.5 h-3.5 text-amber-400" />
            {company.name}
          </h4>
          <div className="flex items-center gap-2 flex-wrap">
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

        <div className="flex flex-col items-end gap-1 shrink-0">
          <span className={`text-[10px] font-mono font-semibold px-2 py-0.5 rounded-full border ${confBadgeColor}`}>
            {conf}%
          </span>
          {presence && (
            <span
              role="status"
              aria-label={`Presence: ${presence.label}`}
              className={`text-[9px] font-medium px-1.5 py-0.5 rounded-full border flex items-center gap-1 ${presence.className}`}
            >
              <presence.Icon className="w-2.5 h-2.5" />
              <span>{presence.label}</span>
            </span>
          )}
        </div>
      </div>

      {/* Discovery badges: Recent Technical Jobs & Arrangements */}
      {(Boolean(techJobCount && techJobCount > 0) || arrangements.length > 0) && (
        <div className="flex items-center gap-1.5 flex-wrap pt-0.5">
          {Boolean(techJobCount && techJobCount > 0) && (
            <span
              role="status"
              aria-label={`${techJobCount} recent technical job${techJobCount === 1 ? '' : 's'}`}
              className="flex items-center gap-1 text-[10px] font-mono font-medium px-1.5 py-0.5 rounded bg-indigo-500/10 text-indigo-300 border border-indigo-500/30"
            >
              <Briefcase className="w-3 h-3 text-indigo-400" />
              <span>
                {techJobCount} tech job{techJobCount === 1 ? '' : 's'}
              </span>
            </span>
          )}

          {arrangements.map((arr) => {
            const info = formatArrangement(arr);
            return (
              <span
                key={arr}
                role="status"
                aria-label={info.ariaLabel}
                className="text-[9px] font-medium px-1.5 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300"
              >
                {info.text}
              </span>
            );
          })}
        </div>
      )}

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
