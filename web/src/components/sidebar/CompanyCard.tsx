import { CompanySearchResult, PresenceType, WorkArrangement } from '@/types';
import { Card, Badge } from '@/components/ui';
import {
  MapPin,
  Users,
  Briefcase,
  CheckCircle2,
  AlertCircle,
  Navigation,
  ChevronRight,
} from 'lucide-react';

interface CompanyCardProps {
  company: CompanySearchResult;
  onClick: () => void;
  isSelected?: boolean;
}

const AVATAR_PALETTES = [
  'bg-blue-500/10 text-blue-400 border-blue-500/25',
  'bg-emerald-500/10 text-emerald-400 border-emerald-500/25',
  'bg-purple-500/10 text-purple-400 border-purple-500/25',
  'bg-amber-500/10 text-amber-400 border-amber-500/25',
  'bg-cyan-500/10 text-cyan-400 border-cyan-500/25',
  'bg-rose-500/10 text-rose-400 border-rose-500/25',
  'bg-indigo-500/10 text-indigo-400 border-indigo-500/25',
];

function getAvatarColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  return AVATAR_PALETTES[Math.abs(hash) % AVATAR_PALETTES.length];
}

function getInitials(name: string): string {
  if (!name) return 'CO';
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return (parts[0][0] + parts[1][0]).toUpperCase();
}

function cleanAddress(raw?: string): string {
  if (!raw || raw.trim() === '' || raw.toLowerCase() === 'address registered') {
    return 'Registered office location';
  }
  return raw
    .replace(/,\s*,+/g, ', ')
    .replace(/\s+/g, ' ')
    .replace(/^,\s*|,\s*$/g, '')
    .trim();
}

function getPresenceBadge(presence?: PresenceType, confidencePercent: number = 60) {
  if (presence === 'confirmed_office' || confidencePercent >= 80) {
    return {
      label: `Confirmed · ${confidencePercent}%`,
      variant: 'emerald' as const,
      Icon: CheckCircle2,
    };
  }
  if (presence === 'job_location_only') {
    return {
      label: `Hiring · ${confidencePercent}%`,
      variant: 'sky' as const,
      Icon: MapPin,
    };
  }
  return {
    label: `Probable · ${confidencePercent}%`,
    variant: 'amber' as const,
    Icon: AlertCircle,
  };
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

export default function CompanyCard({ company, onClick, isSelected = false }: CompanyCardProps) {
  const conf = Math.round(company.confidence * 100);
  const distanceKm =
    typeof company.distance_meters === 'number' && !isNaN(company.distance_meters)
      ? (company.distance_meters / 1000).toFixed(1)
      : null;
  const status = getPresenceBadge(company.presence_type, conf);
  const arrangements = company.arrangements || [];
  const techJobCount = company.recent_technical_job_count;
  const initials = getInitials(company.name);
  const avatarColor = getAvatarColor(company.name);
  const displayAddress = cleanAddress(company.address);

  return (
    <Card
      role="button"
      tabIndex={0}
      hoverable
      selected={isSelected}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onClick();
        }
      }}
      aria-label={`${company.name}, ${distanceKm ? `${distanceKm} km away, ` : ''}${conf}% verified${
        company.presence_type ? `, ${company.presence_type}` : ''
      }`}
      className="p-3.5 space-y-2.5 group"
    >
      {/* Header: Monogram Avatar + Title + Status Pill */}
      <div className="flex items-start gap-2.5">
        <div
          className={`w-8 h-8 rounded-lg border font-mono font-bold text-xs flex items-center justify-center shrink-0 tracking-wider ${avatarColor}`}
          aria-hidden="true"
        >
          {initials}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-1.5">
            <h4
              className="font-semibold text-xs text-slate-100 group-hover:text-amber-300 transition-colors truncate leading-tight"
              title={company.name}
            >
              {company.name}
            </h4>

            {/* Unified status pill */}
            <Badge
              variant={status.variant}
              icon={<status.Icon className="w-2.5 h-2.5" />}
              className="shrink-0 font-mono text-[10px]"
            >
              {status.label}
            </Badge>
          </div>

          {/* Industry & Size */}
          {(company.industry || company.employee_count) && (
            <div className="flex items-center gap-1.5 mt-1 text-[10px] text-slate-400 flex-wrap">
              {company.industry && (
                <span className="truncate max-w-[170px] text-slate-300 font-medium">
                  {company.industry}
                </span>
              )}
              {company.industry && company.employee_count && (
                <span className="text-slate-600">•</span>
              )}
              {company.employee_count && (
                <span className="flex items-center gap-1 text-slate-400 font-mono">
                  <Users className="w-2.5 h-2.5 text-slate-500" />
                  {company.employee_count}
                </span>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Discovery badges: Recent Technical Jobs & Arrangements */}
      {(Boolean(techJobCount && techJobCount > 0) || arrangements.length > 0) && (
        <div className="flex items-center gap-1.5 flex-wrap pt-0.5">
          {Boolean(techJobCount && techJobCount > 0) && (
            <Badge
              variant="sky"
              icon={<Briefcase className="w-2.5 h-2.5" />}
              className="font-mono text-[10px]"
            >
              {techJobCount} tech job{techJobCount === 1 ? '' : 's'}
            </Badge>
          )}

          {arrangements.map((arr) => {
            const info = formatArrangement(arr);
            return (
              <Badge key={arr} variant="slate" className="text-[10px]">
                {info.text}
              </Badge>
            );
          })}
        </div>
      )}

      {/* Address */}
      <p
        className="text-[11px] text-slate-400 leading-relaxed flex items-center gap-1.5"
        title={displayAddress}
      >
        <MapPin className="w-3 h-3 text-slate-500 shrink-0" />
        <span className="truncate">{displayAddress}</span>
      </p>

      {/* Footer: Distance & Details Action */}
      <div className="flex items-center justify-between pt-1.5 border-t border-slate-800/60 text-[11px]">
        <span className="text-slate-400 font-mono text-[10px] flex items-center gap-1">
          <Navigation className="w-2.5 h-2.5 text-slate-500" />
          {distanceKm ? `${distanceKm} km away` : 'Nearby'}
        </span>
        <span className="inline-flex items-center gap-0.5 text-[11px] font-medium text-slate-400 group-hover:text-amber-300 transition-colors">
          <span>Details</span>
          <ChevronRight className="w-3 h-3 transition-transform group-hover:translate-x-0.5" />
        </span>
      </div>
    </Card>
  );
}
