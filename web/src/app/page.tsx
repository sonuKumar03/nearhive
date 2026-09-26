'use client';

import { useState, useEffect } from 'react';
import MapContainer from '@/components/map/MapContainer';
import Sidebar from '@/components/sidebar/Sidebar';
import CompanyDetailDrawer from '@/components/drawers/CompanyDetailDrawer';
import ScrapeModal from '@/components/drawers/ScrapeModal';
import BackgroundScrapeWidget from '@/components/scrapers/BackgroundScrapeWidget';
import { useCompanies } from '@/hooks/useCompanies';
import { useClusters } from '@/hooks/useClusters';
import { useScrapeJob } from '@/hooks/useScrapeJobs';
import { useAuth } from '@/hooks/useAuth';
import { useGeolocation } from '@/hooks/useGeolocation';
import { CompanySearchResult } from '@/types';
import { Play, Layers, RefreshCw, LocateFixed, Loader2 } from 'lucide-react';

const CITY_PRESETS = [
  { name: 'Bangalore', lat: 12.9716, lng: 77.5946 },
  { name: 'Hyderabad', lat: 17.385, lng: 78.4867 },
  { name: 'Pune', lat: 18.5204, lng: 73.8567 },
  { name: 'Chennai', lat: 13.0827, lng: 80.2707 },
  { name: 'Gurgaon', lat: 28.4595, lng: 77.0266 },
  { name: 'Noida', lat: 28.5355, lng: 77.391 },
];

export default function HomePage() {
  const [center, setCenter] = useState({ lat: 12.9716, lng: 77.5946 });
  const [radiusKm, setRadiusKm] = useState(15);
  const [searchQuery, setSearchQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [isClusterMode, setIsClusterMode] = useState(false);
  const [selectedCompany, setSelectedCompany] = useState<CompanySearchResult | null>(null);
  const [isScrapeOpen, setIsScrapeOpen] = useState(false);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);

  // Debounce search input by 300ms to avoid flooding PostGIS
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(searchQuery);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  // Browser Geolocation hook
  const { getCurrentLocation, loading: geoLoading, error: geoError } = useGeolocation();

  async function handleLocateMe() {
    try {
      const coords = await getCurrentLocation();
      setCenter(coords);
    } catch (err) {
      console.warn('Geolocation failed:', err);
    }
  }

  // Initialize guest session
  useAuth();

  // Track active background scraping job
  const { data: activeJob } = useScrapeJob(activeJobId);

  const { data: searchData, isLoading: loadingCompanies } = useCompanies({
    lat: center.lat,
    lng: center.lng,
    radius_km: radiusKm,
    q: debouncedQuery,
  });

  const { data: clusterData } = useClusters(
    {
      lat: center.lat,
      lng: center.lng,
      radius_km: radiusKm,
      k: 20,
    },
    isClusterMode
  );

  const companies = searchData?.companies || [];
  const clusters = clusterData?.clusters || [];
  const totalCount = searchData?.meta?.total ?? companies.length;
  const isJobRunning = activeJob?.status === 'running' || activeJob?.status === 'pending';

  return (
    <div className="h-screen w-screen flex flex-col bg-slate-950">
      {/* Top Header */}
      <header className="h-14 border-b border-slate-800/80 bg-slate-900/90 backdrop-blur-md px-4 flex items-center justify-between shrink-0 z-20">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-amber-400 to-amber-600 flex items-center justify-center text-lg shadow-lg shadow-amber-500/20">
            🐝
          </div>
          <div>
            <div className="flex items-center gap-1.5">
              <span className="font-bold text-base tracking-tight bg-gradient-to-r from-amber-200 to-amber-500 bg-clip-text text-transparent">
                NearHive
              </span>
              <span className="text-[9px] font-mono px-1 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
                POSTGIS
              </span>
            </div>
          </div>

          <div className="h-4 w-px bg-slate-800 hidden md:block mx-1" />

          {/* Current Location Button */}
          <button
            onClick={handleLocateMe}
            disabled={geoLoading}
            className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-lg bg-amber-500/15 hover:bg-amber-500/25 text-amber-300 border border-amber-500/30 transition-all cursor-pointer font-medium disabled:opacity-50"
            title={geoError || 'Locate around current browser location'}
          >
            {geoLoading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <LocateFixed className="w-3.5 h-3.5 text-amber-400" />
            )}
            <span>Locate Me</span>
          </button>

          {/* City Presets */}
          <div className="hidden lg:flex items-center gap-1">
            {CITY_PRESETS.map((c) => (
              <button
                key={c.name}
                onClick={() => setCenter({ lat: c.lat, lng: c.lng })}
                className="px-2 py-1 text-xs rounded-lg bg-slate-800/80 hover:bg-slate-700 text-slate-300 border border-slate-700/40 transition-colors cursor-pointer"
              >
                {c.name}
              </button>
            ))}
          </div>
        </div>

        {/* Header Right Controls */}
        <div className="flex items-center gap-3">
          {/* Active Background Scraper Header Pill */}
          {activeJob && isJobRunning && (
            <button
              onClick={() => setIsScrapeOpen(true)}
              className="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-xl bg-amber-500/10 border border-amber-500/40 text-amber-400 text-xs hover:bg-amber-500/20 transition-all cursor-pointer animate-pulse"
              title="Click to view full scraper tasks"
            >
              <RefreshCw className="w-3.5 h-3.5 animate-spin" />
              <span className="font-semibold">Crawling {activeJob.region}...</span>
              <span className="font-mono text-[10px] bg-amber-500/20 px-1.5 py-0.5 rounded text-amber-300 font-bold">
                {activeJob.sightings} sightings
              </span>
            </button>
          )}

          {/* Radius Slider */}
          <div className="flex items-center gap-2 bg-slate-950/80 px-2.5 py-1 rounded-xl border border-slate-800">
            <span className="text-xs text-slate-400 font-medium">Radius:</span>
            <input
              type="range"
              min={1}
              max={30}
              value={radiusKm}
              onChange={(e) => setRadiusKm(Number(e.target.value))}
              className="w-20 md:w-28 accent-amber-500 cursor-pointer h-1.5 bg-slate-800 rounded-lg"
            />
            <span className="text-xs font-mono font-bold text-amber-400 min-w-[36px]">{radiusKm} km</span>
          </div>

          <button
            onClick={() => setIsScrapeOpen(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-md shadow-amber-500/20 transition-all cursor-pointer"
          >
            <Play className="w-3.5 h-3.5" />
            <span>Scrape Tech Hub</span>
          </button>
        </div>
      </header>

      {/* Main Map + Sidebar Canvas */}
      <div className="flex-1 flex relative overflow-hidden">
        <Sidebar
          companies={companies}
          isLoading={loadingCompanies}
          totalCount={totalCount}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          onSelectCompany={setSelectedCompany}
        />

        {/* Map Canvas */}
        <div className="flex-1 h-full relative">
          {/* Floating Cluster Toggle */}
          <div className="absolute top-4 right-4 z-20">
            <button
              onClick={() => setIsClusterMode(!isClusterMode)}
              className={`px-3 py-1.5 rounded-xl shadow-xl text-xs font-semibold backdrop-blur-md border transition-all flex items-center gap-1.5 cursor-pointer ${
                isClusterMode
                  ? 'bg-amber-500/20 text-amber-300 border-amber-500/50'
                  : 'bg-slate-900/90 border-slate-700/80 text-slate-200 hover:text-amber-400'
              }`}
            >
              <Layers className="w-3.5 h-3.5 text-amber-400" />
              <span>{isClusterMode ? 'Show Pins View' : 'Cluster View (K-Means)'}</span>
            </button>
          </div>

          <MapContainer
            center={center}
            radiusKm={radiusKm}
            companies={companies}
            clusters={clusters}
            isClusterMode={isClusterMode}
            onCenterChange={(lat, lng) => setCenter({ lat, lng })}
            onSelectCompany={setSelectedCompany}
            onClusterZoom={(lat, lng) => {
              setCenter({ lat, lng });
              setIsClusterMode(false);
            }}
          />
        </div>

        {/* Company Detail Drawer */}
        <CompanyDetailDrawer company={selectedCompany} onClose={() => setSelectedCompany(null)} />

        {/* Scrape Modal */}
        <ScrapeModal
          isOpen={isScrapeOpen}
          onClose={() => setIsScrapeOpen(false)}
          defaultRegion="Bangalore"
          currentCenter={center}
          currentRadiusKm={radiusKm}
          activeJobId={activeJobId}
          setActiveJobId={setActiveJobId}
        />

        {/* Floating Background Scraper Widget */}
        {!isScrapeOpen && (
          <BackgroundScrapeWidget
            jobId={activeJobId}
            onOpenDetails={() => setIsScrapeOpen(true)}
            onDismiss={() => setActiveJobId(null)}
          />
        )}
      </div>
    </div>
  );
}
