'use client';

import { useEffect, useState } from 'react';
import { useCancelDiscovery, useDiscoveryJobs, useTriggerDiscovery } from '@/hooks/useDiscoveryJobs';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { SegmentedControl } from '@/components/ui/SegmentedControl';
import { Select } from '@/components/ui/Select';
import { Slider } from '@/components/ui/Slider';
import { Search, Compass, MapPin, Building2, Briefcase, AlertTriangle, ArrowRight } from 'lucide-react';

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

  const cityOptions = CITIES.map((city) => ({
    value: city.name,
    label: `${city.name} (${city.state}) — ${city.hub}`,
  }));

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Discover Companies & Jobs"
      subtitle="Automated crawling across Google Maps, LinkedIn & technical careers"
      icon={<Search className="w-5 h-5 text-amber-400" />}
      maxWidth="lg"
    >
      <div className="space-y-5">
        {/* Mode Switcher */}
        <div>
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">
            Target Mode
          </div>
          <SegmentedControl<'preset' | 'coordinates'>
            options={[
              { value: 'preset', label: 'Major City Presets', icon: <Building2 className="w-3.5 h-3.5" /> },
              { value: 'coordinates', label: 'Map Viewport Focus', icon: <Compass className="w-3.5 h-3.5" /> },
            ]}
            value={mode}
            onChange={setMode}
          />
        </div>

        {/* Location Selection Form */}
        {mode === 'preset' ? (
          <div>
            <Select
              id="city-select"
              label="Select City Hub"
              value={region}
              onChange={(e) => setRegion(e.target.value)}
              options={cityOptions}
            />
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
            <Badge variant="slate" size="sm">
              Live Center
            </Badge>
          </div>
        )}

        {/* Radius Slider & Quick Presets */}
        <Slider
          id="radius-range"
          label="Discovery Radius"
          badgeText={`${radiusKm} km (~${coverageArea} km² coverage)`}
          value={radiusKm}
          onChange={setRadiusKm}
          min={1}
          max={30}
          presets={RADIUS_PRESETS}
          scaleLabels={['1 km (Hyper-local)', '15 km (Tech Hub)', '30 km (Full Metro)']}
        />

        {/* Error Message */}
        {error && (
          <div className="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-400 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Primary Action Button */}
        <Button
          variant="primary"
          size="lg"
          onClick={start}
          isLoading={trigger.isPending}
          disabled={mode === 'coordinates' && !currentCenter}
          className="w-full text-sm font-semibold py-3"
          leftIcon={!trigger.isPending ? <span>🚀</span> : undefined}
        >
          {trigger.isPending
            ? 'Launching Discovery Crawlers...'
            : `Start Discovery (${radiusKm} km in ${mode === 'preset' ? region : 'Viewport'})`}
        </Button>

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
                          <Badge variant="amber" pulsing>
                            Running
                          </Badge>
                        )}
                        {isCompleted && (
                          <Badge variant="emerald" icon={<span>✓</span>}>
                            Completed
                          </Badge>
                        )}
                        {isCancelled && (
                          <Badge variant="slate">
                            Cancelled
                          </Badge>
                        )}

                        <span className="text-xs font-semibold text-slate-200 flex items-center gap-1">
                          <MapPin className="w-3 h-3 text-amber-400" />
                          {locationName}
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
                          <Button
                            size="xs"
                            variant="danger"
                            onClick={() => cancel.mutate(job.id)}
                          >
                            Cancel
                          </Button>
                        )}
                      </div>
                    </div>

                    {/* Bottom row: Statistics tags & Action button */}
                    <div className="flex items-center justify-between text-xs pt-1.5 border-t border-slate-800/40">
                      <div className="flex items-center gap-3 text-slate-300">
                        {isCompleted ? (
                          <>
                            <span className="text-emerald-400 font-semibold flex items-center gap-1 text-[11px]">
                              <Building2 className="w-3 h-3" />
                              <span>{job.company_count ?? 0} Companies</span>
                            </span>
                            <span className="text-amber-300 font-semibold flex items-center gap-1 text-[11px]">
                              <Briefcase className="w-3 h-3" />
                              <span>{job.job_count ?? 0} Tech Jobs</span>
                            </span>
                          </>
                        ) : isRunning ? (
                          <span className="text-slate-400 italic text-[11px] flex items-center gap-1">
                            <span className="animate-spin">⟳</span>
                            <span>Scanning Google Maps, LinkedIn & Portals...</span>
                          </span>
                        ) : (
                          <span className="text-slate-500 text-[11px]">
                            Discovery ended without full crawl
                          </span>
                        )}
                      </div>

                      {isCompleted && onFocusLocation && job.lat !== undefined && job.lng !== undefined && (
                        <Button
                          size="xs"
                          variant="ghost"
                          className="text-amber-400 hover:text-amber-300 h-6 px-1.5"
                          rightIcon={<ArrowRight className="w-3 h-3" />}
                          onClick={() => {
                            onFocusLocation(job.lat!, job.lng!, job.radius_km);
                            onClose();
                          }}
                        >
                          Focus Map
                        </Button>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </div>
    </Modal>
  );
}
