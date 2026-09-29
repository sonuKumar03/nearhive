'use client';

import { useEffect, useState, useMemo } from 'react';
import { useCancelDiscovery, useDiscoveryJobs, useTriggerDiscovery } from '@/hooks/useDiscoveryJobs';
import { DiscoveryJob } from '@/types';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  defaultRegion?: string;
  defaultMode?: 'coordinates' | 'preset';
  currentCenter?: { lat: number; lng: number };
  currentRadiusKm?: number;
  onTriggerJob: (id: string) => void;
  onFocusLocation?: (lat: number, lng: number, radiusKm?: number) => void;
}

interface CityInfo {
  name: string;
  state: string;
  hub: string;
  lat: number;
  lng: number;
}

const CITIES: CityInfo[] = [
  { name: 'Hyderabad', state: 'Telangana', hub: 'Hitec City & Gachibowli', lat: 17.385, lng: 78.4867 },
  { name: 'Bangalore', state: 'Karnataka', hub: 'Koramangala & Whitefield', lat: 12.9716, lng: 77.5946 },
  { name: 'Pune', state: 'Maharashtra', hub: 'Hinjawadi & Magarpatta', lat: 18.5204, lng: 73.8567 },
  { name: 'Chennai', state: 'Tamil Nadu', hub: 'OMR & Guindy', lat: 13.0827, lng: 80.2707 },
  { name: 'Gurgaon', state: 'Haryana', hub: 'Cyber City & Golf Course Rd', lat: 28.4595, lng: 77.0266 },
  { name: 'Noida', state: 'Uttar Pradesh', hub: 'Sector 62 & Expressway', lat: 28.5355, lng: 77.391 },
];

const RADIUS_PRESETS = [
  { label: '5 km', value: 5, desc: 'Core Tech Hub' },
  { label: '15 km', value: 15, desc: 'Tech Corridor' },
  { label: '25 km', value: 25, desc: 'Greater Metro' },
];

function formatRelativeTime(isoString?: string): string {
  if (!isoString) return '';
  try {
    const diffMs = Date.now() - new Date(isoString).getTime();
    const diffSec = Math.floor(diffMs / 1000);
    if (diffSec < 60) return 'Just now';
    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHours = Math.floor(diffMin / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    const diffDays = Math.floor(diffHours / 24);
    if (diffDays === 1) return 'Yesterday';
    return `${diffDays}d ago`;
  } catch {
    return '';
  }
}

function resolveLocationName(lat?: number, lng?: number): string {
  if (lat === undefined || lng === undefined) return 'Custom Region';
  for (const city of CITIES) {
    const dLat = Math.abs(city.lat - lat);
    const dLng = Math.abs(city.lng - lng);
    // within ~35km
    if (dLat < 0.35 && dLng < 0.35) {
      return city.name;
    }
  }
  return `${lat.toFixed(3)}°, ${lng.toFixed(3)}°`;
}

export default function ScrapeModal({
  isOpen,
  onClose,
  defaultRegion = 'Hyderabad',
  defaultMode = 'preset',
  currentCenter,
  currentRadiusKm = 15,
  onTriggerJob,
  onFocusLocation,
}: Props) {
  const [mode, setMode] = useState<'preset' | 'coordinates'>(defaultMode);
  const [region, setRegion] = useState(defaultRegion);
  const [radiusKm, setRadiusKm] = useState(currentRadiusKm);
  const [error, setError] = useState('');

  const trigger = useTriggerDiscovery();
  const cancel = useCancelDiscovery();
  const { data } = useDiscoveryJobs();

  useEffect(() => {
    if (isOpen) {
      setMode(defaultMode);
      setRegion(defaultRegion);
      setRadiusKm(currentRadiusKm);
      setError('');
    }
  }, [isOpen, defaultMode, defaultRegion, currentRadiusKm]);

  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const selectedCity = CITIES.find((item) => item.name === region) || CITIES[0];
  const target = mode === 'coordinates' && currentCenter ? currentCenter : { lat: selectedCity.lat, lng: selectedCity.lng };
  const coverageArea = Math.round(Math.PI * radiusKm * radiusKm);

  async function start() {
    setError('');
    try {
      const job = await trigger.mutateAsync({
        lat: target.lat,
        lng: target.lng,
        radius_km: radiusKm,
      });
      onTriggerJob(job.id);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not launch discovery');
    }
  }

  const jobsList = data?.jobs || [];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4 animate-in fade-in duration-200"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="discovery-title"
        className="w-full max-w-lg max-h-[90vh] flex flex-col rounded-2xl border border-slate-800 bg-slate-900/95 text-slate-100 shadow-2xl shadow-black/80 overflow-hidden"
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-800/80 px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7" />
              </svg>
            </div>
            <div>
              <h2 id="discovery-title" className="text-base font-semibold tracking-tight text-white">
                Discover Companies & Jobs
              </h2>
              <p className="text-xs text-slate-400">
                Automated crawling across Google Maps, LinkedIn & technical careers
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white transition-colors"
          >
            ✕
          </button>
        </div>

        {/* Scrollable Body */}
        <div className="overflow-y-auto px-6 py-5 space-y-5">
          {/* Mode Switcher */}
          <div>
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">
              Target Mode
            </div>
            <div className="grid grid-cols-2 gap-1 rounded-xl bg-slate-950 p-1 border border-slate-800">
              <button
                type="button"
                onClick={() => setMode('preset')}
                className={`flex items-center justify-center gap-2 rounded-lg py-2 text-xs font-medium transition-all ${
                  mode === 'preset'
                    ? 'bg-slate-800 text-amber-400 shadow-sm border border-slate-700/80 font-semibold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <span>🏢</span> Major City Presets
              </button>
              <button
                type="button"
                onClick={() => setMode('coordinates')}
                className={`flex items-center justify-center gap-2 rounded-lg py-2 text-xs font-medium transition-all ${
                  mode === 'coordinates'
                    ? 'bg-slate-800 text-amber-400 shadow-sm border border-slate-700/80 font-semibold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <span>🎯</span> Map Viewport Focus
              </button>
            </div>
          </div>

          {/* Location Selection Form */}
          {mode === 'preset' ? (
            <div>
              <label htmlFor="city-select" className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1.5">
                Select City Hub
              </label>
              <div className="relative">
                <select
                  id="city-select"
                  value={region}
                  onChange={(event) => setRegion(event.target.value)}
                  className="w-full appearance-none rounded-xl border border-slate-700/80 bg-slate-800/90 px-3.5 py-2.5 text-sm text-slate-100 focus:border-amber-500 focus:outline-none focus:ring-1 focus:ring-amber-500 pr-10 cursor-pointer"
                >
                  {CITIES.map((city) => (
                    <option key={city.name} value={city.name}>
                      {city.name} ({city.state}) — {city.hub}
                    </option>
                  ))}
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3.5 text-slate-400">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                  </svg>
                </div>
              </div>
            </div>
          ) : (
            <div className="rounded-xl border border-slate-800 bg-slate-950/70 p-3.5 flex items-center justify-between">
              <div>
                <div className="text-[11px] uppercase tracking-wider text-slate-400 font-semibold">
                  Map Epicenter Focus
                </div>
                <div className="text-sm font-mono font-medium text-amber-400 mt-0.5">
                  {currentCenter
                    ? `${currentCenter.lat.toFixed(4)}°N, ${currentCenter.lng.toFixed(4)}°E`
                    : 'Current map center unavailable'}
                </div>
              </div>
              <span className="text-[11px] px-2.5 py-1 rounded-full bg-slate-800 text-slate-300 border border-slate-700 font-medium">
                Live Center
              </span>
            </div>
          )}

          {/* Radius Slider & Quick Presets */}
          <div className="space-y-2.5">
            <div className="flex items-center justify-between">
              <label htmlFor="radius-range" className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                Discovery Radius
              </label>
              <span className="text-xs font-mono font-semibold text-amber-400 bg-amber-500/10 px-2.5 py-0.5 rounded-full border border-amber-500/30">
                {radiusKm} km (~{coverageArea} km² coverage)
              </span>
            </div>

            {/* Quick chips */}
            <div className="grid grid-cols-3 gap-2">
              {RADIUS_PRESETS.map((p) => {
                const isActive = radiusKm === p.value;
                return (
                  <button
                    key={p.value}
                    type="button"
                    onClick={() => setRadiusKm(p.value)}
                    className={`px-3 py-1.5 rounded-xl text-xs font-medium border text-center transition-all ${
                      isActive
                        ? 'border-amber-500/80 bg-amber-500/15 text-amber-300 font-semibold shadow-sm'
                        : 'border-slate-800 bg-slate-800/40 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
                    }`}
                  >
                    <div>{p.label}</div>
                    <div className="text-[10px] opacity-75">{p.desc}</div>
                  </button>
                );
              })}
            </div>

            <input
              id="radius-range"
              type="range"
              min="1"
              max="30"
              value={radiusKm}
              onChange={(event) => setRadiusKm(Number(event.target.value))}
              className="w-full accent-amber-500 cursor-pointer h-1.5 bg-slate-800 rounded-lg mt-1"
            />
            <div className="flex justify-between text-[11px] text-slate-500 font-medium">
              <span>1 km (Hyper-local)</span>
              <span>15 km (Tech Hub)</span>
              <span>30 km (Full Metro)</span>
            </div>
          </div>

          {/* Error Message */}
          {error && (
            <div className="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-400 flex items-center gap-2">
              <span className="text-base">⚠️</span>
              <span>{error}</span>
            </div>
          )}

          {/* Primary Action Button */}
          <button
            type="button"
            onClick={start}
            disabled={trigger.isPending || (mode === 'coordinates' && !currentCenter)}
            className="w-full flex items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 px-4 py-3 font-semibold text-slate-950 shadow-lg shadow-amber-500/20 hover:from-amber-400 hover:to-amber-500 active:scale-[0.99] disabled:opacity-50 disabled:pointer-events-none transition-all cursor-pointer"
          >
            {trigger.isPending ? (
              <>
                <svg className="animate-spin h-4 w-4 text-slate-950" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                </svg>
                <span>Launching Discovery Crawlers...</span>
              </>
            ) : (
              <>
                <span>🚀</span>
                <span>
                  Start Discovery ({radiusKm} km in {mode === 'preset' ? region : 'Viewport'})
                </span>
              </>
            )}
          </button>

          {/* Recent Discoveries Section */}
          <div className="border-t border-slate-800/80 pt-4">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                Recent Discoveries
              </h3>
              <span className="text-[11px] text-slate-500 font-medium">Live sync</span>
            </div>

            {jobsList.length === 0 ? (
              <div className="rounded-xl border border-dashed border-slate-800 p-6 text-center text-xs text-slate-500">
                No recent discoveries yet. Start one above to map tech companies!
              </div>
            ) : (
              <ul className="space-y-2.5">
                {jobsList.slice(0, 5).map((job) => {
                  const isRunning = job.status === 'in_progress' || job.status === 'queued';
                  const isCompleted = job.status === 'completed';
                  const isCancelled = job.status === 'cancelled';
                  const locationName = resolveLocationName(job.lat, job.lng);
                  const timeAgo = formatRelativeTime(job.created_at);

                  return (
                    <li
                      key={job.id}
                      className="rounded-xl border border-slate-800 bg-slate-950/60 p-3 hover:border-slate-700/80 transition-all flex flex-col gap-2"
                    >
                      {/* Top row: Status, Location, Radius, Time, Cancel */}
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2 flex-wrap">
                          {isRunning && (
                            <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/30">
                              <span className="h-1.5 w-1.5 rounded-full bg-amber-400 animate-pulse" />
                              Running
                            </span>
                          )}
                          {isCompleted && (
                            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                              ✓ Completed
                            </span>
                          )}
                          {isCancelled && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-semibold bg-slate-800 text-slate-400 border border-slate-700/50">
                              Cancelled
                            </span>
                          )}

                          <span className="text-xs font-semibold text-slate-200">
                            📍 {locationName}
                          </span>
                          {job.radius_km && (
                            <span className="text-[11px] text-slate-400">
                              • {job.radius_km} km
                            </span>
                          )}
                        </div>

                        <div className="flex items-center gap-2">
                          {timeAgo && (
                            <span className="text-[11px] text-slate-500">{timeAgo}</span>
                          )}
                          {isRunning && (
                            <button
                              type="button"
                              onClick={() => cancel.mutate(job.id)}
                              className="text-[11px] px-2 py-0.5 rounded bg-rose-500/10 text-rose-400 border border-rose-500/30 hover:bg-rose-500/20 transition-colors font-medium"
                            >
                              Cancel
                            </button>
                          )}
                        </div>
                      </div>

                      {/* Bottom row: Statistics tags & Action button */}
                      <div className="flex items-center justify-between text-xs pt-1.5 border-t border-slate-800/40">
                        <div className="flex items-center gap-3 text-slate-300">
                          {isCompleted ? (
                            <>
                              <span className="text-emerald-400 font-semibold flex items-center gap-1 text-[11px]">
                                <span>🏢</span>
                                <span>{job.company_count ?? 0} Companies</span>
                              </span>
                              <span className="text-amber-300 font-semibold flex items-center gap-1 text-[11px]">
                                <span>💼</span>
                                <span>{job.job_count ?? 0} Tech Jobs</span>
                              </span>
                            </>
                          ) : isRunning ? (
                            <span className="text-slate-400 italic text-[11px] flex items-center gap-1">
                              <span>⟳</span>
                              <span>Scanning Google Maps, LinkedIn & Portals...</span>
                            </span>
                          ) : (
                            <span className="text-slate-500 text-[11px]">
                              Discovery ended without full crawl
                            </span>
                          )}
                        </div>

                        {isCompleted && onFocusLocation && job.lat !== undefined && job.lng !== undefined && (
                          <button
                            type="button"
                            onClick={() => {
                              onFocusLocation(job.lat!, job.lng!, job.radius_km);
                              onClose();
                            }}
                            className="text-[11px] text-amber-400 hover:text-amber-300 font-medium hover:underline flex items-center gap-1 cursor-pointer transition-colors"
                          >
                            <span>Focus Map</span>
                            <span>→</span>
                          </button>
                        )}
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
