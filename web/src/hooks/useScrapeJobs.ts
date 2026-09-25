import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { ScrapeJob } from '@/types';

export function useScrapeJob(jobId: string | null) {
  const queryClient = useQueryClient();

  return useQuery({
    queryKey: ['job', jobId],
    queryFn: async () => {
      const job = await fetchApi<ScrapeJob>(`/api/v1/jobs/${jobId}`);
      if (job.status === 'done' || job.status === 'cancelled') {
        queryClient.invalidateQueries({ queryKey: ['companies'] });
        queryClient.invalidateQueries({ queryKey: ['clusters'] });
      }
      return job;
    },
    enabled: !!jobId,
    refetchInterval: (query) => {
      const s = query.state.data?.status;
      return s === 'running' || s === 'pending' ? 2000 : false;
    },
  });
}

export function useTriggerScraper() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: { region: string; radius_km: number; lat?: number; lng?: number }) =>
      fetchApi<ScrapeJob>('/api/v1/jobs/trigger', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (job) => {
      queryClient.setQueryData(['job', job.id], job);
    },
  });
}

export function useCancelScraper() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (jobId: string) =>
      fetchApi<{ message: string; job: ScrapeJob }>(`/api/v1/jobs/${jobId}/cancel`, {
        method: 'POST',
      }),
    onSuccess: (res, jobId) => {
      queryClient.setQueryData(['job', jobId], res.job);
      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
    },
  });
}
