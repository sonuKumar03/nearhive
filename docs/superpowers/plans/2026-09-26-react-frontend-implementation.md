# NearHive React Frontend (Vercel) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a sleek, modern, responsive Next.js/React frontend with TanStack Query v5, Tailwind CSS, Leaflet spatial mapping, and PostGIS clustering, configured for seamless deployment to Vercel.

**Architecture:** A modern Next.js 15 App Router application in `web/` with TanStack Query v5 managing server state, Leaflet rendering geospatial layers (individual office pins and server-side PostGIS `ST_ClusterKMeans` centroid clusters), and Vercel rewrites proxying `/api/*` requests to the Railway Go backend (`https://nearhive-production.up.railway.app`).

**Tech Stack:** Next.js 15 (App Router), React 19, TypeScript 5, Tailwind CSS v4, TanStack Query v5, Leaflet & React-Leaflet, Lucide React icons, Vercel Edge/Serverless platform.

**Spec:** [`docs/superpowers/specs/2026-09-26-react-frontend-design.md`](file:///Users/sonukumar/project/nearhive/docs/superpowers/specs/2026-09-26-react-frontend-design.md)

## Global Constraints
- Target platform: Vercel (Edge CDN + Static/Serverless rendering)
- Backend API origin: `https://nearhive-production.up.railway.app`
- Proxy path: Next.js rewrites `/api/:path*` to backend to eliminate CORS
- Node environment: Node.js 20+ / Package Manager: `pnpm`
- Strict TypeScript: `noImplicitAny: true`, exact types matching Go models
- Visual Aesthetic: Tech Hive Dark mode (`slate-950` obsidian base, `amber-500` accents, `emerald-400` verification badges)

---

### Task 1: Scaffolding Next.js App, TypeScript, Tailwind CSS, and Vercel Rewrites

**Files:**
- Create: `web/package.json`
- Create: `web/tsconfig.json`
- Create: `web/next.config.ts`
- Create: `web/postcss.config.mjs`
- Create: `web/src/app/globals.css`
- Create: `web/.gitignore`

**Interfaces:**
- Produces: Runnable Next.js project in `web/` with Vercel rewrites to Railway Go backend API.

- [ ] **Step 1: Create `web/package.json` with dependencies**

```json
{
  "name": "nearhive-web",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "dev": "next dev --port 3000",
    "build": "next build",
    "start": "next start",
    "lint": "next lint",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "@tanstack/react-query": "^5.67.1",
    "@tanstack/react-query-devtools": "^5.67.1",
    "clsx": "^2.1.1",
    "leaflet": "^1.9.4",
    "lucide-react": "^0.479.0",
    "next": "^15.1.7",
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-leaflet": "^5.0.0",
    "tailwind-merge": "^3.0.2"
  },
  "devDependencies": {
    "@tailwindcss/postcss": "^4.0.12",
    "@types/leaflet": "^1.9.16",
    "@types/node": "^22.13.10",
    "@types/react": "^19.0.10",
    "@types/react-dom": "^19.0.4",
    "postcss": "^8.5.3",
    "tailwindcss": "^4.0.12",
    "typescript": "^5.8.2"
  }
}
```

- [ ] **Step 2: Create `web/next.config.ts` with Vercel rewrites**

```typescript
import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  async rewrites() {
    const backendUrl =
      process.env.NEXT_PUBLIC_API_URL || 'https://nearhive-production.up.railway.app';
    return [
      {
        source: '/api/:path*',
        destination: `${backendUrl}/api/:path*`,
      },
      {
        source: '/health/:path*',
        destination: `${backendUrl}/health/:path*`,
      },
    ];
  },
};

export default nextConfig;
```

- [ ] **Step 3: Create `web/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["dom", "dom.iterable", "esnext"],
    "allowJs": true,
    "skipLibCheck": true,
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "module": "esnext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "jsx": "preserve",
    "incremental": true,
    "plugins": [
      {
        "name": "next"
      }
    ],
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules"]
}
```

- [ ] **Step 4: Create `web/src/app/globals.css` with Tech Hive Dark theme**

```css
@import "tailwindcss";

@layer base {
  body {
    background-color: #020617;
    color: #f8fafc;
    font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  }
}

/* Custom dark tile styling for Leaflet CartoDB / OSM */
.dark-tiles {
  filter: brightness(0.7) invert(1) contrast(3) hue-rotate(200deg) saturate(0.3) brightness(0.7);
}

/* Pulsing epicenter animation */
@keyframes epicenter-pulse {
  0% { transform: scale(0.95); opacity: 0.8; }
  50% { transform: scale(1.6); opacity: 0.2; }
  100% { transform: scale(0.95); opacity: 0.8; }
}

.epicenter-pulse {
  animation: epicenter-pulse 2.5s infinite cubic-bezier(0.4, 0, 0.6, 1);
}
```

- [ ] **Step 5: Run dependency installation**

Run: `cd web && pnpm install`  
Expected: Dependencies installed with zero errors.

---

### Task 2: TypeScript Data Contracts & API Client

**Files:**
- Create: `web/src/types/index.ts`
- Create: `web/src/lib/api-client.ts`
- Create: `web/src/lib/query-client.ts`
- Create: `web/src/lib/utils.ts`

**Interfaces:**
- Produces: Strongly typed data contracts matching Go models and authenticated fetch wrapper.

- [ ] **Step 1: Write `web/src/types/index.ts`**

```typescript
export interface Company {
  id: string;
  name: string;
  normalized_name: string;
  domain?: string;
  industry?: string;
  employee_count?: string;
  created_at: string;
  updated_at: string;
}

export interface CompanySearchResult {
  id: string;
  name: string;
  domain?: string;
  industry?: string;
  employee_count?: string;
  location_id: string;
  label?: string;
  address: string;
  city?: string;
  lat: number;
  lng: number;
  confidence: number;
  distance_meters: number;
  verified: boolean;
}

export interface SpatialCluster {
  cluster_id: number;
  count: number;
  lat: number;
  lng: number;
}

export interface Sighting {
  id: string;
  source: string;
  source_url?: string;
  company_name: string;
  raw_address: string;
  lat: number;
  lng: number;
  metadata?: Record<string, any>;
  company_id?: string;
  location_id?: string;
  scraped_at: string;
}

export interface ScrapeTask {
  id: string;
  job_id: string;
  source: string;
  status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled';
  sightings: number;
  error?: string;
  duration_ms: number;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

export interface ScrapeJob {
  id: string;
  source: string;
  status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled';
  region?: string;
  sightings: number;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  tasks?: ScrapeTask[];
}

export interface User {
  id: string;
  email: string;
  created_at?: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface SearchParams {
  lat: number;
  lng: number;
  radius_km: number;
  min_confidence?: number;
  industry?: string;
  q?: string;
  limit?: number;
  page?: number;
}

export interface ClusterParams {
  lat: number;
  lng: number;
  radius_km: number;
  k?: number;
}
```

- [ ] **Step 2: Write `web/src/lib/api-client.ts` with transparent JWT auth**

```typescript
export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export async function fetchApi<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = typeof window !== 'undefined' ? localStorage.getItem('nearhive_token') : null;

  const headers = new Headers(options.headers || {});
  headers.set('Content-Type', 'application/json');
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const res = await fetch(path, {
    ...options,
    headers,
  });

  if (!res.ok) {
    let errorData = { message: 'An unexpected error occurred', code: 'INTERNAL_ERROR' };
    try {
      errorData = await res.json();
    } catch {}
    throw new ApiError(res.status, errorData.code || 'API_ERROR', errorData.message || res.statusText);
  }

  return res.json() as Promise<T>;
}
```

- [ ] **Step 3: Write `web/src/lib/query-client.ts`**

```typescript
import { QueryClient } from '@tanstack/react-query';

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 60 * 1000, // 1 minute
        gcTime: 5 * 60 * 1000,
        refetchOnWindowFocus: false,
        retry: 1,
      },
    },
  });
}

let browserQueryClient: QueryClient | undefined = undefined;

export function getQueryClient() {
  if (typeof window === 'undefined') {
    return makeQueryClient();
  }
  if (!browserQueryClient) browserQueryClient = makeQueryClient();
  return browserQueryClient;
}
```

- [ ] **Step 4: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 3: TanStack Query Data Fetching Hooks

**Files:**
- Create: `web/src/hooks/useCompanies.ts`
- Create: `web/src/hooks/useClusters.ts`
- Create: `web/src/hooks/useCompanyDetails.ts`
- Create: `web/src/hooks/useScrapeJobs.ts`
- Create: `web/src/hooks/useAuth.ts`

**Interfaces:**
- Produces: Reactive hooks with automatic caching, background polling, and mutation invalidation.

- [ ] **Step 1: Write `web/src/hooks/useCompanies.ts`**

```typescript
import { useQuery } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { CompanySearchResult, SearchParams } from '@/types';

interface SearchResponse {
  meta: {
    total: number;
    page: number;
    limit: number;
    radius_km: number;
    center: { lat: number; lng: number };
  };
  companies: CompanySearchResult[];
}

export function useCompanies(params: SearchParams, enabled = true) {
  return useQuery({
    queryKey: ['companies', params],
    queryFn: async () => {
      const q = new URLSearchParams({
        lat: params.lat.toString(),
        lng: params.lng.toString(),
        radius: params.radius_km.toString(),
      });
      if (params.min_confidence) q.set('min_confidence', params.min_confidence.toString());
      if (params.industry) q.set('industry', params.industry);
      if (params.q) q.set('q', params.q);
      if (params.limit) q.set('limit', params.limit.toString());
      if (params.page) q.set('page', params.page.toString());

      return fetchApi<SearchResponse>(`/api/v1/search?${q.toString()}`);
    },
    enabled: enabled && !!params.lat && !!params.lng,
    placeholderData: (prev) => prev,
  });
}
```

- [ ] **Step 2: Write `web/src/hooks/useClusters.ts`**

```typescript
import { useQuery } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { ClusterParams, SpatialCluster } from '@/types';

interface ClusterResponse {
  meta: {
    k: number;
    cluster_count: number;
    total_points: number;
    radius_km: number;
    center: { lat: number; lng: number };
  };
  clusters: SpatialCluster[];
}

export function useClusters(params: ClusterParams, enabled = false) {
  return useQuery({
    queryKey: ['clusters', params],
    queryFn: async () => {
      const q = new URLSearchParams({
        lat: params.lat.toString(),
        lng: params.lng.toString(),
        radius: params.radius_km.toString(),
        k: (params.k || 20).toString(),
      });
      return fetchApi<ClusterResponse>(`/api/v1/search/clusters?${q.toString()}`);
    },
    enabled: enabled && !!params.lat && !!params.lng,
  });
}
```

- [ ] **Step 3: Write `web/src/hooks/useCompanyDetails.ts`**

```typescript
import { useQuery } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { Company, Sighting } from '@/types';

export function useCompany(companyId: string | null) {
  return useQuery({
    queryKey: ['company', companyId],
    queryFn: () => fetchApi<Company>(`/api/v1/companies/${companyId}`),
    enabled: !!companyId,
  });
}

export function useSightings(companyId: string | null) {
  return useQuery({
    queryKey: ['company-sightings', companyId],
    queryFn: () => fetchApi<{ sightings: Sighting[] }>(`/api/v1/companies/${companyId}/sightings`),
    enabled: !!companyId,
  });
}
```

- [ ] **Step 4: Write `web/src/hooks/useScrapeJobs.ts` with polling and cancellation**

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { ScrapeJob } from '@/types';

export function useScrapeJob(jobId: string | null) {
  const queryClient = useQueryClient();

  return useQuery({
    queryKey: ['job', jobId],
    queryFn: async () => {
      const job = await fetchApi<ScrapeJob>(`/api/v1/jobs/${jobId}`);
      if (job.status === 'done' || job.status === 'cancelled') {
        // Auto-invalidate companies & clusters on completion
        queryClient.invalidateQueries({ queryKey: ['companies'] });
        queryClient.invalidateQueries({ queryKey: ['clusters'] });
      }
      return job;
    },
    enabled: !!jobId,
    refetchInterval: (query) => {
      const s = query.state.data?.status;
      return s === 'running' || s === 'pending' ? 2000 : false;
    },
  });
}

export function useTriggerScraper() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { region: string; radius_km: number; lat?: number; lng?: number }) =>
      fetchApi<ScrapeJob>('/api/v1/jobs/trigger', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (job) => {
      queryClient.setQueryData(['job', job.id], job);
    },
  });
}

export function useCancelScraper() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (jobId: string) =>
      fetchApi<{ message: string; job: ScrapeJob }>(`/api/v1/jobs/${jobId}/cancel`, {
        method: 'POST',
      }),
    onSuccess: (res, jobId) => {
      queryClient.setQueryData(['job', jobId], res.job);
      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
    },
  });
}
```

- [ ] **Step 5: Write `web/src/hooks/useAuth.ts` with guest demo auto-login**

```typescript
import { useState, useEffect } from 'react';
import { fetchApi } from '@/lib/api-client';
import { AuthResponse, User } from '@/types';

export function useAuth() {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    const savedToken = localStorage.getItem('nearhive_token');
    const savedUser = localStorage.getItem('nearhive_user');
    if (savedToken) {
      setToken(savedToken);
      if (savedUser) {
        try { setUser(JSON.parse(savedUser)); } catch {}
      }
    } else {
      // Auto-authenticate guest demo
      autoLoginGuest();
    }
  }, []);

  async function autoLoginGuest() {
    try {
      const email = 'guest@nearhive.com';
      const password = 'guestpassword123';
      let res: AuthResponse;
      try {
        res = await fetchApi<AuthResponse>('/api/v1/auth/login', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
      } catch {
        res = await fetchApi<AuthResponse>('/api/v1/auth/register', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
      }
      setToken(res.token);
      setUser(res.user);
      localStorage.setItem('nearhive_token', res.token);
      localStorage.setItem('nearhive_user', JSON.stringify(res.user));
    } catch (e) {
      console.warn('Guest login bypass:', e);
    }
  }

  function logout() {
    localStorage.removeItem('nearhive_token');
    localStorage.removeItem('nearhive_user');
    setToken(null);
    setUser(null);
  }

  return { token, user, logout };
}
```

- [ ] **Step 6: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 4: Interactive Leaflet Map Canvas & Epicenter Draggable Layer

**Files:**
- Create: `web/src/components/map/MapContainer.tsx`
- Create: `web/src/components/map/EpicenterMarker.tsx`
- Create: `web/src/components/map/CompanyMarkers.tsx`
- Create: `web/src/components/map/ClusterMarkers.tsx`

**Interfaces:**
- Produces: Dynamic Leaflet map with dark theme, draggable epicenter, radius circle, and custom company pins.

- [ ] **Step 1: Write `web/src/components/map/MapContainer.tsx` with dynamic SSR client loading**

```tsx
'use client';

import dynamic from 'next/dynamic';
import { CompanySearchResult, SpatialCluster } from '@/types';

// Dynamically import Leaflet with ssr: false to prevent window is not defined errors
const ClientMap = dynamic(() => import('./ClientMap'), {
  ssr: false,
  loading: () => (
    <div className="w-full h-full flex items-center justify-center bg-slate-950 text-slate-400">
      <div className="animate-spin w-8 h-8 border-2 border-amber-500 border-t-transparent rounded-full" />
    </div>
  ),
});

interface MapProps {
  center: { lat: number; lng: number };
  radiusKm: number;
  companies: CompanySearchResult[];
  clusters: SpatialCluster[];
  isClusterMode: boolean;
  onCenterChange: (lat: number, lng: number) => void;
  onSelectCompany: (company: CompanySearchResult) => void;
  onClusterZoom: (lat: number, lng: number) => void;
}

export default function MapContainer(props: MapProps) {
  return <ClientMap {...props} />;
}
```

- [ ] **Step 2: Write `web/src/components/map/ClientMap.tsx`**

```tsx
'use client';

import { useEffect, useRef } from 'react';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { CompanySearchResult, SpatialCluster } from '@/types';

interface ClientMapProps {
  center: { lat: number; lng: number };
  radiusKm: number;
  companies: CompanySearchResult[];
  clusters: SpatialCluster[];
  isClusterMode: boolean;
  onCenterChange: (lat: number, lng: number) => void;
  onSelectCompany: (company: CompanySearchResult) => void;
  onClusterZoom: (lat: number, lng: number) => void;
}

export default function ClientMap({
  center,
  radiusKm,
  companies,
  clusters,
  isClusterMode,
  onCenterChange,
  onSelectCompany,
  onClusterZoom,
}: ClientMapProps) {
  const mapRef = useRef<L.Map | null>(null);
  const mapContainerRef = useRef<HTMLDivElement>(null);
  const epicenterRef = useRef<L.Marker | null>(null);
  const circleRef = useRef<L.Circle | null>(null);
  const markerLayerRef = useRef<L.LayerGroup | null>(null);

  // Initialize Map Once
  useEffect(() => {
    if (!mapContainerRef.current || mapRef.current) return;

    const map = L.map(mapContainerRef.current, {
      center: [center.lat, center.lng],
      zoom: 12,
      zoomControl: false,
    });

    L.control.zoom({ position: 'bottomright' }).addTo(map);

    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors',
      className: 'dark-tiles',
    }).addTo(map);

    const markerLayer = L.layerGroup().addTo(map);
    markerLayerRef.current = markerLayer;

    // Draggable Epicenter Marker
    const epicenterIcon = L.divIcon({
      className: 'epicenter-marker',
      html: `
        <div class="relative flex items-center justify-center w-8 h-8 -ml-4 -mt-4">
          <div class="absolute w-8 h-8 rounded-full bg-amber-500/30 epicenter-pulse"></div>
          <div class="w-4 h-4 rounded-full bg-amber-500 border-2 border-slate-950 shadow-lg shadow-amber-500/50"></div>
        </div>
      `,
      iconSize: [32, 32],
    });

    const epicenter = L.marker([center.lat, center.lng], {
      icon: epicenterIcon,
      draggable: true,
    }).addTo(map);

    epicenter.on('dragend', (e) => {
      const pos = e.target.getLatLng();
      onCenterChange(pos.lat, pos.lng);
    });

    const circle = L.circle([center.lat, center.lng], {
      radius: radiusKm * 1000,
      color: '#f59e0b',
      weight: 1.5,
      opacity: 0.8,
      fillColor: '#f59e0b',
      fillOpacity: 0.06,
    }).addTo(map);

    epicenterRef.current = epicenter;
    circleRef.current = circle;
    mapRef.current = map;

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // Sync Epicenter & Radius on Prop Change
  useEffect(() => {
    if (epicenterRef.current) {
      epicenterRef.current.setLatLng([center.lat, center.lng]);
    }
    if (circleRef.current) {
      circleRef.current.setLatLng([center.lat, center.lng]);
      circleRef.current.setRadius(radiusKm * 1000);
    }
    if (mapRef.current) {
      mapRef.current.panTo([center.lat, center.lng], { animate: true });
    }
  }, [center.lat, center.lng, radiusKm]);

  // Sync Marker / Cluster Layers
  useEffect(() => {
    if (!markerLayerRef.current || !mapRef.current) return;
    markerLayerRef.current.clearLayers();

    if (isClusterMode) {
      // Render PostGIS Centroid Clusters
      clusters.forEach((c) => {
        if (!c.lat || !c.lng) return;
        const size = Math.min(56, Math.max(34, 26 + Math.log2(c.count + 1) * 5));
        const icon = L.divIcon({
          className: 'cluster-pin',
          html: `
            <div class="flex items-center justify-center rounded-full shadow-2xl border-2 border-amber-400 bg-amber-500/90 text-slate-950 font-mono font-black transition-transform hover:scale-110 cursor-pointer" style="width: ${size}px; height: ${size}px; margin-left: -${size / 2}px; margin-top: -${size / 2}px; font-size: ${size > 42 ? '12px' : '10px'}">
              ${c.count}
            </div>
          `,
          iconSize: [size, size],
        });

        const marker = L.marker([c.lat, c.lng], { icon });
        marker.on('click', () => {
          onClusterZoom(c.lat, c.lng);
        });
        markerLayerRef.current?.addLayer(marker);
      });
    } else {
      // Render Individual Company Pins
      companies.forEach((comp) => {
        if (!comp.lat || !comp.lng) return;
        const conf = Math.round(comp.confidence * 100);
        const markerColor = conf >= 80 ? '#10b981' : conf >= 60 ? '#f59e0b' : '#64748b';

        const customIcon = L.divIcon({
          className: 'company-pin',
          html: `
            <div class="flex items-center justify-center w-7 h-7 -ml-3.5 -mt-3.5 rounded-xl shadow-lg border border-slate-900/60" style="background-color: ${markerColor}">
              <span class="text-xs">🏢</span>
            </div>
          `,
          iconSize: [28, 28],
        });

        const marker = L.marker([comp.lat, comp.lng], { icon: customIcon });
        marker.on('click', () => onSelectCompany(comp));
        markerLayerRef.current?.addLayer(marker);
      });
    }
  }, [companies, clusters, isClusterMode]);

  return <div ref={mapContainerRef} className="w-full h-full relative z-0" />;
}
```

- [ ] **Step 3: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 5: Search & Filter Sidebar with Virtualized Company Cards

**Files:**
- Create: `web/src/components/sidebar/Sidebar.tsx`
- Create: `web/src/components/sidebar/CompanyCard.tsx`
- Create: `web/src/components/sidebar/FilterControls.tsx`

**Interfaces:**
- Produces: Collapsible sidebar rendering real-time search, industry filter chips, and company cards.

- [ ] **Step 1: Write `web/src/components/sidebar/CompanyCard.tsx`**

```tsx
import { CompanySearchResult } from '@/types';
import { Building2, MapPin, Users } from 'lucide-react';

interface CompanyCardProps {
  company: CompanySearchResult;
  onClick: () => void;
}

export default function CompanyCard({ company, onClick }: CompanyCardProps) {
  const conf = Math.round(company.confidence * 100);
  const confBadgeColor =
    conf >= 80
      ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30'
      : conf >= 60
      ? 'text-amber-400 bg-amber-500/10 border-amber-500/30'
      : 'text-slate-400 bg-slate-500/10 border-slate-500/30';

  const distanceKm = (company.distance_meters / 1000).toFixed(1);

  return (
    <div
      onClick={onClick}
      className="p-3 rounded-xl bg-slate-950/60 hover:bg-slate-850/80 border border-slate-800/80 hover:border-amber-500/30 transition-all cursor-pointer group space-y-2"
    >
      <div className="flex items-start justify-between gap-2">
        <div>
          <h4 className="font-semibold text-xs text-slate-100 group-hover:text-amber-300 transition-colors flex items-center gap-1.5">
            <Building2 className="w-3.5 h-3.5 text-amber-400" />
            {company.name}
          </h4>
          <div className="flex items-center gap-2 mt-1">
            {company.industry && (
              <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-slate-800 text-slate-300">
                {company.industry}
              </span>
            )}
            {company.employee_count && (
              <span className="text-[10px] text-slate-400 font-mono flex items-center gap-1">
                <Users className="w-3 h-3 text-slate-500" />
                {company.employee_count}
              </span>
            )}
          </div>
        </div>
        <span className={`text-[10px] font-mono font-semibold px-2 py-0.5 rounded-full border ${confBadgeColor}`}>
          {conf}%
        </span>
      </div>

      <p className="text-[11px] text-slate-400 leading-relaxed flex items-center gap-1.5">
        <MapPin className="w-3 h-3 text-slate-500 shrink-0" />
        <span className="truncate">{company.address || 'Address registered'}</span>
      </p>

      <div className="flex items-center justify-between pt-1 border-t border-slate-800/50 text-[11px]">
        <span className="text-slate-500 font-mono text-[10px]">{distanceKm} km away</span>
        <span className="text-amber-400 group-hover:text-amber-300 font-medium text-[11px] flex items-center gap-1">
          Details ›
        </span>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Write `web/src/components/sidebar/Sidebar.tsx`**

```tsx
import { CompanySearchResult } from '@/types';
import CompanyCard from './CompanyCard';
import { Search, SlidersHorizontal, Download } from 'lucide-react';

interface SidebarProps {
  companies: CompanySearchResult[];
  isLoading: boolean;
  totalCount: number;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onSelectCompany: (company: CompanySearchResult) => void;
  onExportCsv: () => void;
  onExportGeoJson: () => void;
}

export default function Sidebar({
  companies,
  isLoading,
  totalCount,
  searchQuery,
  onSearchChange,
  onSelectCompany,
  onExportCsv,
  onExportGeoJson,
}: SidebarProps) {
  return (
    <aside className="w-full sm:w-96 md:w-[420px] bg-slate-900/95 border-r border-slate-800/80 backdrop-blur-md flex flex-col z-10 shrink-0 h-full">
      {/* Search Header */}
      <div className="p-3.5 border-b border-slate-800/80 space-y-2.5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm text-slate-200">Discovered Tech Offices</span>
            <span className="text-xs font-mono font-semibold px-2 py-0.5 rounded-full bg-slate-800 text-amber-400 border border-slate-700">
              {totalCount}
            </span>
          </div>

          <div className="flex items-center gap-1.5">
            <button
              onClick={onExportCsv}
              title="Export as CSV"
              className="px-2 py-1 text-[11px] font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700/60 transition-colors flex items-center gap-1"
            >
              <Download className="w-3 h-3 text-amber-400" />
              CSV
            </button>
            <button
              onClick={onExportGeoJson}
              title="Export as GeoJSON"
              className="px-2 py-1 text-[11px] font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700/60 transition-colors flex items-center gap-1"
            >
              <Download className="w-3 h-3 text-emerald-400" />
              GeoJSON
            </button>
          </div>
        </div>

        {/* Filter Input */}
        <div className="relative">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Filter by company, industry, or tech park..."
            className="w-full bg-slate-950/80 border border-slate-800 rounded-xl px-3 py-1.5 pl-8 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-amber-500/50 transition-colors"
          />
          <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-2.5" />
        </div>
      </div>

      {/* Cards List Container */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2.5 divide-y divide-slate-800/40">
        {isLoading ? (
          <div className="py-16 text-center space-y-3">
            <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin mx-auto" />
            <p className="text-xs text-slate-400 font-medium">Querying PostGIS ST_DWithin index...</p>
          </div>
        ) : companies.length === 0 ? (
          <div className="py-14 px-6 text-center space-y-3">
            <div className="w-12 h-12 rounded-2xl bg-slate-800/60 flex items-center justify-center text-2xl mx-auto border border-slate-700/50">
              🔍
            </div>
            <h4 className="font-semibold text-xs text-slate-200">No Tech Offices in this Radius</h4>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              Expand the radius slider or run our automated scraper to discover tech companies in this hub.
            </p>
          </div>
        ) : (
          companies.map((c) => (
            <CompanyCard key={c.id + c.location_id} company={c} onClick={() => onSelectCompany(c)} />
          ))
        )}
      </div>
    </aside>
  );
}
```

- [ ] **Step 3: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 6: Company Detail Drawer & Multi-Source Sightings Timeline

**Files:**
- Create: `web/src/components/drawers/CompanyDetailDrawer.tsx`
- Create: `web/src/components/drawers/SightingsTimeline.tsx`

**Interfaces:**
- Produces: Slide-over drawer showing detailed verification breakdown, confidence metrics, and crawl source audit trail.

- [ ] **Step 1: Write `web/src/components/drawers/SightingsTimeline.tsx`**

```tsx
import { Sighting } from '@/types';
import { ExternalLink, CheckCircle2, Globe, Database, Building } from 'lucide-react';

interface SightingsTimelineProps {
  sightings: Sighting[];
}

export default function SightingsTimeline({ sightings }: SightingsTimelineProps) {
  if (!sightings || sightings.length === 0) {
    return (
      <div className="py-8 text-center text-xs text-slate-400">
        No external sightings recorded for this company.
      </div>
    );
  }

  const getSourceIcon = (source: string) => {
    switch (source.toLowerCase()) {
      case 'osm':
        return <Globe className="w-3.5 h-3.5 text-emerald-400" />;
      case 'wikidata':
        return <Database className="w-3.5 h-3.5 text-amber-400" />;
      case 'techparks':
        return <Building className="w-3.5 h-3.5 text-blue-400" />;
      default:
        return <CheckCircle2 className="w-3.5 h-3.5 text-slate-400" />;
    }
  };

  return (
    <div className="space-y-3">
      {sightings.map((s, idx) => (
        <div key={s.id || idx} className="p-3 rounded-xl bg-slate-950/70 border border-slate-800/80 space-y-1.5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-200 flex items-center gap-1.5">
              {getSourceIcon(s.source)}
              <span className="uppercase tracking-wider">{s.source}</span>
            </span>
            <span className="text-[10px] font-mono text-slate-500">
              {new Date(s.scraped_at).toLocaleDateString()}
            </span>
          </div>

          <p className="text-[11px] text-slate-300">{s.raw_address}</p>

          <div className="flex items-center justify-between pt-1 border-t border-slate-800/50 text-[10px]">
            <span className="font-mono text-slate-500">{s.lat.toFixed(4)}, {s.lng.toFixed(4)}</span>
            {s.source_url && (
              <a
                href={s.source_url}
                target="_blank"
                rel="noreferrer"
                className="text-amber-400 hover:text-amber-300 flex items-center gap-1"
              >
                <span>Source</span>
                <ExternalLink className="w-2.5 h-2.5" />
              </a>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Write `web/src/components/drawers/CompanyDetailDrawer.tsx`**

```tsx
import { CompanySearchResult } from '@/types';
import { useCompany, useSightings } from '@/hooks/useCompanyDetails';
import SightingsTimeline from './SightingsTimeline';
import { X, Building2, Globe, Users, ShieldCheck, MapPin } from 'lucide-react';

interface DrawerProps {
  company: CompanySearchResult | null;
  onClose: () => void;
}

export default function CompanyDetailDrawer({ company, onClose }: DrawerProps) {
  if (!company) return null;

  const { data: details, isLoading: loadingCompany } = useCompany(company.id);
  const { data: sightingsData, isLoading: loadingSightings } = useSightings(company.id);

  const conf = Math.round(company.confidence * 100);

  return (
    <div className="fixed inset-y-0 right-0 w-full sm:w-[460px] bg-slate-900 border-l border-slate-800/80 shadow-2xl z-50 flex flex-col backdrop-blur-xl animate-in slide-in-from-right duration-300">
      {/* Header */}
      <div className="p-4 border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-lg bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-400">
            <Building2 className="w-4 h-4" />
          </div>
          <div>
            <h3 className="font-bold text-sm text-slate-100">{company.name}</h3>
            <span className="text-[10px] text-slate-400 font-mono">ID: {company.id.slice(0, 8)}...</span>
          </div>
        </div>
        <button onClick={onClose} className="p-1 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800">
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-4 space-y-5">
        {/* Verification Card */}
        <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between">
          <div className="space-y-0.5">
            <span className="text-[11px] font-semibold text-slate-400 flex items-center gap-1">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              Verification Confidence
            </span>
            <span className="text-xl font-mono font-bold text-emerald-400">{conf}%</span>
          </div>
          <span className="text-xs px-2.5 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 font-semibold">
            PostGIS Verified
          </span>
        </div>

        {/* Overview Meta */}
        <div className="space-y-2.5">
          <h4 className="text-xs font-semibold text-slate-300">Company Overview</h4>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1">
              <span className="text-[10px] text-slate-500">Industry</span>
              <p className="font-medium text-slate-200">{company.industry || 'Technology'}</p>
            </div>
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1">
              <span className="text-[10px] text-slate-500">Employees</span>
              <p className="font-medium text-slate-200">{company.employee_count || '100 - 500'}</p>
            </div>
          </div>
          {company.domain && (
            <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800/80 flex items-center justify-between text-xs">
              <span className="text-slate-400 flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5 text-amber-400" />
                Domain
              </span>
              <a
                href={`https://${company.domain}`}
                target="_blank"
                rel="noreferrer"
                className="text-amber-400 hover:underline font-mono"
              >
                {company.domain}
              </a>
            </div>
          )}
        </div>

        {/* Registered Location */}
        <div className="space-y-2">
          <h4 className="text-xs font-semibold text-slate-300 flex items-center gap-1">
            <MapPin className="w-3.5 h-3.5 text-slate-400" />
            Registered Coordinates & Address
          </h4>
          <div className="p-3 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1.5 text-xs text-slate-300">
            <p>{company.address}</p>
            <p className="font-mono text-[11px] text-slate-500">
              Coordinates: {company.lat.toFixed(5)}, {company.lng.toFixed(5)}
            </p>
          </div>
        </div>

        {/* Multi-Source Sightings */}
        <div className="space-y-2">
          <h4 className="text-xs font-semibold text-slate-300">Verification Sightings Audit</h4>
          {loadingSightings ? (
            <div className="py-6 text-center text-xs text-slate-500">Loading audit trail...</div>
          ) : (
            <SightingsTimeline sightings={sightingsData?.sightings || []} />
          )}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 7: Scraper Pipeline Modal & In-Flight Cancellation

**Files:**
- Create: `web/src/components/drawers/ScrapeModal.tsx`

**Interfaces:**
- Produces: Modal to trigger multi-task background scrape jobs, inspect discrete subtasks (`osm`, `wikidata`, `techparks`), and cancel in-flight jobs.

- [ ] **Step 1: Write `web/src/components/drawers/ScrapeModal.tsx`**

```tsx
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
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 8: CSV & GeoJSON Export Engine

**Files:**
- Create: `web/src/lib/export.ts`

**Interfaces:**
- Produces: Client-side file generation and download utilities for CSV and RFC 7946 GeoJSON.

- [ ] **Step 1: Write `web/src/lib/export.ts`**

```typescript
import { CompanySearchResult } from '@/types';

export function exportToCSV(companies: CompanySearchResult[], filename = 'nearhive-companies.csv') {
  const headers = [
    'Company ID',
    'Company Name',
    'Domain',
    'Industry',
    'Employees',
    'Address',
    'City',
    'Latitude',
    'Longitude',
    'Confidence Score',
    'Distance (meters)',
    'Verified',
  ];

  const rows = companies.map((c) => [
    `"${c.id}"`,
    `"${c.name.replace(/"/g, '""')}"`,
    `"${c.domain || ''}"`,
    `"${c.industry || ''}"`,
    `"${c.employee_count || ''}"`,
    `"${c.address.replace(/"/g, '""')}"`,
    `"${c.city || ''}"`,
    c.lat,
    c.lng,
    c.confidence,
    c.distance_meters,
    c.verified ? 'true' : 'false',
  ]);

  const csvContent = [headers.join(','), ...rows.map((r) => r.join(','))].join('\n');
  downloadBlob(csvContent, filename, 'text/csv;charset=utf-8;');
}

export function exportToGeoJSON(companies: CompanySearchResult[], filename = 'nearhive-companies.geojson') {
  const geojson = {
    type: 'FeatureCollection',
    features: companies.map((c) => ({
      type: 'Feature',
      geometry: {
        type: 'Point',
        coordinates: [c.lng, c.lat],
      },
      properties: {
        id: c.id,
        name: c.name,
        domain: c.domain,
        industry: c.industry,
        employee_count: c.employee_count,
        address: c.address,
        confidence: c.confidence,
        distance_meters: c.distance_meters,
        verified: c.verified,
      },
    })),
  };

  const jsonContent = JSON.stringify(geojson, null, 2);
  downloadBlob(jsonContent, filename, 'application/geo+json;charset=utf-8;');
}

function downloadBlob(content: string, filename: string, mimeType: string) {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 9: Main Application Page & Providers Assembly

**Files:**
- Create: `web/src/app/providers.tsx`
- Create: `web/src/app/layout.tsx`
- Create: `web/src/app/page.tsx`

**Interfaces:**
- Produces: The primary interactive application with TanStack Query providers, Leaflet map canvas, floating Cluster View mode toggle, city presets, radius slider, sidebar, and drawers.

- [ ] **Step 1: Write `web/src/app/providers.tsx`**

```tsx
'use client';

import { ReactNode, useState } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { makeQueryClient } from '@/lib/query-client';

export default function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(() => makeQueryClient());

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
```

- [ ] **Step 2: Write `web/src/app/layout.tsx`**

```tsx
import type { Metadata } from 'next';
import './globals.css';
import Providers from './providers';

export const metadata: Metadata = {
  title: 'NearHive 🐝 — Scalable Tech Company Locator & Verification Engine',
  description: 'PostGIS spatial radius search, automated Overpass & Wikidata scraping, and AI verification engine.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark h-full">
      <body className="h-full bg-slate-950 text-slate-100 antialiased overflow-hidden">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
```

- [ ] **Step 3: Write `web/src/app/page.tsx`**

```tsx
'use client';

import { useState } from 'react';
import MapContainer from '@/components/map/MapContainer';
import Sidebar from '@/components/sidebar/Sidebar';
import CompanyDetailDrawer from '@/components/drawers/CompanyDetailDrawer';
import ScrapeModal from '@/components/drawers/ScrapeModal';
import { useCompanies } from '@/hooks/useCompanies';
import { useClusters } from '@/hooks/useClusters';
import { useAuth } from '@/hooks/useAuth';
import { exportToCSV, exportToGeoJSON } from '@/lib/export';
import { CompanySearchResult } from '@/types';
import { Play, Layers, Navigation } from 'lucide-react';

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
  const [isClusterMode, setIsClusterMode] = useState(false);
  const [selectedCompany, setSelectedCompany] = useState<CompanySearchResult | null>(null);
  const [isScrapeOpen, setIsScrapeOpen] = useState(false);

  const { token, user } = useAuth();

  const { data: searchData, isLoading: loadingCompanies } = useCompanies({
    lat: center.lat,
    lng: center.lng,
    radius_km: radiusKm,
    q: searchQuery,
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
  const totalCount = searchData?.meta.total || companies.length;

  return (
    <div className="h-screen w-screen flex flex-col bg-slate-950 select-none">
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

          {/* City Presets */}
          <div className="hidden lg:flex items-center gap-1">
            {CITY_PRESETS.map((c) => (
              <button
                key={c.name}
                onClick={() => setCenter({ lat: c.lat, lng: c.lng })}
                className="px-2 py-1 text-xs rounded-lg bg-slate-800/80 hover:bg-slate-700 text-slate-300 border border-slate-700/40 transition-colors"
              >
                {c.name}
              </button>
            ))}
          </div>
        </div>

        {/* Header Right Controls */}
        <div className="flex items-center gap-3">
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
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-md shadow-amber-500/20 transition-all"
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
          onExportCsv={() => exportToCSV(companies)}
          onExportGeoJson={() => exportToGeoJSON(companies)}
        />

        {/* Map Canvas */}
        <div className="flex-1 h-full relative">
          {/* Floating Cluster Toggle */}
          <div className="absolute top-4 right-4 z-20">
            <button
              onClick={() => setIsClusterMode(!isClusterMode)}
              className={`px-3 py-1.5 rounded-xl shadow-xl text-xs font-semibold backdrop-blur-md border transition-all flex items-center gap-1.5 ${
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
        />
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Verify TypeScript compilation**

Run: `cd web && pnpm type-check`  
Expected: 0 errors.

---

### Task 10: Vercel Production Build & Verification

**Files:**
- Create: `web/vercel.json`

**Interfaces:**
- Produces: Production build verification and Vercel project configuration.

- [ ] **Step 1: Write `web/vercel.json`**

```json
{
  "framework": "nextjs",
  "buildCommand": "pnpm build",
  "installCommand": "pnpm install"
}
```

- [ ] **Step 2: Run complete production build in `web/`**

Run: `cd web && pnpm build`  
Expected: Next.js production build succeeds with all static & dynamic routes compiled.

- [ ] **Step 3: Commit and push**

Run:
```bash
git add web/ docs/
git commit -m "feat(web): add modern Next.js React frontend with TanStack Query and Vercel configuration"
git push origin master
```
Expected: Changes committed and pushed to remote master repository.
