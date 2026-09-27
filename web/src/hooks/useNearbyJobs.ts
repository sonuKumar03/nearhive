import { useQuery } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { NearbyJobsParams, TechnicalJobSearchResponse } from '@/types';

export function useNearbyJobs(params: NearbyJobsParams, enabled = true) {
  return useQuery({
    queryKey: ['nearby-jobs', params],
    queryFn: async () => {
      const q = new URLSearchParams({
        lat: params.lat.toString(),
        lng: params.lng.toString(),
      });
      if (params.radius_km !== undefined) q.set('radius', params.radius_km.toString());
      if (params.q) q.set('q', params.q);
      if (params.work_arrangement) q.set('work_arrangement', params.work_arrangement);
      if (params.limit) q.set('limit', params.limit.toString());
      if (params.page) q.set('page', params.page.toString());

      return fetchApi<TechnicalJobSearchResponse>(`/api/v1/search/jobs?${q.toString()}`);
    },
    enabled: enabled && !Number.isNaN(params.lat) && !Number.isNaN(params.lng),
    placeholderData: (prev) => prev,
  });
}
