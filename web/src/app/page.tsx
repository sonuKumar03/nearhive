'use client';

import { useState, useEffect } from 'react';
import MapContainer from '@/components/map/MapContainer';
import Sidebar, { SidebarMode } from '@/components/sidebar/Sidebar';
import CompanyDetailDrawer from '@/components/drawers/CompanyDetailDrawer';
import ScrapeModal from '@/components/drawers/ScrapeModal';
import BackgroundScrapeWidget from '@/components/scrapers/BackgroundScrapeWidget';
import { useCompanies } from '@/hooks/useCompanies';
import { useNearbyJobs } from '@/hooks/useNearbyJobs';
import { useClusters } from '@/hooks/useClusters';
import { useDiscoveryJobs } from '@/hooks/useDiscoveryJobs';
import { useAuth } from '@/hooks/useAuth';
import { useGeolocation } from '@/hooks/useGeolocation';
import { CompanySearchResult } from '@/types';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { Play, Layers, RefreshCw, LocateFixed, Loader2, Map as MapIcon, List, AlertCircle, X, ChevronDown, Sparkles } from 'lucide-react';

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
  const [sidebarMode, setSidebarMode] = useState<SidebarMode>('companies');
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

  // Track Python discovery runs.
  const { data: discoveryData } = useDiscoveryJobs();
  const allDiscoveryJobs = discoveryData?.jobs || [];
  const runningDiscovery = allDiscoveryJobs.filter(
    (j) => j.status === 'in_progress' || j.status === 'queued'
  );
  const isAnyJobRunning = runningDiscovery.length > 0;

  const { data: searchData, isLoading: loadingCompanies } = useCompanies({
    lat: center.lat,
    lng: center.lng,
    radius_km: radiusKm,
    q: debouncedQuery,
  });

  const { data: nearbyJobsData, isLoading: loadingJobs } = useNearbyJobs(
    {
      lat: center.lat,
      lng: center.lng,
      radius_km: radiusKm,
      q: debouncedQuery,
    },
    true
  );

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
    <div className="h-screen w-screen flex flex-col bg-[#09090b] overflow-hidden">
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
            className="p-1 rounded-md text-zinc-400 hover:text-white hover:bg-white/10 transition-colors ml-1 cursor-pointer"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Top Header */}
      <header className="h-14 border-b border-white/[0.08] bg-[#121216]/90 backdrop-blur-md px-3 md:px-4 flex items-center justify-between shrink-0 z-20">
        <div className="flex items-center gap-2 md:gap-3">
          {/* Modern Geometric Logo & Typography */}
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center shadow-lg shadow-indigo-500/10 shrink-0">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="w-4 h-4 text-indigo-400"
              >
                <polygon points="12 2 20.66 7 20.66 17 12 22 3.34 17 3.34 7 12 2" fill="rgba(99, 102, 241, 0.2)" stroke="currentColor" />
                <circle cx="12" cy="12" r="2.5" fill="currentColor" />
              </svg>
            </div>
            <span className="font-bold text-base tracking-tight text-white flex items-center">
              Near<span className="text-transparent bg-clip-text bg-gradient-to-r from-indigo-400 to-violet-400">Hive</span>
            </span>
          </div>

          <div className="h-4 w-px bg-white/[0.08] hidden sm:block mx-0.5 md:mx-1" />

          {/* Current Location Button - only active when geolocation is active */}
          <Button
            size="xs"
            variant={isUserLocationActive ? 'primary' : 'secondary'}
            onClick={handleLocateMe}
            disabled={geoLoading}
            isLoading={geoLoading}
            leftIcon={!geoLoading ? <LocateFixed className="w-3.5 h-3.5" /> : undefined}
            title={geoError || 'Locate around current browser location'}
            aria-label="Locate Me around current browser location"
          >
            <span className="hidden sm:inline">Locate Me</span>
          </Button>

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
            {CITY_PRESETS.map((c) => {
              const isSelected = !isUserLocationActive && center.lat === c.lat && center.lng === c.lng;
              return (
                <Button
                  key={c.name}
                  size="xs"
                  variant={isSelected ? 'primary' : 'secondary'}
                  onClick={() => {
                    setIsUserLocationActive(false);
                    setCenter({ lat: c.lat, lng: c.lng });
                  }}
                >
                  {c.name}
                </Button>
              );
            })}
          </div>
        </div>

        {/* Header Right Controls */}
        <div className="flex items-center gap-2 md:gap-3">
          {/* Active Background Scraper / Discovery Header Pill */}
          {isAnyJobRunning && (
            <button
              type="button"
              onClick={() => setIsScrapeOpen(true)}
              className="hidden md:flex cursor-pointer transition-transform hover:scale-105"
              title="View discovery runs"
            >
              <Badge variant="indigo" pulsing size="md" icon={<RefreshCw className="w-3 h-3 animate-spin" />}>
                <span className="font-semibold">{runningDiscovery.length} Discovery Active</span>
              </Badge>
            </button>
          )}

          {/* Radius Slider with accessible label */}
          <div className="flex items-center gap-1.5 md:gap-2 bg-[#09090b] px-2 py-1 md:px-2.5 rounded-xl border border-white/[0.08]">
            <label htmlFor="header-radius-slider" className="text-xs text-zinc-400 font-medium hidden sm:inline">
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
              className="w-16 sm:w-20 md:w-28 accent-indigo-500 cursor-pointer h-1.5 bg-zinc-800 rounded-lg"
            />
            <span className="text-xs font-mono font-bold text-indigo-400 min-w-[32px] md:min-w-[36px]">{radiusKm} km</span>
          </div>

          <Button
            size="sm"
            variant="primary"
            onClick={() => setIsScrapeOpen(true)}
            leftIcon={<Sparkles className="w-3.5 h-3.5 fill-current" />}
            className="shrink-0"
          >
            <span className="hidden sm:inline">Discover Tech Hub</span>
            <span className="sm:hidden">Discover</span>
          </Button>
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
          jobs={nearbyJobsData?.jobs || []}
          isLoadingJobs={loadingJobs}
          totalJobsCount={nearbyJobsData?.meta?.total}
          activeMode={sidebarMode}
          onModeChange={setSidebarMode}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          onSelectCompany={(c) => {
            setSelectedCompany(c);
            if (isClusterMode) setIsClusterMode(false);
          }}
          selectedCompany={selectedCompany}
        />

        {/* Map Canvas - responsive visibility */}
        <div className={`flex-1 h-full relative ${mobileTab === 'map' ? 'block' : 'hidden md:block'}`}>
          {/* Floating Cluster Toggle */}
          <div className="absolute top-4 right-4 z-20">
            <Button
              size="sm"
              variant={isClusterMode ? 'primary' : 'secondary'}
              leftIcon={<Layers className="w-3.5 h-3.5" />}
              onClick={() => setIsClusterMode(!isClusterMode)}
              className="shadow-xl backdrop-blur-md"
            >
              {isClusterMode ? 'Pins View' : 'Cluster View'}
            </Button>
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
                ? 'bg-amber-500 text-black shadow-md shadow-amber-500/25'
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
                ? 'bg-amber-500 text-black shadow-md shadow-amber-500/25'
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
          onTriggerJob={(id) => {
            setActiveJobIds((prev) => [id, ...prev.filter((x) => x !== id)]);
          }}
          onFocusLocation={(lat, lng, rKm) => {
            setCenter({ lat, lng });
            if (rKm) setRadiusKm(rKm);
            setMobileTab('map');
          }}
        />

        {/* Floating Background Scraper Widget */}
        {!isScrapeOpen && (
          <BackgroundScrapeWidget
            activeJobIds={activeJobIds}
            onOpenDetails={() => setIsScrapeOpen(true)}
            onDismissAll={() => setActiveJobIds([])}
          />
        )}
      </main>
    </div>
  );
}
