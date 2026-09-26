'use client';

import { useState, useEffect } from 'react';
import MapContainer from '@/components/map/MapContainer';
import Sidebar from '@/components/sidebar/Sidebar';
import CompanyDetailDrawer from '@/components/drawers/CompanyDetailDrawer';
import ScrapeModal from '@/components/drawers/ScrapeModal';
import BackgroundScrapeWidget from '@/components/scrapers/BackgroundScrapeWidget';
import { useCompanies } from '@/hooks/useCompanies';
import { useClusters } from '@/hooks/useClusters';
import { useScrapeJobs } from '@/hooks/useScrapeJobs';
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
  const [activeJobIds, setActiveJobIds] = useState<string[]>([]);
  const [mobileTab, setMobileTab] = useState<'map' | 'list'>('map');
  const [geoNotice, setGeoNotice] = useState<{ type: 'error' | 'success'; message: string } | null>(null);
  const [isUserLocationActive, setIsUserLocationActive] = useState(false);

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
      setIsUserLocationActive(true);
      setGeoNotice({ type: 'success', message: 'Centered map on your current location' });
      setTimeout(() => setGeoNotice(null), 4000);
    } catch (err: any) {
      setIsUserLocationActive(false);
      setGeoNotice({
        type: 'error',
        message: err?.message || 'Location access denied. Please allow location permissions in your browser.',
      });
      setTimeout(() => setGeoNotice(null), 6000);
    }
  }

  // Initialize guest session
  useAuth();

  // Track active background scraping jobs
  const { data: jobsData } = useScrapeJobs();
  const allJobs = jobsData?.jobs || [];
  const runningJobs = allJobs.filter((j) => j.status === 'running' || j.status === 'pending');
  const totalRunningSightings = runningJobs.reduce((acc, j) => acc + (j.sightings || 0), 0);

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
  const isJobRunning = runningJobs.length > 0;

  // Automatically sync selected company profile with latest verified search data
  useEffect(() => {
    if (selectedCompany && companies.length > 0) {
      const fresh = companies.find((c) => c.id === selectedCompany.id);
      if (
        fresh &&
        (fresh.confidence !== selectedCompany.confidence ||
          fresh.verified !== selectedCompany.verified ||
          fresh.address !== selectedCompany.address ||
          fresh.distance_meters !== selectedCompany.distance_meters)
      ) {
        setSelectedCompany(fresh);
      }
    }
  }, [companies, selectedCompany]);

  // Derive pre-selected tech hub region and coordinates for the scrape modal
  const { selectedRegion, selectedModalCenter, selectedDefaultMode } = (() => {
    // 1. If a company is currently selected, prioritize its location & city
    if (selectedCompany) {
      const companyCoords = { lat: selectedCompany.lat, lng: selectedCompany.lng };
      if (selectedCompany.city) {
        const matched = CITY_PRESETS.find(
          (c) =>
            selectedCompany.city?.toLowerCase().includes(c.name.toLowerCase()) ||
            c.name.toLowerCase().includes(selectedCompany.city?.toLowerCase() || '')
        );
        if (matched) {
          return {
            selectedRegion: matched.name,
            selectedModalCenter: companyCoords,
            selectedDefaultMode: 'preset' as const,
          };
        }
      }
      let closest = CITY_PRESETS[0];
      let minD = Number.MAX_VALUE;
      for (const p of CITY_PRESETS) {
        const d = Math.hypot(p.lat - selectedCompany.lat, p.lng - selectedCompany.lng);
        if (d < minD) {
          minD = d;
          closest = p;
        }
      }
      return {
        selectedRegion: closest.name,
        selectedModalCenter: companyCoords,
        selectedDefaultMode: 'preset' as const,
      };
    }

    // 2. If user explicitly used browser geolocation
    if (isUserLocationActive) {
      let closest = CITY_PRESETS[0];
      let minD = Number.MAX_VALUE;
      for (const p of CITY_PRESETS) {
        const d = Math.hypot(p.lat - center.lat, p.lng - center.lng);
        if (d < minD) {
          minD = d;
          closest = p;
        }
      }
      return {
        selectedRegion: closest.name,
        selectedModalCenter: center,
        selectedDefaultMode: 'coordinates' as const,
      };
    }

    // 3. Check if current center matches an active city preset
    const activePreset = CITY_PRESETS.find(
      (c) => Math.abs(c.lat - center.lat) < 0.05 && Math.abs(c.lng - center.lng) < 0.05
    );
    if (activePreset) {
      return {
        selectedRegion: activePreset.name,
        selectedModalCenter: center,
        selectedDefaultMode: 'preset' as const,
      };
    }

    // 4. Otherwise, find closest preset to map center
    let closest = CITY_PRESETS[0];
    let minD = Number.MAX_VALUE;
    for (const p of CITY_PRESETS) {
      const d = Math.hypot(p.lat - center.lat, p.lng - center.lng);
      if (d < minD) {
        minD = d;
        closest = p;
      }
    }
    return {
      selectedRegion: closest.name,
      selectedModalCenter: center,
      selectedDefaultMode: 'preset' as const,
    };
  })();

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
          {/* Modern Geometric Logo & Typography */}
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-xl bg-slate-800/90 border border-amber-500/30 flex items-center justify-center shadow-lg shadow-amber-500/10 shrink-0">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="w-4 h-4 text-amber-400"
              >
                <polygon points="12 2 20.66 7 20.66 17 12 22 3.34 17 3.34 7 12 2" fill="rgba(245, 158, 11, 0.15)" stroke="currentColor" />
                <circle cx="12" cy="12" r="2.5" fill="currentColor" />
              </svg>
            </div>
            <span className="font-bold text-base tracking-tight text-slate-100 flex items-center">
              Near<span className="text-amber-400">Hive</span>
            </span>
          </div>

          <div className="h-4 w-px bg-slate-800 hidden sm:block mx-0.5 md:mx-1" />

          {/* Current Location Button - only active when geolocation is active */}
          <button
            onClick={handleLocateMe}
            disabled={geoLoading}
            className={`flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-lg transition-colors cursor-pointer border font-medium disabled:opacity-50 ${
              isUserLocationActive
                ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 font-semibold'
                : 'bg-slate-800/80 hover:bg-slate-700 text-slate-300 hover:text-white border-slate-700/40'
            }`}
            title={geoError || 'Locate around current browser location'}
            aria-label="Locate Me around current browser location"
          >
            {geoLoading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <LocateFixed className={`w-3.5 h-3.5 ${isUserLocationActive ? 'text-amber-400' : 'text-slate-400'}`} />
            )}
            <span className="hidden sm:inline">Locate Me</span>
          </button>

          {/* City Presets - Dropdown on small screens, Pills on large */}
          <div className="flex lg:hidden items-center">
            <select
              aria-label="Select target tech city"
              value={!isUserLocationActive ? CITY_PRESETS.find((c) => c.lat === center.lat && c.lng === center.lng)?.name || '' : ''}
              onChange={(e) => {
                const found = CITY_PRESETS.find((c) => c.name === e.target.value);
                if (found) {
                  setIsUserLocationActive(false);
                  setCenter({ lat: found.lat, lng: found.lng });
                }
              }}
              className="bg-slate-800/90 text-slate-300 border border-slate-700/60 rounded-lg px-2 py-1 text-xs focus:outline-none focus:border-amber-500 cursor-pointer"
            >
              <option value="" disabled>{isUserLocationActive ? 'Current Location' : 'City Hubs'}</option>
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
                onClick={() => {
                  setIsUserLocationActive(false);
                  setCenter({ lat: c.lat, lng: c.lng });
                }}
                className={`px-2 py-1 text-xs rounded-lg transition-colors cursor-pointer border ${
                  !isUserLocationActive && center.lat === c.lat && center.lng === c.lng
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
          {runningJobs.length > 0 && (
            <button
              onClick={() => setIsScrapeOpen(true)}
              className="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-xl bg-amber-500/10 border border-amber-500/40 text-amber-400 text-xs hover:bg-amber-500/20 transition-all cursor-pointer animate-pulse"
              title="Click to view full scraper tasks"
            >
              <RefreshCw className="w-3.5 h-3.5 animate-spin" />
              <span className="font-semibold">
                {runningJobs.length === 1
                  ? `Crawling ${runningJobs[0].region}...`
                  : `${runningJobs.length} Scrapers Active`}
              </span>
              <span className="font-mono text-[10px] bg-amber-500/20 px-1.5 py-0.5 rounded text-amber-300 font-bold">
                {totalRunningSightings} sightings
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
            if (isClusterMode) setIsClusterMode(false);
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
            selectedCompany={selectedCompany}
            onCenterChange={(lat, lng) => {
              setIsUserLocationActive(false);
              setCenter({ lat, lng });
            }}
            onSelectCompany={setSelectedCompany}
            onClusterZoom={(lat, lng) => {
              setIsUserLocationActive(false);
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
          <CompanyDetailDrawer
            company={selectedCompany}
            onClose={() => setSelectedCompany(null)}
            onFocusOnMap={() => setMobileTab('map')}
          />
        )}

        {/* Scrape Modal */}
        <ScrapeModal
          isOpen={isScrapeOpen}
          onClose={() => setIsScrapeOpen(false)}
          defaultRegion={selectedRegion}
          defaultMode={selectedDefaultMode}
          currentCenter={selectedModalCenter}
          currentRadiusKm={radiusKm}
          activeJobIds={activeJobIds}
          onTriggerJob={(id) => {
            setActiveJobIds((prev) => [id, ...prev.filter((x) => x !== id)]);
          }}
        />

        {/* Floating Background Scraper Widget */}
        {!isScrapeOpen && (
          <BackgroundScrapeWidget
            activeJobIds={activeJobIds}
            onOpenDetails={() => setIsScrapeOpen(true)}
            onDismissJob={(id) => {
              setActiveJobIds((prev) => prev.filter((x) => x !== id));
            }}
            onDismissAll={() => setActiveJobIds([])}
          />
        )}
      </main>
    </div>
  );
}
