import { useState, useEffect } from 'react';
import { useTriggerScraper, useCancelScraper, useScrapeJob } from '@/hooks/useScrapeJobs';
import { ScrapeTask } from '@/types';
import { X, Play, StopCircle, RefreshCw, CheckCircle2, AlertCircle, ArrowDownRight, MapPin, Building } from 'lucide-react';

interface ScrapeModalProps {
  isOpen: boolean;
  onClose: () => void;
  defaultRegion?: string;
  currentCenter?: { lat: number; lng: number };
  currentRadiusKm?: number;
  activeJobId: string | null;
  setActiveJobId: (id: string | null) => void;
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
  currentCenter,
  currentRadiusKm = 15,
  activeJobId,
  setActiveJobId,
}: ScrapeModalProps) {
  const [mode, setMode] = useState<'coordinates' | 'preset'>('coordinates');
  const [region, setRegion] = useState(defaultRegion);
  const [radiusKm, setRadiusKm] = useState(currentRadiusKm);
  const [errorText, setErrorText] = useState<string | null>(null);

  useEffect(() => {
    if (currentRadiusKm) {
      setRadiusKm(currentRadiusKm);
    }
  }, [currentRadiusKm]);

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
  const { data: job } = useScrapeJob(activeJobId);

  if (!isOpen) return null;

  async function handleStart() {
    setErrorText(null);
    try {
      if (mode === 'coordinates' && currentCenter) {
        const res = await triggerMutation.mutateAsync({
          region: `Loc(${currentCenter.lat.toFixed(3)}, ${currentCenter.lng.toFixed(3)})`,
          lat: currentCenter.lat,
          lng: currentCenter.lng,
          radius_km: radiusKm,
        });
        setActiveJobId(res.id);
      } else {
        const preset = CITY_PRESET_OPTIONS.find((p) => p.name === region);
        const res = await triggerMutation.mutateAsync({
          region,
          lat: preset?.lat,
          lng: preset?.lng,
          radius_km: radiusKm,
        });
        setActiveJobId(res.id);
      }
    } catch (err: any) {
      setErrorText(err.message || 'Failed to trigger scraper');
    }
  }

  async function handleCancel() {
    if (!activeJobId) return;
    try {
      await cancelMutation.mutateAsync(activeJobId);
    } catch (err: any) {
      setErrorText(err.message || 'Failed to cancel scraper');
    }
  }

  const isRunning = job?.status === 'running' || job?.status === 'pending';

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Run Scraper Pipeline"
      className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 animate-in fade-in duration-200"
    >
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-5 shadow-2xl animate-in zoom-in-95 duration-200">
        <div className="flex items-center justify-between">
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

        {/* Mode Selector Tabs */}
        <div className="flex rounded-xl bg-slate-950 p-1 border border-slate-800 text-xs">
          <button
            type="button"
            onClick={() => setMode('coordinates')}
            disabled={isRunning}
            className={`flex-1 py-1.5 px-3 rounded-lg font-medium flex items-center justify-center gap-1.5 transition-all ${
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
            disabled={isRunning}
            className={`flex-1 py-1.5 px-3 rounded-lg font-medium flex items-center justify-center gap-1.5 transition-all ${
              mode === 'preset'
                ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Building className="w-3.5 h-3.5" />
            Preset Tech Hub
          </button>
        </div>

        <div className="space-y-4">
          {mode === 'coordinates' ? (
            <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800/80 space-y-1.5 text-xs">
              <span className="text-slate-400 font-semibold block text-[11px] uppercase tracking-wider">Dynamic Scrape Coordinates</span>
              {currentCenter ? (
                <div className="flex items-center justify-between font-mono text-slate-200 pt-1">
                  <span>Lat: {currentCenter.lat.toFixed(4)}</span>
                  <span>Lng: {currentCenter.lng.toFixed(4)}</span>
                </div>
              ) : (
                <p className="text-slate-400">Map coordinates not detected. Drag map marker or select preset.</p>
              )}
              <p className="text-[10px] text-slate-400 pt-1">
                Discovers tech offices centered around your active map location across open registries and spatial datasets.
              </p>
            </div>
          ) : (
            <div>
              <label htmlFor="target-tech-hub-select" className="block text-xs font-semibold text-slate-300 mb-1">Target Tech Hub</label>
              <select
                id="target-tech-hub-select"
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                disabled={isRunning}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-amber-500"
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
              disabled={isRunning}
              className="w-full accent-amber-500 cursor-pointer"
            />
          </div>

          {errorText && (
            <div className="p-2.5 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
              <AlertCircle className="w-4 h-4 shrink-0 text-rose-400" />
              <span>{errorText}</span>
            </div>
          )}

          {job && (
            <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5">
              <div className="flex items-center justify-between text-xs">
                <span className="text-slate-400">Status:</span>
                <span
                  className={`font-mono px-2 py-0.5 rounded font-semibold text-xs ${
                    job.status === 'done'
                      ? 'text-emerald-400 bg-emerald-500/10'
                      : job.status === 'cancelled'
                      ? 'text-slate-400 bg-slate-500/10'
                      : job.status === 'failed'
                      ? 'text-rose-400 bg-rose-500/10'
                      : 'text-amber-400 bg-amber-500/10'
                  }`}
                >
                  {job.status.toUpperCase()}
                </span>
              </div>

              {job.tasks && job.tasks.length > 0 && (
                <div className="space-y-1.5 pt-2 border-t border-slate-800 text-[11px]">
                  {job.tasks.map((task: ScrapeTask) => (
                    <div key={task.id} className="flex items-center justify-between py-0.5">
                      <div className="flex items-center gap-1.5">
                        {task.status === 'running' ? (
                          <RefreshCw className="w-3 h-3 text-amber-400 animate-spin" />
                        ) : task.status === 'done' ? (
                          <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                        ) : (
                          <AlertCircle className="w-3 h-3 text-slate-500" />
                        )}
                        <span className="text-slate-300 font-medium uppercase">{task.source}</span>
                      </div>
                      <span className="text-slate-500 font-mono text-[10px]">
                        {task.sightings} sightings ({task.duration_ms}ms)
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between gap-2.5 pt-2">
          {isRunning ? (
            <button
              onClick={handleCancel}
              className="px-3 py-1.5 text-xs font-semibold rounded-xl bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 flex items-center gap-1.5 cursor-pointer transition-colors"
            >
              <StopCircle className="w-3.5 h-3.5" />
              Cancel
            </button>
          ) : (
            <div />
          )}

          <div className="flex items-center gap-2">
            {isRunning ? (
              <button
                onClick={onClose}
                className="px-3.5 py-1.5 text-xs font-semibold rounded-xl bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/40 flex items-center gap-1.5 transition-colors cursor-pointer"
              >
                <span>Run in Background</span>
                <ArrowDownRight className="w-3.5 h-3.5" />
              </button>
            ) : (
              <button
                onClick={onClose}
                className="px-3.5 py-1.5 text-xs font-medium rounded-xl text-slate-400 hover:text-slate-200 transition-colors"
              >
                Close
              </button>
            )}

            {!isRunning && (
              <button
                onClick={handleStart}
                disabled={triggerMutation.isPending}
                className="px-4 py-2 text-xs font-semibold rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-md shadow-amber-500/20 flex items-center gap-1.5 disabled:opacity-50 cursor-pointer transition-all"
              >
                <Play className="w-3.5 h-3.5" />
                Start Scraping
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
