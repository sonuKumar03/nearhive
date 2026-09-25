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
    enabled: enabled && !Number.isNaN(params.lat) && !Number.isNaN(params.lng),
  });
}
