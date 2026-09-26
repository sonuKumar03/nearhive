import { useState, useEffect } from 'react';
import { useTriggerScraper, useCancelScraper, useScrapeJobs } from '@/hooks/useScrapeJobs';
import { ScrapeTask } from '@/types';
import {
  X,
  Play,
  StopCircle,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  ArrowDownRight,
  MapPin,
  Building,
  Layers,
} from 'lucide-react';

interface ScrapeModalProps {
  isOpen: boolean;
  onClose: () => void;
  defaultRegion?: string;
  defaultMode?: 'coordinates' | 'preset';
  currentCenter?: { lat: number; lng: number };
  currentRadiusKm?: number;
  activeJobIds: string[];
  onTriggerJob: (id: string) => void;
}

const CITY_PRESET_OPTIONS = [
  { name: 'Bangalore', lat: 12.9716, lng: 77.5946, label: 'Bangalore (Electronic City, Whitefield, ORR)' },
  { name: 'Hyderabad', lat: 17.385, lng: 78.4867, label: 'Hyderabad (HITEC City, Gachibowli)' },
  { name: 'Pune', lat: 18.5204, lng: 73.8567, label: 'Pune (Hinjawadi, Magarpatta Cybercity)' },
  { name: 'Chennai', lat: 13.0827, lng: 80.2707, label: 'Chennai (OMR Corridor, Tidel Park)' },
  { name: 'Gurgaon', lat: 28.4595, lng: 77.0266, label: 'Gurgaon (DLF Cyber City, Udyog Vihar)' },
  { name: 'Noida', lat: 28.5355, lng: 77.391, label: 'Noida (Sector 62, Expressway IT Hub)' },
];

export default function ScrapeModal({
  isOpen,
  onClose,
  defaultRegion = 'Bangalore',
  defaultMode = 'preset',
  currentCenter,
  currentRadiusKm = 15,
  activeJobIds,
  onTriggerJob,
}: ScrapeModalProps) {
  const [mode, setMode] = useState<'coordinates' | 'preset'>(defaultMode);
  const [region, setRegion] = useState(defaultRegion);
  const [radiusKm, setRadiusKm] = useState(currentRadiusKm);
  const [errorText, setErrorText] = useState<string | null>(null);
  const [successNotice, setSuccessNotice] = useState<string | null>(null);

  // Re-sync selection state whenever the modal is opened
  useEffect(() => {
    if (isOpen) {
      if (defaultRegion) {
        setRegion(defaultRegion);
      }
      if (currentRadiusKm) {
        setRadiusKm(currentRadiusKm);
      }
      if (defaultMode) {
        setMode(defaultMode);
      }
      setErrorText(null);
      setSuccessNotice(null);
    }
  }, [isOpen, defaultRegion, currentRadiusKm, defaultMode]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
    }
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const triggerMutation = useTriggerScraper();
  const cancelMutation = useCancelScraper();
  const { data: jobsData } = useScrapeJobs();

  if (!isOpen) return null;

  const allJobs = jobsData?.jobs || [];
  // Show active jobs or recently completed jobs
  const relevantJobs = allJobs.slice(0, 4);
  const hasActiveJobs = triggerMutation.isPending || allJobs.some((j) => j.status === 'running' || j.status === 'pending');

  async function handleStart() {
    setErrorText(null);
    setSuccessNotice(null);
    try {
      if (mode === 'coordinates' && currentCenter) {
        const res = await triggerMutation.mutateAsync({
          region: `Loc(${currentCenter.lat.toFixed(3)}, ${currentCenter.lng.toFixed(3)})`,
          lat: currentCenter.lat,
          lng: currentCenter.lng,
          radius_km: radiusKm,
        });
        onTriggerJob(res.id);
        setSuccessNotice(`Queued scrape at (${currentCenter.lat.toFixed(3)}, ${currentCenter.lng.toFixed(3)})`);
      } else {
        const preset = CITY_PRESET_OPTIONS.find((p) => p.name === region);
        const res = await triggerMutation.mutateAsync({
          region,
          lat: preset?.lat,
          lng: preset?.lng,
          radius_km: radiusKm,
        });
        onTriggerJob(res.id);
        setSuccessNotice(`Queued scrape for ${region}`);
      }
    } catch (err: any) {
      setErrorText(err.message || 'Failed to trigger scraper');
    }
  }

  async function handleCancel(jobId: string) {
    try {
      await cancelMutation.mutateAsync(jobId);
    } catch (err: any) {
      setErrorText(err.message || 'Failed to cancel scraper');
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Run Scraper Pipeline"
      className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 animate-in fade-in duration-200"
    >
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg p-6 space-y-4 shadow-2xl animate-in zoom-in-95 duration-200 max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between shrink-0">
          <div className="flex items-center gap-2">
            <span className="text-2xl">🕷️</span>
            <div>
              <h3 className="font-bold text-slate-100 text-base">Run Scraper Pipeline</h3>
              <p className="text-xs text-slate-400">Multi-source: OpenStreetMap + Wikidata + Tech Parks</p>
            </div>
          </div>
          <button
            onClick={onClose}
            aria-label="Close scrape dialog"
            className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800 transition-colors cursor-pointer"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto space-y-4 pr-1">
          {/* Mode Selector Tabs */}
          <div className="flex rounded-xl bg-slate-950 p-1 border border-slate-800 text-xs">
            <button
              type="button"
              onClick={() => setMode('coordinates')}
              className={`flex-1 py-1.5 px-3 rounded-lg font-medium flex items-center justify-center gap-1.5 transition-all cursor-pointer ${
                mode === 'coordinates'
                  ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <MapPin className="w-3.5 h-3.5" />
              Current Map Location
            </button>
            <button
              type="button"
              onClick={() => setMode('preset')}
              className={`flex-1 py-1.5 px-3 rounded-lg font-medium flex items-center justify-center gap-1.5 transition-all cursor-pointer ${
                mode === 'preset'
                  ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <Building className="w-3.5 h-3.5" />
              Preset Tech Hub
            </button>
          </div>

          {/* Trigger Form Controls */}
          <div className="space-y-3 bg-slate-950/60 p-3.5 rounded-xl border border-slate-800/80">
            {mode === 'coordinates' ? (
              <div className="space-y-1 text-xs">
                <span className="text-slate-400 font-semibold block text-[11px] uppercase tracking-wider">
                  Target Coordinates
                </span>
                {currentCenter ? (
                  <div className="flex items-center justify-between font-mono text-slate-200 pt-0.5">
                    <span>Lat: {currentCenter.lat.toFixed(4)}</span>
                    <span>Lng: {currentCenter.lng.toFixed(4)}</span>
                  </div>
                ) : (
                  <p className="text-slate-400">Map coordinates not detected.</p>
                )}
                <p className="text-[10px] text-slate-400 pt-0.5">
                  Discovers tech offices centered around your active map location.
                </p>
              </div>
            ) : (
              <div>
                <label htmlFor="target-tech-hub-select" className="block text-xs font-semibold text-slate-300 mb-1">
                  Target Tech Hub
                </label>
                <select
                  id="target-tech-hub-select"
                  value={region}
                  onChange={(e) => setRegion(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-amber-500 cursor-pointer"
                >
                  {CITY_PRESET_OPTIONS.map((c) => (
                    <option key={c.name} value={c.name}>
                      {c.label}
                    </option>
                  ))}
                </select>
              </div>
            )}

            <div>
              <div className="flex items-center justify-between text-xs font-semibold text-slate-300 mb-1">
                <label htmlFor="scrape-radius-slider">Search Radius (km)</label>
                <span className="text-amber-400 font-mono">{radiusKm} km</span>
              </div>
              <input
                id="scrape-radius-slider"
                aria-label="Search radius in kilometers"
                type="range"
                min={5}
                max={30}
                step={1}
                value={radiusKm}
                onChange={(e) => setRadiusKm(Number(e.target.value))}
                className="w-full accent-amber-500 cursor-pointer"
              />
            </div>

            <div className="flex items-center justify-end pt-1">
              <button
                type="button"
                onClick={handleStart}
                disabled={triggerMutation.isPending}
                className="px-4 py-2 text-xs font-semibold rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-md shadow-amber-500/20 flex items-center gap-1.5 disabled:opacity-50 cursor-pointer transition-all"
              >
                {triggerMutation.isPending ? (
                  <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Play className="w-3.5 h-3.5 fill-current" />
                )}
                <span>Dispatch Scraper</span>
              </button>
            </div>
          </div>

          {/* Feedback Alerts */}
          {errorText && (
            <div className="p-2.5 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
              <AlertCircle className="w-4 h-4 shrink-0 text-rose-400" />
              <span>{errorText}</span>
            </div>
          )}

          {successNotice && (
            <div className="p-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
              <span>{successNotice}</span>
            </div>
          )}

          {/* Active & Recent Jobs List */}
          {relevantJobs.length > 0 && (
            <div className="space-y-2 pt-2 border-t border-slate-800">
              <div className="flex items-center justify-between text-xs text-slate-400">
                <span className="font-semibold flex items-center gap-1.5">
                  <Layers className="w-3.5 h-3.5 text-amber-400" />
                  Recent & Active Pipelines ({relevantJobs.length})
                </span>
                <span className="text-[10px] text-slate-400 font-mono">Concurrent Worker Fleet</span>
              </div>

              <div className="space-y-2">
                {relevantJobs.map((job) => {
                  const isJobActive = job.status === 'running' || job.status === 'pending';
                  const tasks = job.tasks || [];
                  return (
                    <div
                      key={job.id}
                      className="p-3 rounded-xl bg-slate-950 border border-slate-800/90 space-y-2 text-xs"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {isJobActive ? (
                            <RefreshCw className="w-3.5 h-3.5 text-amber-400 animate-spin" />
                          ) : job.status === 'done' ? (
                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                          ) : (
                            <AlertCircle className="w-3.5 h-3.5 text-slate-400" />
                          )}
                          <span className="font-semibold text-slate-200">
                            {job.region || 'Coordinates Scrape'}
                          </span>
                          <span className="text-[10px] text-slate-400 font-mono">
                            {job.radius_km ? `${job.radius_km} km` : ''}
                          </span>
                        </div>

                        <div className="flex items-center gap-2">
                          <span
                            className={`font-mono px-2 py-0.5 rounded font-semibold text-[10px] uppercase ${
                              job.status === 'done'
                                ? 'text-emerald-400 bg-emerald-500/10'
                                : job.status === 'cancelled'
                                ? 'text-slate-400 bg-slate-500/10'
                                : job.status === 'failed'
                                ? 'text-rose-400 bg-rose-500/10'
                                : 'text-amber-400 bg-amber-500/10'
                            }`}
                          >
                            {job.status}
                          </span>
                          {isJobActive && (
                            <button
                              type="button"
                              onClick={() => handleCancel(job.id)}
                              className="px-2 py-0.5 text-[10px] font-semibold rounded bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 transition-colors cursor-pointer"
                            >
                              Cancel
                            </button>
                          )}
                        </div>
                      </div>

                      {/* Discovered Stats & Task badges */}
                      <div className="flex items-center justify-between text-[11px] text-slate-400 pt-1 border-t border-slate-800/60">
                        <span>Discovered: <strong className="text-amber-400 font-mono">{job.sightings}</strong> sightings</span>
                        {tasks.length > 0 && (
                          <div className="flex items-center gap-1">
                            {tasks.map((t: ScrapeTask) => (
                              <span
                                key={t.id}
                                className="text-[9px] font-mono px-1.5 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300"
                              >
                                {t.source}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-slate-800 shrink-0">
          <span className="text-[11px] text-slate-400">
            {hasActiveJobs
              ? 'Jobs execute in parallel in the background.'
              : relevantJobs.length > 0
              ? 'All background pipelines completed.'
              : 'Ready to dispatch background pipelines.'}
          </span>
          {hasActiveJobs ? (
            <button
              type="button"
              onClick={onClose}
              className="px-3.5 py-1.5 text-xs font-semibold rounded-xl bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/40 flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <span>Run in Background</span>
              <ArrowDownRight className="w-3.5 h-3.5" />
            </button>
          ) : (
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-1.5 text-xs font-semibold rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/60 flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <span>Close</span>
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
