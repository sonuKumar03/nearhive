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
import { Play, Layers, RefreshCw, LocateFixed, Loader2, Map as MapIcon, List, AlertCircle, X, ChevronDown } from 'lucide-react';

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
  const [mobileTab, setMobileTab] = useState<'map' | 'list'>('map');
  const [geoNotice, setGeoNotice] = useState<{ type: 'error' | 'success'; message: string } | null>(null);

  // Debounce search input by 300ms to avoid flooding backend
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(searchQuery);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  // Browser Geolocation hook
  const { getCurrentLocation, loading: geoLoading, error: geoError } = useGeolocation();

  async function handleLocateMe() {
    setGeoNotice(null);
    try {
      const coords = await getCurrentLocation();
      setCenter(coords);
      setGeoNotice({ type: 'success', message: 'Centered map on your current location' });
      setTimeout(() => setGeoNotice(null), 4000);
    } catch (err: any) {
      setGeoNotice({
        type: 'error',
        message: err?.message || 'Location access denied. Please allow location permissions in your browser.',
      });
      setTimeout(() => setGeoNotice(null), 6000);
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
    <div className="h-screen w-screen flex flex-col bg-slate-950 overflow-hidden">
      {/* Geolocation Feedback Toast */}
      {geoNotice && (
        <div
          role="alert"
          aria-live="polite"
          className={`fixed top-16 left-1/2 -translate-x-1/2 z-50 px-4 py-2 rounded-xl border text-xs flex items-center gap-2.5 shadow-2xl backdrop-blur-md animate-in fade-in slide-in-from-top-2 ${
            geoNotice.type === 'error'
              ? 'bg-rose-950/95 border-rose-500/50 text-rose-200'
              : 'bg-emerald-950/95 border-emerald-500/50 text-emerald-200'
          }`}
        >
          {geoNotice.type === 'error' ? (
            <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
          ) : (
            <LocateFixed className="w-4 h-4 text-emerald-400 shrink-0" />
          )}
          <span className="font-medium">{geoNotice.message}</span>
          <button
            onClick={() => setGeoNotice(null)}
            aria-label="Dismiss notification"
            className="p-1 rounded-md text-slate-400 hover:text-white hover:bg-white/10 transition-colors ml-1 cursor-pointer"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Top Header */}
      <header className="h-14 border-b border-slate-800/80 bg-slate-900/90 backdrop-blur-md px-3 md:px-4 flex items-center justify-between shrink-0 z-20">
        <div className="flex items-center gap-2 md:gap-3">
          <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-amber-400 to-amber-600 flex items-center justify-center text-lg shadow-lg shadow-amber-500/20 shrink-0">
            🐝
          </div>
          <div>
            <div className="flex items-center gap-1.5">
              <span className="font-bold text-base tracking-tight bg-gradient-to-r from-amber-200 to-amber-500 bg-clip-text text-transparent">
                NearHive
              </span>
              <span className="text-[9px] font-mono px-1 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 font-semibold">
                SPATIAL
              </span>
            </div>
          </div>

          <div className="h-4 w-px bg-slate-800 hidden sm:block mx-0.5 md:mx-1" />

          {/* Current Location Button */}
          <button
            onClick={handleLocateMe}
            disabled={geoLoading}
            className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-lg bg-amber-500/15 hover:bg-amber-500/25 text-amber-300 border border-amber-500/30 transition-all cursor-pointer font-medium disabled:opacity-50"
            title={geoError || 'Locate around current browser location'}
            aria-label="Locate around current browser location"
          >
            {geoLoading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <LocateFixed className="w-3.5 h-3.5 text-amber-400" />
            )}
            <span className="hidden sm:inline">Locate Me</span>
          </button>

          {/* City Presets - Dropdown on small screens, Pills on large */}
          <div className="flex lg:hidden items-center">
            <select
              aria-label="Select target tech city"
              value={CITY_PRESETS.find((c) => c.lat === center.lat && c.lng === center.lng)?.name || ''}
              onChange={(e) => {
                const found = CITY_PRESETS.find((c) => c.name === e.target.value);
                if (found) setCenter({ lat: found.lat, lng: found.lng });
              }}
              className="bg-slate-800/90 text-slate-300 border border-slate-700/60 rounded-lg px-2 py-1 text-xs focus:outline-none focus:border-amber-500 cursor-pointer"
            >
              <option value="" disabled>City Hubs</option>
              {CITY_PRESETS.map((c) => (
                <option key={c.name} value={c.name}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>

          <div className="hidden lg:flex items-center gap-1">
            {CITY_PRESETS.map((c) => (
              <button
                key={c.name}
                onClick={() => setCenter({ lat: c.lat, lng: c.lng })}
                className={`px-2 py-1 text-xs rounded-lg transition-colors cursor-pointer border ${
                  center.lat === c.lat && center.lng === c.lng
                    ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 font-semibold'
                    : 'bg-slate-800/80 hover:bg-slate-700 text-slate-300 border-slate-700/40'
                }`}
              >
                {c.name}
              </button>
            ))}
          </div>
        </div>

        {/* Header Right Controls */}
        <div className="flex items-center gap-2 md:gap-3">
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

          {/* Radius Slider with accessible label */}
          <div className="flex items-center gap-1.5 md:gap-2 bg-slate-950/80 px-2 py-1 md:px-2.5 rounded-xl border border-slate-800">
            <label htmlFor="header-radius-slider" className="text-xs text-slate-400 font-medium hidden sm:inline">
              Radius:
            </label>
            <input
              id="header-radius-slider"
              aria-label="Search radius in kilometers"
              type="range"
              min={1}
              max={30}
              value={radiusKm}
              onChange={(e) => setRadiusKm(Number(e.target.value))}
              className="w-16 sm:w-20 md:w-28 accent-amber-500 cursor-pointer h-1.5 bg-slate-800 rounded-lg"
            />
            <span className="text-xs font-mono font-bold text-amber-400 min-w-[32px] md:min-w-[36px]">{radiusKm} km</span>
          </div>

          <button
            onClick={() => setIsScrapeOpen(true)}
            className="flex items-center gap-1.5 px-2.5 py-1.5 md:px-3 text-xs font-semibold rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-md shadow-amber-500/20 transition-all cursor-pointer shrink-0"
          >
            <Play className="w-3.5 h-3.5 fill-current" />
            <span className="hidden sm:inline">Scrape Tech Hub</span>
            <span className="sm:hidden">Scrape</span>
          </button>
        </div>
      </header>

      {/* Main Landmark: Canvas with Map + Sidebar */}
      <main className="flex-1 flex relative overflow-hidden">
        {/* Sidebar - responsive visibility */}
        <Sidebar
          className={mobileTab === 'list' ? 'flex' : 'hidden md:flex'}
          companies={companies}
          isLoading={loadingCompanies}
          totalCount={totalCount}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          onSelectCompany={(c) => {
            setSelectedCompany(c);
          }}
        />

        {/* Map Canvas - responsive visibility */}
        <div className={`flex-1 h-full relative ${mobileTab === 'map' ? 'block' : 'hidden md:block'}`}>
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
              <span>{isClusterMode ? 'Pins View' : 'Cluster View'}</span>
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

        {/* Mobile Tab Switcher (Floating Bottom Segmented Control) */}
        <div className="md:hidden fixed bottom-5 left-1/2 -translate-x-1/2 z-30 flex items-center bg-slate-900/95 backdrop-blur-xl border border-slate-700/80 rounded-full p-1 shadow-2xl">
          <button
            type="button"
            onClick={() => setMobileTab('map')}
            className={`flex items-center gap-1.5 px-4 py-2 rounded-full text-xs font-semibold transition-all cursor-pointer ${
              mobileTab === 'map'
                ? 'bg-amber-500 text-slate-950 shadow-md shadow-amber-500/25'
                : 'text-slate-300 hover:text-white'
            }`}
          >
            <MapIcon className="w-3.5 h-3.5" />
            <span>Map</span>
          </button>
          <button
            type="button"
            onClick={() => setMobileTab('list')}
            className={`flex items-center gap-1.5 px-4 py-2 rounded-full text-xs font-semibold transition-all cursor-pointer ${
              mobileTab === 'list'
                ? 'bg-amber-500 text-slate-950 shadow-md shadow-amber-500/25'
                : 'text-slate-300 hover:text-white'
            }`}
          >
            <List className="w-3.5 h-3.5" />
            <span>List ({totalCount})</span>
          </button>
        </div>

        {/* Company Detail Drawer */}
        {selectedCompany && (
          <CompanyDetailDrawer company={selectedCompany} onClose={() => setSelectedCompany(null)} />
        )}

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
      </main>
    </div>
  );
}
