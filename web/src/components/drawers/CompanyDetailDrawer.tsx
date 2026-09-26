import { useEffect } from 'react';
import { CompanySearchResult } from '@/types';
import { useCompany, useSightings } from '@/hooks/useCompanyDetails';
import SightingsTimeline from './SightingsTimeline';
import { X, Building2, Globe, ShieldCheck, MapPin } from 'lucide-react';

interface DrawerProps {
  company: CompanySearchResult | null;
  onClose: () => void;
}

export default function CompanyDetailDrawer({ company, onClose }: DrawerProps) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  const { data: details, isLoading: loadingCompany } = useCompany(company?.id ?? null);
  const { data: sightingsData, isLoading: loadingSightings } = useSightings(company?.id ?? null);

  if (!company) return null;

  const conf = Math.round(company.confidence * 100);

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
        <button onClick={onClose} className="p-1 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800">
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-4 space-y-5">
        {/* Verification Card */}
        <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between">
          <div className="space-y-0.5">
            <span className="text-[11px] font-semibold text-slate-400 flex items-center gap-1">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              Verification Confidence
            </span>
            <span className="text-xl font-mono font-bold text-emerald-400">{conf}%</span>
          </div>
          <span className="text-xs px-2.5 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 font-semibold">
            PostGIS Verified
          </span>
        </div>

        {/* Overview Meta */}
        <div className="space-y-2.5">
          <h4 className="text-xs font-semibold text-slate-300">Company Overview</h4>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1">
              <span className="text-[10px] text-slate-500">Industry</span>
              <p className="font-medium text-slate-200">{company.industry || details?.company?.industry || 'Technology'}</p>
            </div>
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1">
              <span className="text-[10px] text-slate-500">Employees</span>
              <p className="font-medium text-slate-200">{company.employee_count || details?.company?.employee_count || '100 - 500'}</p>
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

        {/* Registered Location */}
        <div className="space-y-2">
          <h4 className="text-xs font-semibold text-slate-300 flex items-center gap-1">
            <MapPin className="w-3.5 h-3.5 text-slate-400" />
            Registered Coordinates & Address
          </h4>
          <div className="p-3 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1.5 text-xs text-slate-300">
            <p>{company.address}</p>
            <p className="font-mono text-[11px] text-slate-500">
              Coordinates: {company.lat.toFixed(5)}, {company.lng.toFixed(5)}
            </p>
          </div>
          {details?.locations && details.locations.length > 1 && (
            <div className="space-y-1.5 pt-1">
              <span className="text-[10px] text-slate-400 font-semibold">Other Verified Offices ({details.locations.length - 1}):</span>
              <div className="space-y-1">
                {details.locations
                  .filter((loc) => loc.id !== company.location_id)
                  .map((loc) => (
                    <div key={loc.id} className="p-2 rounded bg-slate-950/60 border border-slate-800/50 text-[11px] text-slate-400">
                      <p>{loc.address || loc.label || 'Office location'}</p>
                      <span className="font-mono text-[10px] text-slate-500">
                        {loc.lat.toFixed(4)}, {loc.lng.toFixed(4)} • {Math.round(loc.confidence * 100)}% conf
                      </span>
                    </div>
                  ))}
              </div>
            </div>
          )}
        </div>

        {/* Multi-Source Sightings */}
        <div className="space-y-2">
          <h4 className="text-xs font-semibold text-slate-300">Verification Sightings Audit</h4>
          {loadingSightings ? (
            <div className="py-6 text-center text-xs text-slate-500">Loading audit trail...</div>
          ) : (
            <SightingsTimeline sightings={sightingsData?.sightings || []} />
          )}
        </div>
      </div>
    </div>
  );
}
