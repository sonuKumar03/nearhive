import { useQuery } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { CompanyDetailResponse, Sighting } from '@/types';

export function useCompany(companyId: string | null) {
  return useQuery({
    queryKey: ['company', companyId],
    queryFn: () => fetchApi<CompanyDetailResponse>(`/api/v1/companies/${companyId}`),
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
