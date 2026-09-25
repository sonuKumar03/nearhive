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
