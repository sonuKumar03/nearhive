import { TechnicalJobSearchResult, WorkArrangement } from '@/types';
import { Briefcase, Building2, MapPin, ExternalLink, Calendar, Radio } from 'lucide-react';

interface JobCardProps {
  job: TechnicalJobSearchResult;
}

function formatArrangement(arr: WorkArrangement): { text: string; className: string } {
  switch (arr) {
    case 'in_office':
      return { text: 'In-Office', className: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30' };
    case 'hybrid':
      return { text: 'Hybrid', className: 'text-cyan-400 bg-cyan-500/10 border-cyan-500/30' };
    case 'remote':
      return { text: 'Remote', className: 'text-purple-400 bg-purple-500/10 border-purple-500/30' };
    case 'unknown':
    default:
      return { text: 'Unknown', className: 'text-slate-400 bg-slate-500/10 border-slate-500/30' };
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
  const distanceKm = (job.distance_meters / 1000).toFixed(1);
  const arr = formatArrangement(job.work_arrangement);
  const posted = formatPostedDate(job.posted_at);

  return (
    <div
      aria-label={`${job.title} at ${job.company_name}, ${distanceKm} km away`}
      className="p-3.5 rounded-xl bg-slate-950/60 hover:bg-slate-850/80 border border-slate-800/80 hover:border-amber-500/30 transition-all space-y-2.5"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="space-y-1 min-w-0">
          <h4 className="font-semibold text-xs text-slate-100 truncate flex items-center gap-1.5" title={job.title}>
            <Briefcase className="w-3.5 h-3.5 text-amber-400 shrink-0" />
            <span className="truncate">{job.title}</span>
          </h4>
          <div className="flex items-center gap-1.5 text-xs text-slate-300">
            <Building2 className="w-3 h-3 text-slate-400 shrink-0" />
            <span className="truncate">{job.company_name}</span>
          </div>
        </div>

        <div className="flex flex-col items-end gap-1 shrink-0">
          <span className={`text-[10px] font-mono font-semibold px-2 py-0.5 rounded-full border ${arr.className}`}>
            {arr.text}
          </span>
          <span className="text-[10px] font-mono text-slate-400">
            {distanceKm} km
          </span>
        </div>
      </div>

      {job.description_excerpt && (
        <p className="text-[11px] text-slate-400 line-clamp-2 leading-relaxed">
          {job.description_excerpt}
        </p>
      )}

      <div className="flex items-center justify-between pt-1 border-t border-slate-800/60 text-[10px] text-slate-400">
        <div className="flex items-center gap-3 flex-wrap">
          {job.location_raw && (
            <span className="flex items-center gap-1 truncate max-w-[140px]" title={job.location_raw}>
              <MapPin className="w-3 h-3 text-slate-400 shrink-0" />
              <span className="truncate">{job.location_raw}</span>
            </span>
          )}
          <span className="flex items-center gap-1">
            <Calendar className="w-3 h-3 text-slate-400 shrink-0" />
            <span>{posted}</span>
          </span>
          <span className="flex items-center gap-1 font-mono text-slate-400">
            <Radio className="w-3 h-3 text-slate-400 shrink-0" />
            <span>{job.source}</span>
          </span>
        </div>

        {job.canonical_url && (
          <a
            href={job.canonical_url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 text-amber-400 hover:text-amber-300 transition-colors font-medium ml-2 shrink-0"
            aria-label={`Apply for ${job.title} at ${job.company_name} (opens in new tab)`}
          >
            <span>View</span>
            <ExternalLink className="w-3 h-3" />
          </a>
        )}
      </div>
    </div>
  );
}
