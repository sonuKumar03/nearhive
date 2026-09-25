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
    enabled: enabled && !Number.isNaN(params.lat) && !Number.isNaN(params.lng),
    placeholderData: (prev) => prev,
  });
}
