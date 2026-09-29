'use client';

import { useEffect, useRef } from 'react';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { ClusterViewport, CompanySearchResult, SpatialCluster } from '@/types';

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
  onViewportChange: (viewport: ClusterViewport) => void;
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
  onViewportChange,
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
  const onViewportChangeRef = useRef(onViewportChange);

  useEffect(() => {
    onCenterChangeRef.current = onCenterChange;
    onSelectCompanyRef.current = onSelectCompany;
    onClusterZoomRef.current = onClusterZoom;
    onViewportChangeRef.current = onViewportChange;
  });

  useEffect(() => {
    if (!mapContainerRef.current || mapRef.current) return;

    const map = L.map(mapContainerRef.current, {
      center: [center.lat, center.lng],
      zoom: 12,
      zoomControl: false,
    });

    L.control.zoom({ position: 'bottomright' }).addTo(map);

    const reportViewport = () => {
      const bounds = map.getBounds();
      const round = (value: number) => Number(value.toFixed(5));
      onViewportChangeRef.current({
        west: round(bounds.getWest()),
        south: round(bounds.getSouth()),
        east: round(bounds.getEast()),
        north: round(bounds.getNorth()),
        zoom: map.getZoom(),
      });
    };
    map.on('moveend zoomend', reportViewport);
    map.whenReady(reportViewport);

    // OpenStreetMap tile layer styled with sleek dark CSS filter in dark-map-tiles class
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      className: 'dark-map-tiles',
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap</a> contributors',
      keepBuffer: 16,
      updateWhenIdle: false,
      updateWhenZooming: true,
    }).addTo(map);

    const markerLayer = L.layerGroup().addTo(map);
    markerLayerRef.current = markerLayer;

    const epicenterIcon = L.divIcon({
      className: 'epicenter-marker',
      html: `
        <div role="button" aria-label="Search center location, draggable marker" class="relative flex items-center justify-center w-8 h-8 -ml-4 -mt-4 cursor-grab active:cursor-grabbing">
          <div class="absolute w-8 h-8 rounded-full bg-indigo-500/30 epicenter-pulse"></div>
          <div class="w-4 h-4 rounded-full bg-indigo-500 border-2 border-[#09090b] shadow-lg shadow-indigo-500/50"></div>
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
      color: '#6366f1',
      weight: 1.5,
      opacity: 0.8,
      fillColor: '#6366f1',
      fillOpacity: 0.08,
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
        const canZoom = c.count > 1 && mapRef.current!.getZoom() < 18;
        const size = Math.min(56, Math.max(34, 26 + Math.log2(c.count + 1) * 5));
        const icon = L.divIcon({
          className: 'cluster-pin',
          html: `
              <div role="button" aria-label="${c.count} ${c.count === 1 ? 'office' : 'offices'}, click to ${canZoom ? 'zoom in' : 'show nearby pins'}" class="flex items-center justify-center rounded-full shadow-2xl border-2 border-indigo-400 bg-indigo-600/90 text-white font-mono font-black transition-transform hover:scale-110 cursor-pointer shadow-indigo-500/30" style="width: ${size}px; height: ${size}px; margin-left: -${size / 2}px; margin-top: -${size / 2}px; font-size: ${size > 42 ? '12px' : '10px'}">
              ${c.count}
            </div>
          `,
          iconSize: [size, size],
        });

        const marker = L.marker([c.lat, c.lng], {
          icon,
          title: `${c.count} ${c.count === 1 ? 'office' : 'offices'}`,
          alt: `${c.count} ${c.count === 1 ? 'office' : 'offices'}`,
        });
        marker.on('click', () => {
          const map = mapRef.current;
          if (!map) return;
          if (c.count > 1 && map.getZoom() < 18) {
            const bounds = L.latLngBounds([c.south, c.west], [c.north, c.east]);
            const zoom = Math.min(18, Math.max(map.getZoom() + 1, map.getBoundsZoom(bounds.pad(0.2))));
            map.setView([c.lat, c.lng], zoom, { animate: true });
          } else {
            map.setView([c.lat, c.lng], Math.max(map.getZoom(), 16), { animate: true });
            onClusterZoomRef.current(c.lat, c.lng);
          }
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
              ${isSelected ? '<div class="absolute -inset-2.5 rounded-2xl bg-indigo-500/40 animate-ping pointer-events-none"></div>' : ''}
              <div class="w-7 h-7 rounded-xl shadow-lg border-2 ${
                isSelected
                  ? 'border-indigo-400 ring-2 ring-indigo-400/60 shadow-indigo-500/50'
                  : 'border-[#09090b] shadow-black/60'
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
            <div style="font-weight: 700; font-size: 13px; color: #f4f4f5; line-height: 1.3;">${escapeHtml(comp.name)}</div>
            ${comp.industry ? `<div style="font-size: 10px; color: #a1a1aa; margin-top: 2px;">${escapeHtml(comp.industry)}</div>` : ''}
            <div style="font-size: 11px; color: #d4d4d8; margin-top: 5px; line-height: 1.3;">${escapeHtml(comp.address || 'Office location')}</div>
            <div style="margin-top: 8px; padding-top: 6px; border-top: 1px solid rgba(255,255,255,0.1); display: flex; align-items: center; justify-content: space-between; font-size: 10px;">
              <span style="font-weight: 700; color: #10b981;">✓ ${conf}% Verified</span>
              <span style="color: #a1a1aa; font-family: monospace;">${((comp.distance_meters || 0) / 1000).toFixed(1)} km</span>
            </div>
          </div>
        `, {
          closeButton: true,
          className: 'company-leaflet-popup',
        });

        marker.on('click', () => {
          onSelectCompanyRef.current(comp);
        });

        markerLayerRef.current?.addLayer(marker);
        markerMapRef.current.set(`${comp.id}_${comp.location_id}`, marker);
      });
    }
  }, [companies, clusters, isClusterMode, selectedCompany]);

  useEffect(() => {
    if (!selectedCompany || isClusterMode) return;
    const marker = markerMapRef.current.get(
      `${selectedCompany.id}_${selectedCompany.location_id}`
    );

    if (mapRef.current && selectedCompany.lat && selectedCompany.lng) {
      mapRef.current.setView(
        [selectedCompany.lat, selectedCompany.lng],
        Math.max(mapRef.current.getZoom(), 14),
        { animate: true }
      );
    }

    if (marker) {
      setTimeout(() => {
        marker.openPopup();
      }, 400);
    }
  }, [selectedCompany, isClusterMode]);

  return <div ref={mapContainerRef} className="w-full h-full relative z-0 bg-[#09090b]" />;
}
