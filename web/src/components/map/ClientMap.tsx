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

  // Store latest callbacks in refs to avoid stale closures
  const onCenterChangeRef = useRef(onCenterChange);
  const onSelectCompanyRef = useRef(onSelectCompany);
  const onClusterZoomRef = useRef(onClusterZoom);

  useEffect(() => {
    onCenterChangeRef.current = onCenterChange;
    onSelectCompanyRef.current = onSelectCompany;
    onClusterZoomRef.current = onClusterZoom;
  });

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

    const epicenterIcon = L.divIcon({
      className: 'epicenter-marker',
      html: `
        <div role="button" aria-label="Search center location, draggable marker" class="relative flex items-center justify-center w-8 h-8 -ml-4 -mt-4 cursor-grab active:cursor-grabbing">
          <div class="absolute w-8 h-8 rounded-full bg-amber-500/30 epicenter-pulse"></div>
          <div class="w-4 h-4 rounded-full bg-amber-500 border-2 border-slate-950 shadow-lg shadow-amber-500/50"></div>
        </div>
      `,
      iconSize: [32, 32],
    });

    const epicenter = L.marker([center.lat, center.lng], {
      icon: epicenterIcon,
      draggable: true,
      title: 'Search center epicenter: drag or click anywhere on map to reposition',
      alt: 'Search center epicenter marker',
    }).addTo(map);

    epicenter.on('dragend', (e) => {
      const pos = e.target.getLatLng();
      onCenterChangeRef.current(pos.lat, pos.lng);
    });

    // Clicking anywhere on map repositions the search epicenter
    map.on('click', (e: L.LeafletMouseEvent) => {
      onCenterChangeRef.current(e.latlng.lat, e.latlng.lng);
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
      epicenterRef.current = null;
      circleRef.current = null;
      markerLayerRef.current = null;
    };
  }, []);

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

  useEffect(() => {
    if (!markerLayerRef.current || !mapRef.current) return;
    markerLayerRef.current.clearLayers();

    if (isClusterMode) {
      clusters.forEach((c) => {
        if (!c.lat || !c.lng) return;
        const size = Math.min(56, Math.max(34, 26 + Math.log2(c.count + 1) * 5));
        const icon = L.divIcon({
          className: 'cluster-pin',
          html: `
            <div role="button" aria-label="Cluster of ${c.count} companies, click to zoom" class="flex items-center justify-center rounded-full shadow-2xl border-2 border-amber-400 bg-amber-500/90 text-slate-950 font-mono font-black transition-transform hover:scale-110 cursor-pointer" style="width: ${size}px; height: ${size}px; margin-left: -${size / 2}px; margin-top: -${size / 2}px; font-size: ${size > 42 ? '12px' : '10px'}">
              ${c.count}
            </div>
          `,
          iconSize: [size, size],
        });

        const marker = L.marker([c.lat, c.lng], {
          icon,
          title: `Cluster of ${c.count} companies`,
          alt: `Cluster of ${c.count} companies`,
        });
        marker.on('click', () => {
          onClusterZoomRef.current(c.lat, c.lng);
        });
        markerLayerRef.current?.addLayer(marker);
      });
    } else {
      companies.forEach((comp) => {
        if (!comp.lat || !comp.lng) return;
        const conf = Math.round(comp.confidence * 100);
        const markerColor = conf >= 80 ? '#10b981' : conf >= 60 ? '#f59e0b' : '#64748b';

        const customIcon = L.divIcon({
          className: 'company-pin',
          html: `
            <div role="button" aria-label="${comp.name}, ${conf}% verified, click to view details" class="flex items-center justify-center w-7 h-7 -ml-3.5 -mt-3.5 rounded-xl shadow-lg border border-slate-900/60 transition-transform hover:scale-110 cursor-pointer" style="background-color: ${markerColor}">
              <span class="text-xs">🏢</span>
            </div>
          `,
          iconSize: [28, 28],
        });

        const marker = L.marker([comp.lat, comp.lng], {
          icon: customIcon,
          title: `${comp.name} (${conf}% verified)`,
          alt: `${comp.name} office pin`,
        });
        marker.on('click', () => onSelectCompanyRef.current(comp));
        markerLayerRef.current?.addLayer(marker);
      });
    }
  }, [companies, clusters, isClusterMode]);

  return <div ref={mapContainerRef} className="w-full h-full relative z-0" />;
}
