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
  selectedCompany?: CompanySearchResult | null;
  onCenterChange: (lat: number, lng: number) => void;
  onSelectCompany: (company: CompanySearchResult) => void;
  onClusterZoom: (lat: number, lng: number) => void;
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

export default function ClientMap({
  center,
  radiusKm,
  companies,
  clusters,
  isClusterMode,
  selectedCompany,
  onCenterChange,
  onSelectCompany,
  onClusterZoom,
}: ClientMapProps) {
  const mapRef = useRef<L.Map | null>(null);
  const mapContainerRef = useRef<HTMLDivElement>(null);
  const epicenterRef = useRef<L.Marker | null>(null);
  const circleRef = useRef<L.Circle | null>(null);
  const markerLayerRef = useRef<L.LayerGroup | null>(null);
  const markerMapRef = useRef<Map<string, L.Marker>>(new Map());

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
    markerMapRef.current.clear();

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
        const isSelected =
          selectedCompany?.id === comp.id && selectedCompany?.location_id === comp.location_id;
        const conf = Math.round(comp.confidence * 100);
        const markerColor = conf >= 80 ? '#10b981' : conf >= 60 ? '#f59e0b' : '#64748b';

        const customIcon = L.divIcon({
          className: 'company-pin-wrapper',
          html: `
            <div role="button" aria-label="${escapeHtml(comp.name)}, ${conf}% verified" class="relative flex items-center justify-center cursor-pointer transition-all ${
              isSelected ? 'scale-125 z-40' : 'hover:scale-110 z-10'
            }">
              ${isSelected ? '<div class="absolute -inset-2.5 rounded-2xl bg-amber-400/50 animate-ping pointer-events-none"></div>' : ''}
              <div class="w-7 h-7 rounded-xl shadow-lg border-2 ${
                isSelected
                  ? 'border-amber-300 ring-2 ring-amber-400/60 shadow-amber-500/50'
                  : 'border-slate-950/80 shadow-black/40'
              } flex items-center justify-center text-xs text-white" style="background-color: ${markerColor}">
                <span>🏢</span>
              </div>
            </div>
          `,
          iconSize: [28, 28],
          iconAnchor: [14, 14],
          popupAnchor: [0, -16],
        });

        const marker = L.marker([comp.lat, comp.lng], {
          icon: customIcon,
          title: `${comp.name} (${conf}% verified)`,
          alt: `${comp.name} office pin`,
          zIndexOffset: isSelected ? 1000 : 0,
        });

        marker.bindPopup(`
          <div style="font-family: inherit; min-width: 170px; padding: 2px;">
            <div style="font-weight: 700; font-size: 13px; color: #f8fafc; line-height: 1.3;">${escapeHtml(comp.name)}</div>
            ${comp.industry ? `<div style="font-size: 10px; color: #94a3b8; margin-top: 2px;">${escapeHtml(comp.industry)}</div>` : ''}
            <div style="font-size: 11px; color: #cbd5e1; margin-top: 5px; line-height: 1.3;">${escapeHtml(comp.address || 'Office location')}</div>
            <div style="margin-top: 8px; padding-top: 6px; border-top: 1px solid #334155; display: flex; align-items: center; justify-content: space-between; font-size: 10px;">
              <span style="font-weight: 700; color: #34d399;">✓ ${conf}% Verified</span>
              <span style="color: #94a3b8; font-family: monospace;">${((comp.distance_meters || 0) / 1000).toFixed(1)} km</span>
            </div>
          </div>
        `, {
          closeButton: true,
          className: 'company-leaflet-popup',
        });

        marker.on('click', () => onSelectCompanyRef.current(comp));
        const markerKey = `${comp.id}-${comp.location_id}`;
        markerMapRef.current.set(markerKey, marker);
        markerLayerRef.current?.addLayer(marker);
      });
    }
  }, [companies, clusters, isClusterMode, selectedCompany?.id, selectedCompany?.location_id]);

  // When a company is selected (e.g. from the sidebar), fly to it and open its popup
  useEffect(() => {
    if (!selectedCompany || !mapRef.current || isClusterMode) return;
    const markerKey = `${selectedCompany.id}-${selectedCompany.location_id}`;
    const marker = markerMapRef.current.get(markerKey);

    if (selectedCompany.lat && selectedCompany.lng) {
      mapRef.current.flyTo(
        [selectedCompany.lat, selectedCompany.lng],
        Math.max(mapRef.current.getZoom(), 15),
        { animate: true, duration: 0.8 }
      );
    }

    if (marker) {
      setTimeout(() => {
        marker.openPopup();
      }, 400);
    }
  }, [selectedCompany, isClusterMode]);

  return <div ref={mapContainerRef} className="w-full h-full relative z-0" />;
}
