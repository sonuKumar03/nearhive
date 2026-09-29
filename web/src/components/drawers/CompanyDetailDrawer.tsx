import { useEffect } from 'react';
import { CompanySearchResult, PresenceType, WorkArrangement, TechnicalJobPosting } from '@/types';
import { useCompany, useSightings } from '@/hooks/useCompanyDetails';
import { useCompanyTechnicalJobs } from '@/hooks/useDiscoveryJobs';
import SightingsTimeline from './SightingsTimeline';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import {
  X,
  Building2,
  Globe,
  ShieldCheck,
  MapPin,
  Briefcase,
  ExternalLink,
  Calendar,
  Clock,
  CheckCircle2,
  AlertCircle,
} from 'lucide-react';

interface DrawerProps {
  company: CompanySearchResult | null;
  onClose: () => void;
  onFocusOnMap?: () => void;
}

function getPresenceDisplay(presence?: PresenceType) {
  switch (presence) {
    case 'confirmed_office':
      return {
        label: 'Confirmed Office',
        description: 'Verified physical office location backed by corroborated registry or primary evidence.',
        variant: 'emerald' as const,
        Icon: CheckCircle2,
      };
    case 'probable_office':
      return {
        label: 'Probable Office',
        description: 'Probable office location inferred from directory signals and consistent address listings.',
        variant: 'amber' as const,
        Icon: AlertCircle,
      };
    case 'job_location_only':
      return {
        label: 'Job Location Only',
        description: 'Location detected from hiring and job postings; physical office presence unconfirmed.',
        variant: 'sky' as const,
        Icon: MapPin,
      };
    default:
      return {
        label: 'Verified Location',
        description: 'Location verified through algorithmic evidence matching.',
        variant: 'emerald' as const,
        Icon: ShieldCheck,
      };
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

function formatDate(dateStr?: string | null): string {
  if (!dateStr) return '';
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '';
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  } catch {
    return '';
  }
}

function isPostedWithin14Days(job: TechnicalJobPosting): boolean {
  if (job.publication_state === 'stale') return false;
  if (job.publication_state === 'posted_recently') return true;
  if (!job.posted_at) return false;
  try {
    const postedTime = new Date(job.posted_at).getTime();
    const now = Date.now();
    const fourteenDaysMs = 14 * 24 * 60 * 60 * 1000;
    return now - postedTime <= fourteenDaysMs && now >= postedTime;
  } catch {
    return false;
  }
}

export default function CompanyDetailDrawer({ company, onClose, onFocusOnMap }: DrawerProps) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  const { data: details } = useCompany(company?.id ?? null);
  const { data: sightingsData, isLoading: loadingSightings } = useSightings(company?.id ?? null);
  const { data: techJobsData, isLoading: loadingTechJobs } = useCompanyTechnicalJobs(
    company?.id ?? null,
    14
  );

  if (!company) return null;

  const conf = Math.round(company.confidence * 100);
  const presenceType = company.presence_type || details?.company?.presence_type;
  const presenceInfo = getPresenceDisplay(presenceType);
  const technicalJobs = techJobsData?.technical_jobs || techJobsData?.jobs || [];

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Company Details"
      className="fixed inset-y-0 right-0 w-full sm:w-[460px] bg-slate-900 border-l border-slate-800/80 shadow-2xl z-50 flex flex-col backdrop-blur-xl animate-in slide-in-from-right duration-300"
    >
      {/* Header */}
      <div className="p-4 border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-lg bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-400">
            <Building2 className="w-4 h-4" />
          </div>
          <div>
            <h3 className="font-bold text-sm text-slate-100">{company.name}</h3>
            <span className="text-[10px] text-slate-400 font-mono">ID: {company.id.slice(0, 8)}...</span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          {onFocusOnMap && (
            <Button
              size="xs"
              variant="secondary"
              onClick={onFocusOnMap}
              leftIcon={<MapPin className="w-3.5 h-3.5 text-amber-400" />}
              title="Show on map"
            >
              Map
            </Button>
          )}
          <button
            onClick={onClose}
            aria-label="Close company details"
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-4 space-y-5 [scrollbar-gutter:stable]">
        {/* Verification & Presence State Card */}
        <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <span className="text-[11px] font-semibold text-slate-400 flex items-center gap-1">
                <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                Verification Confidence
              </span>
              <span className="text-xl font-mono font-bold text-emerald-400">{conf}%</span>
            </div>

            {/* Presence Badge */}
            <Badge
              variant={presenceInfo.variant}
              size="md"
              icon={<presenceInfo.Icon className="w-3.5 h-3.5" />}
              role="status"
              aria-label={`Presence: ${presenceInfo.label}`}
            >
              {presenceInfo.label}
            </Badge>
          </div>

          <p className="text-[11px] text-slate-400 leading-relaxed border-t border-slate-800/60 pt-2">
            {presenceInfo.description}
          </p>
        </div>

        {/* Overview Meta */}
        <div className="space-y-2.5">
          <h4 className="text-xs font-semibold text-slate-300">Company Overview</h4>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1">
              <span className="text-[10px] text-slate-400">Industry</span>
              <p className="font-medium text-slate-200">{company.industry || details?.company?.industry || 'Technology'}</p>
            </div>
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1">
              <span className="text-[10px] text-slate-400">Employees</span>
              <p className="font-medium text-slate-200">{company.employee_count || details?.company?.employee_count || 'Not specified'}</p>
            </div>
          </div>
          {(company.domain || details?.company?.domain) && (
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 flex items-center justify-between text-xs">
              <span className="text-slate-400 flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5 text-amber-400" />
                Domain
              </span>
              <a
                href={`https://${company.domain || details?.company?.domain}`}
                target="_blank"
                rel="noreferrer"
                className="text-amber-400 hover:underline font-mono"
              >
                {company.domain || details?.company?.domain}
              </a>
            </div>
          )}
        </div>

        {/* Recent Technical Jobs Section */}
        <div className="space-y-2.5">
          <div className="flex items-center justify-between">
            <h4 className="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <Briefcase className="w-3.5 h-3.5 text-sky-400" />
              Recent Technical Jobs
              <span className="text-[10px] text-slate-400 font-normal">
                (Last 14 days)
              </span>
            </h4>
            {technicalJobs.length > 0 && (
              <Badge variant="sky" size="sm">
                {technicalJobs.length} active
              </Badge>
            )}
          </div>

          {loadingTechJobs ? (
            <div className="py-4 text-center text-xs text-slate-400 space-y-2">
              <div className="w-5 h-5 border-2 border-sky-400 border-t-transparent rounded-full animate-spin mx-auto" />
              <p>Loading technical job postings...</p>
            </div>
          ) : technicalJobs.length === 0 ? (
            <div className="p-3 rounded-lg bg-slate-950 border border-slate-800/80 text-xs text-slate-400 text-center">
              No recent technical jobs recorded for this company within the last 14 days.
            </div>
          ) : (
            <div className="space-y-2">
              {technicalJobs.map((job) => {
                const arrangement = formatArrangement(job.work_arrangement);
                const isPostedRecent = isPostedWithin14Days(job);
                const postedFormatted = formatDate(job.posted_at);
                const observedFormatted = formatDate(job.last_seen_at || job.first_seen_at);

                return (
                  <div
                    key={job.id}
                    className="p-3 rounded-xl bg-slate-950 border border-slate-800/80 space-y-2 text-xs"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div>
                        <h5 className="font-semibold text-slate-200 text-xs">
                          {job.title}
                        </h5>
                        {job.canonical_url && (
                          <a
                            href={job.canonical_url}
                            target="_blank"
                            rel="noreferrer"
                            className="text-[11px] text-sky-400 hover:underline flex items-center gap-1 mt-0.5"
                          >
                            <span>View job posting</span>
                            <ExternalLink className="w-2.5 h-2.5" />
                          </a>
                        )}
                      </div>

                      {/* Source badge */}
                      <span className="text-[9px] font-mono px-1.5 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-400 uppercase shrink-0">
                        {job.source}
                      </span>
                    </div>

                    {/* Excerpt if present */}
                    {job.description_excerpt && (
                      <p className="text-[11px] text-slate-400 line-clamp-2 leading-relaxed">
                        {job.description_excerpt}
                      </p>
                    )}

                    {/* Badges: Arrangement & Recency Label */}
                    <div className="flex items-center gap-2 flex-wrap pt-1 border-t border-slate-800/60">
                      {/* Work arrangement */}
                      <Badge variant="slate" size="sm" role="status" aria-label={arrangement.ariaLabel}>
                        {arrangement.text}
                      </Badge>

                      {/* Recency label */}
                      {job.publication_state === 'stale' ? (
                        <Badge
                          variant="slate"
                          size="sm"
                          icon={<Clock className="w-2.5 h-2.5" />}
                          role="status"
                          aria-label="Stale job posting"
                        >
                          Stale
                        </Badge>
                      ) : isPostedRecent ? (
                        <Badge
                          variant="emerald"
                          size="sm"
                          icon={<Calendar className="w-2.5 h-2.5" />}
                          role="status"
                          aria-label="Posted within 14 days"
                        >
                          Posted within 14 days{postedFormatted ? ` (${postedFormatted})` : ''}
                        </Badge>
                      ) : (
                        <Badge
                          variant="amber"
                          size="sm"
                          icon={<Clock className="w-2.5 h-2.5" />}
                          role="status"
                          aria-label="Recently observed"
                        >
                          Recently observed{observedFormatted ? ` (${observedFormatted})` : ''}
                        </Badge>
                      )}

                      {/* Raw Location */}
                      {job.location_raw && (
                        <span className="text-[10px] text-slate-400 flex items-center gap-1">
                          <MapPin className="w-2.5 h-2.5 text-slate-500" />
                          <span className="truncate max-w-[150px]">{job.location_raw}</span>
                        </span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Registered Location */}
        <div className="space-y-2">
          <h4 className="text-xs font-semibold text-slate-300 flex items-center gap-1">
            <MapPin className="w-3.5 h-3.5 text-slate-400" />
            Registered Coordinates & Address
          </h4>
          <div className="p-3 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1.5 text-xs text-slate-300">
            <p>{company.address}</p>
            <p className="font-mono text-[11px] text-slate-400">
              Coordinates: {company.lat.toFixed(5)}, {company.lng.toFixed(5)}
            </p>
          </div>
          {details?.locations && details.locations.length > 1 && (
            <div className="space-y-1.5 pt-1">
              <span className="text-[10px] text-slate-400 font-semibold">
                Other Verified Offices ({details.locations.length - 1}):
              </span>
              <div className="space-y-1">
                {details.locations
                  .filter((loc) => loc.id !== company.location_id)
                  .map((loc) => (
                    <div
                      key={loc.id}
                      className="p-2 rounded bg-slate-950/60 border border-slate-800/50 text-[11px] text-slate-400"
                    >
                      <p>{loc.address || loc.label || 'Office location'}</p>
                      <span className="font-mono text-[10px] text-slate-400">
                        {loc.lat.toFixed(4)}, {loc.lng.toFixed(4)} • {Math.round(loc.confidence * 100)}% conf
                        {loc.presence_type ? ` • ${loc.presence_type}` : ''}
                      </span>
                    </div>
                  ))}
              </div>
            </div>
          )}
        </div>

        {/* Multi-Source Sightings */}
        <div className="space-y-2">
          <h4 className="text-xs font-semibold text-slate-300">Verified Data Sources</h4>
          {loadingSightings ? (
            <div className="py-4 text-center text-xs text-slate-400">Loading verified sources...</div>
          ) : (
            <SightingsTimeline sightings={sightingsData?.sightings || []} />
          )}
        </div>
      </div>
    </div>
  );
}
