import { useState } from 'react';
import { useTriggerScraper, useCancelScraper, useScrapeJob } from '@/hooks/useScrapeJobs';
import { ScrapeTask } from '@/types';
import { X, Play, StopCircle, RefreshCw, CheckCircle2, AlertCircle } from 'lucide-react';

interface ScrapeModalProps {
  isOpen: boolean;
  onClose: () => void;
  defaultRegion: string;
}

export default function ScrapeModal({ isOpen, onClose, defaultRegion }: ScrapeModalProps) {
  if (!isOpen) return null;

  const [region, setRegion] = useState(defaultRegion || 'Bangalore');
  const [radiusKm, setRadiusKm] = useState(15);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);

  const triggerMutation = useTriggerScraper();
  const cancelMutation = useCancelScraper();
  const { data: job } = useScrapeJob(activeJobId);

  async function handleStart() {
    try {
      const res = await triggerMutation.mutateAsync({ region, radius_km: radiusKm });
      setActiveJobId(res.id);
    } catch (err) {
      console.error('Trigger error:', err);
    }
  }

  async function handleCancel() {
    if (!activeJobId) return;
    try {
      await cancelMutation.mutateAsync(activeJobId);
    } catch (err) {
      console.error('Cancel error:', err);
    }
  }

  const isRunning = job?.status === 'running' || job?.status === 'pending';

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-5 shadow-2xl">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-2xl">🕷️</span>
            <div>
              <h3 className="font-bold text-slate-100 text-base">Run Scraper Pipeline</h3>
              <p className="text-xs text-slate-400">Multi-source: OSM Overpass + Wikidata SPARQL + Tech Parks</p>
            </div>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">Target Tech Hub</label>
            <select
              value={region}
              onChange={(e) => setRegion(e.target.value)}
              disabled={isRunning}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-amber-500"
            >
              <option value="Bangalore">Bangalore (Electronic City, Whitefield, Outer Ring Road)</option>
              <option value="Hyderabad">Hyderabad (HITEC City, Gachibowli, Financial Dist)</option>
              <option value="Pune">Pune (Hinjawadi, Magarpatta Cybercity)</option>
              <option value="Chennai">Chennai (OMR Corridor, Tidel Park, DLF Cybercity)</option>
              <option value="Gurgaon">Gurgaon (DLF Cyber City, Udyog Vihar)</option>
              <option value="Noida">Noida (Sector 62, Expressway IT Hub)</option>
            </select>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">Search Radius (km)</label>
            <input
              type="number"
              min={5}
              max={30}
              value={radiusKm}
              onChange={(e) => setRadiusKm(Number(e.target.value))}
              disabled={isRunning}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-amber-500"
            />
          </div>

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
              className="px-3.5 py-1.5 text-xs font-semibold rounded-xl bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 flex items-center gap-1.5"
            >
              <StopCircle className="w-3.5 h-3.5" />
              Cancel Scrape
            </button>
          ) : (
            <div />
          )}

          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="px-3.5 py-1.5 text-xs font-medium rounded-xl text-slate-400 hover:text-slate-200"
            >
              Close
            </button>
            <button
              onClick={handleStart}
              disabled={isRunning || triggerMutation.isPending}
              className="px-4 py-2 text-xs font-semibold rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-md shadow-amber-500/20 flex items-center gap-1.5 disabled:opacity-50"
            >
              <Play className="w-3.5 h-3.5" />
              Start Scraping
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
