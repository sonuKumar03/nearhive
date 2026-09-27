import { useState, useEffect } from 'react';
import { useTriggerScraper, useCancelScraper, useScrapeJobs } from '@/hooks/useScrapeJobs';
import {
  useTriggerDiscovery,
  useCancelDiscovery,
  useDiscoveryJobs,
} from '@/hooks/useDiscoveryJobs';
import { ScrapeTask, DiscoveryJob, DiscoverySourceRun } from '@/types';
import {
  X,
  Play,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  ArrowDownRight,
  MapPin,
  Building,
  Layers,
  Sparkles,
  ChevronDown,
  ChevronUp,
  Cpu,
  Compass,
  Clock,
} from 'lucide-react';
import { useLiveTimer, getJobRuntimeInfo, formatDurationMs } from '@/lib/runtime';

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
  const [showLegacySection, setShowLegacySection] = useState(false);
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

  // Discovery hooks (Primary Python workflow)
  const triggerDiscoveryMutation = useTriggerDiscovery();
  const cancelDiscoveryMutation = useCancelDiscovery();
  const { data: discoveryJobsData } = useDiscoveryJobs();

  // Legacy Scraper hooks (Go worker workflow)
  const triggerLegacyMutation = useTriggerScraper();
  const cancelLegacyMutation = useCancelScraper();
  const { data: legacyJobsData } = useScrapeJobs();

  const discoveryJobs = discoveryJobsData?.jobs || [];
  const legacyJobs = legacyJobsData?.jobs || [];

  const relevantDiscoveryJobs = discoveryJobs.slice(0, 4);
  const relevantLegacyJobs = legacyJobs.slice(0, 3);

  const hasActiveDiscovery =
    triggerDiscoveryMutation.isPending ||
    discoveryJobs.some((j) => j.status === 'running' || j.status === 'pending');
  const hasActiveLegacy =
    triggerLegacyMutation.isPending ||
    legacyJobs.some((j) => j.status === 'running' || j.status === 'pending');
  const hasActiveJobs = hasActiveDiscovery || hasActiveLegacy;
  const now = useLiveTimer(isOpen && hasActiveJobs);

  if (!isOpen) return null;

  function getTargetCoordinates(): { lat: number; lng: number } {
    if (mode === 'coordinates' && currentCenter) {
      return { lat: currentCenter.lat, lng: currentCenter.lng };
    }
    const preset = CITY_PRESET_OPTIONS.find((p) => p.name === region);
    return { lat: preset?.lat ?? 12.9716, lng: preset?.lng ?? 77.5946 };
  }

  // Primary unified action: Run live spatial scraper (Go worker) + Python discovery
  async function handleStartAll() {
    setErrorText(null);
    setSuccessNotice(null);
    const { lat, lng } = getTargetCoordinates();
    const regionName = mode === 'coordinates' ? `Loc(${lat.toFixed(3)}, ${lng.toFixed(3)})` : region;

    try {
      // 1. Dispatch the live spatial Go scraper (OSM Overpass & Wikidata)
      const legacyRes = await triggerLegacyMutation.mutateAsync({
        region: regionName,
        lat,
        lng,
        radius_km: radiusKm,
      });
      onTriggerJob(legacyRes.id);

      // 2. Also dispatch Python Discovery in parallel for job & evidence enrichment
      try {
        const discRes = await triggerDiscoveryMutation.mutateAsync({
          lat,
          lng,
          radius_km: radiusKm,
        });
        onTriggerJob(discRes.id);
      } catch (discErr: any) {
        console.warn('Discovery trigger error:', discErr);
      }

      setSuccessNotice(`Dispatched company scraper for ${regionName} (${radiusKm} km radius)`);
    } catch (err: any) {
      setErrorText(err.message || 'Failed to dispatch company scraper');
    }
  }

  // Secondary action: Python Discovery Pipeline Only
  async function handleStartDiscoveryOnly() {
    setErrorText(null);
    setSuccessNotice(null);
    const { lat, lng } = getTargetCoordinates();

    try {
      const res = await triggerDiscoveryMutation.mutateAsync({
        lat,
        lng,
        radius_km: radiusKm,
      });
      onTriggerJob(res.id);
      setSuccessNotice(
        `Dispatched Python Job Discovery around (${lat.toFixed(3)}, ${lng.toFixed(3)}) with ${radiusKm}km radius.`
      );
    } catch (err: any) {
      setErrorText(err.message || 'Failed to dispatch discovery job');
    }
  }

  // Live Go scraper only
  async function handleStartLegacyScraper() {
    setErrorText(null);
    setSuccessNotice(null);
    const { lat, lng } = getTargetCoordinates();

    try {
      const res = await triggerLegacyMutation.mutateAsync({
        region: mode === 'coordinates' ? `Loc(${lat.toFixed(3)}, ${lng.toFixed(3)})` : region,
        lat,
        lng,
        radius_km: radiusKm,
      });
      onTriggerJob(res.id);
      setSuccessNotice(`Dispatched live Go scraper for ${region}`);
    } catch (err: any) {
      setErrorText(err.message || 'Failed to dispatch scraper');
    }
  }

  async function handleCancelDiscovery(jobId: string) {
    try {
      await cancelDiscoveryMutation.mutateAsync(jobId);
    } catch (err: any) {
      setErrorText(err.message || 'Failed to cancel discovery job');
    }
  }

  async function handleCancelLegacy(jobId: string) {
    try {
      await cancelLegacyMutation.mutateAsync(jobId);
    } catch (err: any) {
      setErrorText(err.message || 'Failed to cancel scraper');
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Company Discovery & Scraper Pipeline"
      className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 animate-in fade-in duration-200"
    >
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg p-6 space-y-4 shadow-2xl animate-in zoom-in-95 duration-200 max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between shrink-0">
          <div className="flex items-center gap-2">
            <span className="text-2xl">✨</span>
            <div>
              <h3 className="font-bold text-slate-100 text-base">Company Discovery</h3>
              <p className="text-xs text-slate-400">
                Multi-source Python Discovery: ATS Jobs, Directories & Registries
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            aria-label="Close discovery dialog"
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

          {/* Location Controls & Primary Trigger */}
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
                  Discovers tech companies and jobs centered around your active map location.
                </p>
              </div>
            ) : (
              <div>
                <label
                  htmlFor="target-tech-hub-select"
                  className="block text-xs font-semibold text-slate-300 mb-1"
                >
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
                min={1}
                max={30}
                step={1}
                value={radiusKm}
                onChange={(e) => setRadiusKm(Number(e.target.value))}
                className="w-full accent-amber-500 cursor-pointer"
              />
            </div>

            {/* Action Buttons */}
            <div className="flex items-center justify-end gap-2 pt-1 flex-wrap">
              <button
                type="button"
                onClick={handleStartDiscoveryOnly}
                disabled={triggerDiscoveryMutation.isPending || triggerLegacyMutation.isPending}
                className="px-3 py-2 text-xs font-semibold rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/80 flex items-center gap-1.5 disabled:opacity-50 cursor-pointer transition-all"
                title="Runs Python ATS job and directory discovery only"
              >
                {triggerDiscoveryMutation.isPending ? (
                  <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Sparkles className="w-3.5 h-3.5 text-amber-400" />
                )}
                <span>Job Discovery Only</span>
              </button>

              <button
                type="button"
                onClick={handleStartAll}
                disabled={triggerLegacyMutation.isPending || triggerDiscoveryMutation.isPending}
                className="px-4 py-2 text-xs font-semibold rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-md shadow-amber-500/20 flex items-center gap-1.5 disabled:opacity-50 cursor-pointer transition-all"
                title="Scrapes companies from OpenStreetMap & Wikidata within the selected radius"
              >
                {triggerLegacyMutation.isPending ? (
                  <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Play className="w-3.5 h-3.5 fill-current" />
                )}
                <span>Scrape Companies ({radiusKm} km)</span>
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

          {/* Active & Recent Spatial Scrape Jobs (Live OSM & Wikidata) */}
          {relevantLegacyJobs.length > 0 && (
            <div className="space-y-2 pt-2 border-t border-slate-800">
              <div className="flex items-center justify-between text-xs text-slate-400">
                <span className="font-semibold flex items-center gap-1.5 text-slate-200">
                  <span className="text-sm">🕷️</span>
                  Live Company Scrapes ({relevantLegacyJobs.length})
                </span>
                <span className="text-[10px] text-slate-400 font-mono">OpenStreetMap & Wikidata</span>
              </div>

              <div className="space-y-2">
                {relevantLegacyJobs.map((job) => {
                  const isLegacyActive = job.status === 'running' || job.status === 'pending';
                  const legacyRuntime = getJobRuntimeInfo(job, now);
                  return (
                    <div
                      key={job.id}
                      className="p-3 rounded-xl bg-slate-950 border border-slate-800/90 space-y-2 text-xs"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {isLegacyActive ? (
                            <RefreshCw className="w-3.5 h-3.5 text-amber-400 animate-spin" />
                          ) : job.status === 'done' ? (
                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                          ) : job.status === 'cancelled' ? (
                            <AlertCircle className="w-3.5 h-3.5 text-slate-400" />
                          ) : (
                            <AlertCircle className="w-3.5 h-3.5 text-rose-400" />
                          )}
                          <span className="font-semibold text-slate-200">
                            {job.region || 'Coordinates'}
                          </span>
                          {job.radius_km && (
                            <span className="text-[10px] text-slate-400 font-mono">
                              {job.radius_km} km
                            </span>
                          )}
                        </div>

                        <div className="flex items-center gap-2">
                          {legacyRuntime.text && (
                            <span
                              className={`font-mono px-2 py-0.5 rounded text-[10px] flex items-center gap-1 ${
                                legacyRuntime.isRunning
                                  ? 'text-amber-300 bg-amber-500/10 border border-amber-500/20'
                                  : legacyRuntime.isPending
                                  ? 'text-slate-400 bg-slate-900 border border-slate-800'
                                  : 'text-slate-400 bg-slate-900 border border-slate-800/80'
                              }`}
                            >
                              <Clock className={`w-2.5 h-2.5 ${legacyRuntime.isRunning ? 'text-amber-400 animate-spin' : 'text-slate-500'}`} />
                              <span>{legacyRuntime.text}</span>
                            </span>
                          )}
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
                          {isLegacyActive && (
                            <button
                              type="button"
                              onClick={() => handleCancelLegacy(job.id)}
                              className="px-2 py-0.5 text-[10px] font-semibold rounded bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 transition-colors cursor-pointer"
                            >
                              Cancel
                            </button>
                          )}
                        </div>
                      </div>

                      <div className="flex items-center justify-between text-[11px] text-slate-400 pt-1 border-t border-slate-800/60">
                        <span>
                          Sightings Discovered: <strong className="text-amber-400 font-mono">{job.sightings}</strong>
                        </span>
                        {job.error && (
                          <span className="text-rose-400 text-[10px] truncate max-w-[200px]" title={job.error}>
                            {job.error}
                          </span>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Active & Recent Python Discovery Jobs */}
          {relevantDiscoveryJobs.length > 0 && (
            <div className="space-y-2 pt-2 border-t border-slate-800">
              <div className="flex items-center justify-between text-xs text-slate-400">
                <span className="font-semibold flex items-center gap-1.5 text-slate-200">
                  <Compass className="w-3.5 h-3.5 text-amber-400" />
                  Discovery Jobs ({relevantDiscoveryJobs.length})
                </span>
                <span className="text-[10px] text-slate-400 font-mono">Python Workers</span>
              </div>

              <div className="space-y-2">
                {relevantDiscoveryJobs.map((job: DiscoveryJob) => {
                  const isActive = job.status === 'running' || job.status === 'pending';
                  const runtime = getJobRuntimeInfo(job, now);
                  const sourceRuns = job.source_runs || [];
                  return (
                    <div
                      key={job.id}
                      className="p-3 rounded-xl bg-slate-950 border border-slate-800/90 space-y-2 text-xs"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {isActive ? (
                            <RefreshCw className="w-3.5 h-3.5 text-amber-400 animate-spin" />
                          ) : job.status === 'completed' ? (
                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                          ) : job.status === 'partial' ? (
                            <AlertCircle className="w-3.5 h-3.5 text-amber-400" />
                          ) : (
                            <AlertCircle className="w-3.5 h-3.5 text-slate-400" />
                          )}
                          <span className="font-semibold text-slate-200">
                            Discovery ({job.lat.toFixed(3)}, {job.lng.toFixed(3)})
                          </span>
                          <span className="text-[10px] text-slate-400 font-mono">
                            {job.radius_km} km
                          </span>
                        </div>

                        <div className="flex items-center gap-2">
                          {runtime.text && (
                            <span
                              className={`font-mono px-2 py-0.5 rounded text-[10px] flex items-center gap-1 ${
                                runtime.isRunning
                                  ? 'text-amber-300 bg-amber-500/10 border border-amber-500/20'
                                  : runtime.isPending
                                  ? 'text-slate-400 bg-slate-900 border border-slate-800'
                                  : 'text-slate-400 bg-slate-900 border border-slate-800/80'
                              }`}
                            >
                              <Clock className={`w-2.5 h-2.5 ${runtime.isRunning ? 'text-amber-400 animate-spin' : 'text-slate-500'}`} />
                              <span>{runtime.text}</span>
                            </span>
                          )}
                          <span
                            className={`font-mono px-2 py-0.5 rounded font-semibold text-[10px] uppercase ${
                              job.status === 'completed'
                                ? 'text-emerald-400 bg-emerald-500/10'
                                : job.status === 'partial'
                                ? 'text-amber-400 bg-amber-500/10'
                                : job.status === 'cancelled'
                                ? 'text-slate-400 bg-slate-500/10'
                                : job.status === 'failed'
                                ? 'text-rose-400 bg-rose-500/10'
                                : 'text-amber-400 bg-amber-500/10'
                            }`}
                          >
                            {job.status}
                          </span>
                          {isActive && (
                            <button
                              type="button"
                              onClick={() => handleCancelDiscovery(job.id)}
                              disabled={cancelDiscoveryMutation.isPending}
                              className="px-2 py-0.5 text-[10px] font-semibold rounded bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 transition-colors cursor-pointer"
                            >
                              Cancel
                            </button>
                          )}
                        </div>
                      </div>

                      {/* Discovered Stats */}
                      <div className="flex items-center justify-between text-[11px] text-slate-400 pt-1 border-t border-slate-800/60">
                        <div className="flex items-center gap-3">
                          <span>
                            Companies: <strong className="text-amber-400 font-mono">{job.company_count}</strong>
                          </span>
                          <span>
                            Tech Jobs: <strong className="text-indigo-400 font-mono">{job.job_count}</strong>
                          </span>
                          <span>
                            Evidence: <strong className="text-slate-300 font-mono">{job.evidence_count}</strong>
                          </span>
                        </div>
                      </div>

                      {/* Source runs breakdown */}
                      {sourceRuns.length > 0 && (
                        <div className="flex items-center gap-1.5 flex-wrap pt-1">
                          {sourceRuns.map((sr: DiscoverySourceRun) => (
                            <span
                              key={sr.id}
                              className="text-[9px] font-mono px-1.5 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300 flex items-center gap-1"
                            >
                              <span className="font-semibold">{sr.source}</span>
                              <span className="text-amber-400">({sr.company_count} co / {sr.job_count} jobs)</span>
                              {sr.status === 'running' ? (
                                <span className="text-amber-300 font-semibold animate-pulse">• running</span>
                              ) : sr.duration_ms > 0 ? (
                                <span className="text-slate-400">• {formatDurationMs(sr.duration_ms)}</span>
                              ) : null}
                            </span>
                          ))}
                        </div>
                      )}
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
              ? 'Pipelines are running concurrently in the background.'
              : 'Ready to discover tech companies nearby.'}
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
