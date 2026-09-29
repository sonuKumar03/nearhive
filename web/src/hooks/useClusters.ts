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
      });
      if (params.viewport) {
        q.set('zoom', params.viewport.zoom.toString());
        for (const key of ['west', 'south', 'east', 'north'] as const) {
          q.set(key, params.viewport[key].toString());
        }
      } else {
        q.set('k', (params.k || 20).toString());
      }
      if (params.q) q.set('q', params.q);
      return fetchApi<ClusterResponse>(`/api/v1/search/clusters?${q.toString()}`);
    },
    enabled: enabled && !!params.viewport && !Number.isNaN(params.lat) && !Number.isNaN(params.lng),
  });
}
