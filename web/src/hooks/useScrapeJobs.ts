import { useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import { ScrapeJob } from '@/types';

export function useScrapeJob(jobId: string | null) {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ['job', jobId],
    queryFn: () => fetchApi<ScrapeJob>(`/api/v1/jobs/${jobId}`),
    enabled: !!jobId,
    refetchInterval: (q) => {
      const s = q.state.data?.status;
      return s === 'running' || s === 'pending' ? 2000 : false;
    },
  });

  const status = query.data?.status;
  useEffect(() => {
    if (status === 'done' || status === 'cancelled') {
      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
    }
  }, [status, queryClient]);

  return query;
}

export function useScrapeJobs() {
  return useQuery({
    queryKey: ['jobs'],
    queryFn: () => fetchApi<{ jobs: ScrapeJob[] }>('/api/v1/jobs'),
    refetchInterval: (q) => {
      const jobs = q.state.data?.jobs || [];
      const hasActive = jobs.some((j) => j.status === 'running' || j.status === 'pending');
      return hasActive ? 2500 : 10000;
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
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
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
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
    },
  });
}
