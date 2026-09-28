'use client';

import { useEffect, useState } from 'react';
import { useCancelDiscovery, useDiscoveryJobs, useTriggerDiscovery } from '@/hooks/useDiscoveryJobs';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  defaultRegion?: string;
  defaultMode?: 'coordinates' | 'preset';
  currentCenter?: { lat: number; lng: number };
  currentRadiusKm?: number;
  onTriggerJob: (id: string) => void;
}

const cities = [
  { name: 'Bangalore', lat: 12.9716, lng: 77.5946 },
  { name: 'Hyderabad', lat: 17.385, lng: 78.4867 },
  { name: 'Pune', lat: 18.5204, lng: 73.8567 },
  { name: 'Chennai', lat: 13.0827, lng: 80.2707 },
  { name: 'Gurgaon', lat: 28.4595, lng: 77.0266 },
  { name: 'Noida', lat: 28.5355, lng: 77.391 },
];

export default function ScrapeModal({ isOpen, onClose, defaultRegion = 'Bangalore', defaultMode = 'preset', currentCenter, currentRadiusKm = 15, onTriggerJob }: Props) {
  const [mode, setMode] = useState(defaultMode);
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
    const close = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', close);
    return () => window.removeEventListener('keydown', close);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  async function start() {
    const city = cities.find((item) => item.name === region) || cities[0];
    const target = mode === 'coordinates' && currentCenter ? currentCenter : city;
    setError('');
    try {
      const job = await trigger.mutateAsync({ lat: target.lat, lng: target.lng, radius_km: radiusKm });
      onTriggerJob(job.id);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not start discovery');
    }
  }

  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 p-4" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <section role="dialog" aria-modal="true" aria-labelledby="discovery-title" className="w-full max-w-lg rounded-2xl border border-slate-700 bg-slate-900 p-6 text-slate-100 shadow-2xl">
      <div className="flex items-center justify-between">
        <h2 id="discovery-title" className="text-lg font-semibold">Discover companies and jobs</h2>
        <button type="button" onClick={onClose} aria-label="Close" className="text-slate-400 hover:text-white">✕</button>
      </div>
      <p className="mt-2 text-sm text-slate-400">Find nearby tech companies and recent technical jobs.</p>
      <div className="mt-5 flex gap-2">
        <button type="button" onClick={() => setMode('preset')} className={`rounded-lg px-3 py-2 text-sm ${mode === 'preset' ? 'bg-amber-500 text-black' : 'bg-slate-800'}`}>City</button>
        <button type="button" onClick={() => setMode('coordinates')} className={`rounded-lg px-3 py-2 text-sm ${mode === 'coordinates' ? 'bg-amber-500 text-black' : 'bg-slate-800'}`}>Map center</button>
      </div>
      {mode === 'preset' ? <label className="mt-4 block text-sm">City
        <select value={region} onChange={(event) => setRegion(event.target.value)} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-800 p-2">
          {cities.map((city) => <option key={city.name} value={city.name}>{city.name}</option>)}
        </select>
      </label> : <p className="mt-4 text-sm text-slate-300">{currentCenter ? `${currentCenter.lat.toFixed(4)}, ${currentCenter.lng.toFixed(4)}` : 'Current map center unavailable'}</p>}
      <label className="mt-4 block text-sm">Radius: {radiusKm} km
        <input type="range" min="1" max="30" value={radiusKm} onChange={(event) => setRadiusKm(Number(event.target.value))} className="mt-2 w-full accent-amber-500" />
      </label>
      {error && <p role="alert" className="mt-3 text-sm text-red-400">{error}</p>}
      <button type="button" onClick={start} disabled={trigger.isPending || (mode === 'coordinates' && !currentCenter)} className="mt-5 w-full rounded-lg bg-amber-500 px-4 py-2 font-semibold text-black disabled:opacity-50">{trigger.isPending ? 'Starting…' : 'Start discovery'}</button>
      {(data?.jobs || []).length > 0 && <div className="mt-6 border-t border-slate-700 pt-4">
        <h3 className="text-sm font-semibold">Recent discoveries</h3>
        <ul className="mt-2 space-y-2 text-sm">{data?.jobs.slice(0, 5).map((job) => <li key={job.id} className="flex items-center justify-between rounded-lg bg-slate-800 p-2">
          <span>{job.status.replace('_', ' ')}</span>
          {(job.status === 'queued' || job.status === 'in_progress') && <button type="button" onClick={() => cancel.mutate(job.id)} className="text-amber-400 hover:text-amber-300">Cancel</button>}
        </li>)}</ul>
      </div>}
    </section>
  </div>;
}
