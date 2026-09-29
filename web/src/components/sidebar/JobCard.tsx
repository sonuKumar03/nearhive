import { TechnicalJobSearchResult, WorkArrangement } from '@/types';
import { Card, Badge } from '@/components/ui';
import {
  MapPin,
  ExternalLink,
  Calendar,
  Radio,
  Navigation,
} from 'lucide-react';

interface JobCardProps {
  job: TechnicalJobSearchResult;
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
  if (!name) return 'JB';
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return (parts[0][0] + parts[1][0]).toUpperCase();
}

function formatArrangement(arr: WorkArrangement): { text: string; variant: 'emerald' | 'sky' | 'slate' | 'amber' } {
  switch (arr) {
    case 'in_office':
      return { text: 'In-Office', variant: 'emerald' };
    case 'hybrid':
      return { text: 'Hybrid', variant: 'sky' };
    case 'remote':
      return { text: 'Remote', variant: 'amber' };
    case 'unknown':
    default:
      return { text: 'Unknown', variant: 'slate' };
  }
}

function formatPostedDate(dateStr?: string): string {
  if (!dateStr) return 'Recently';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return 'Recently';
  const now = new Date();
  const diffDays = Math.floor((now.getTime() - d.getTime()) / (1000 * 60 * 60 * 24));
  if (diffDays <= 0) return 'Today';
  if (diffDays === 1) return 'Yesterday';
  if (diffDays < 14) return `${diffDays}d ago`;
  return d.toLocaleDateString();
}

export default function JobCard({ job }: JobCardProps) {
  const distanceKm =
    typeof job.distance_meters === 'number' && !isNaN(job.distance_meters)
      ? (job.distance_meters / 1000).toFixed(1)
      : null;
  const arr = formatArrangement(job.work_arrangement);
  const posted = formatPostedDate(job.posted_at);
  const initials = getInitials(job.company_name);
  const avatarColor = getAvatarColor(job.company_name);

  return (
    <Card
      hoverable
      aria-label={`${job.title} at ${job.company_name}${distanceKm ? `, ${distanceKm} km away` : ''}`}
      className="p-3.5 space-y-2.5 group"
    >
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
              title={job.title}
            >
              {job.title}
            </h4>
            <Badge variant={arr.variant} className="shrink-0 font-mono text-[10px]">
              {arr.text}
            </Badge>
          </div>

          <p className="text-[11px] text-slate-400 truncate mt-0.5 font-medium">
            {job.company_name}
          </p>
        </div>
      </div>

      {job.description_excerpt && (
        <p className="text-[11px] text-slate-400 line-clamp-2 leading-relaxed">
          {job.description_excerpt}
        </p>
      )}

      <div className="flex items-center justify-between pt-1.5 border-t border-slate-800/60 text-[10px] text-slate-400">
        <div className="flex items-center gap-2.5 flex-wrap">
          {job.location_raw && (
            <span className="flex items-center gap-1 truncate max-w-[130px]" title={job.location_raw}>
              <MapPin className="w-2.5 h-2.5 text-slate-500 shrink-0" />
              <span className="truncate">{job.location_raw}</span>
            </span>
          )}
          <span className="flex items-center gap-1">
            <Calendar className="w-2.5 h-2.5 text-slate-500 shrink-0" />
            <span>{posted}</span>
          </span>
          <span className="flex items-center gap-1 font-mono text-slate-400">
            <Radio className="w-2.5 h-2.5 text-slate-500 shrink-0" />
            <span>{job.source}</span>
          </span>
          {distanceKm && (
            <span className="flex items-center gap-1 font-mono text-slate-400">
              <Navigation className="w-2.5 h-2.5 text-slate-500 shrink-0" />
              <span>{distanceKm} km</span>
            </span>
          )}
        </div>

        {job.canonical_url && (
          <a
            href={job.canonical_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-amber-400 hover:text-amber-300 transition-colors font-medium ml-2 shrink-0 text-[11px]"
            aria-label={`Apply for ${job.title} at ${job.company_name} (opens in new tab)`}
          >
            <span>View</span>
            <ExternalLink className="w-2.5 h-2.5" />
          </a>
        )}
      </div>
    </Card>
  );
}
