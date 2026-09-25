'use client';

import dynamic from 'next/dynamic';
import { CompanySearchResult, SpatialCluster } from '@/types';

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
