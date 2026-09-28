import { useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchApi } from '@/lib/api-client';
import {
  DiscoveryJob,
  CreateDiscoveryJobParams,
  CompanyTechnicalJobsResponse,
} from '@/types';

export function useDiscoveryJob(jobId: string | null) {
  const queryClient = useQueryClient();
  const prevSignatureRef = useRef<string | null>(null);

  const query = useQuery({
    queryKey: ['discovery-job', jobId],
    queryFn: () => fetchApi<DiscoveryJob>(`/api/v1/discovery/jobs/${jobId}`),
    enabled: !!jobId,
    refetchInterval: (q) => {
      const s = q.state.data?.status;
      return s === 'in_progress' || s === 'queued' ? 2000 : false;
    },
  });

  const job = query.data;
  useEffect(() => {
    if (!job) return;

    const signature = `${job.id}:${job.status}`;

    if (prevSignatureRef.current === null) {
      prevSignatureRef.current = signature;
      return;
    }

    if (prevSignatureRef.current !== signature) {
      prevSignatureRef.current = signature;

      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
      queryClient.invalidateQueries({ queryKey: ['company'] });
      queryClient.invalidateQueries({ queryKey: ['technical-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['company-sightings'] });
      queryClient.invalidateQueries({ queryKey: ['discovery-jobs'] });
    }
  }, [job, queryClient]);

  return query;
}

export function useDiscoveryJobs() {
  const queryClient = useQueryClient();
  const prevSignatureRef = useRef<string | null>(null);

  const query = useQuery({
    queryKey: ['discovery-jobs'],
    queryFn: () => fetchApi<{ jobs: DiscoveryJob[] }>('/api/v1/discovery/jobs'),
    refetchInterval: (q) => {
      const jobs = q.state.data?.jobs || [];
      const hasActive = jobs.some((j) => j.status === 'in_progress' || j.status === 'queued');
      return hasActive ? 2000 : 10000;
    },
  });

  const jobs = query.data?.jobs;

  // Refresh search results when a run changes state.
  useEffect(() => {
    if (!jobs) return;

    const parts: string[] = [];
    for (const job of jobs) {
      parts.push(`${job.id}:${job.status}`);
    }
    const currentSignature = parts.join('|');

    if (prevSignatureRef.current === null) {
      prevSignatureRef.current = currentSignature;
      return;
    }

    if (prevSignatureRef.current !== currentSignature) {
      prevSignatureRef.current = currentSignature;

      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['nearby-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
      queryClient.invalidateQueries({ queryKey: ['company'] });
      queryClient.invalidateQueries({ queryKey: ['technical-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['company-sightings'] });
    }
  }, [jobs, queryClient]);

  return query;
}

export function useTriggerDiscovery() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateDiscoveryJobParams) =>
      fetchApi<DiscoveryJob>('/api/v1/discovery/jobs', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (job) => {
      queryClient.setQueryData(['discovery-job', job.id], job);
      queryClient.invalidateQueries({ queryKey: ['discovery-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['nearby-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
    },
  });
}

export function useCancelDiscovery() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (jobId: string) =>
      fetchApi<DiscoveryJob>(`/api/v1/discovery/jobs/${jobId}/cancel`, {
        method: 'POST',
      }),
    onSuccess: (res, jobId) => {
      queryClient.setQueryData(['discovery-job', jobId], res);
      queryClient.invalidateQueries({ queryKey: ['discovery-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['companies'] });
      queryClient.invalidateQueries({ queryKey: ['nearby-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['clusters'] });
      queryClient.invalidateQueries({ queryKey: ['company'] });
      queryClient.invalidateQueries({ queryKey: ['technical-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['company-sightings'] });
    },
  });
}

export function useCompanyTechnicalJobs(companyId: string | null, days: number = 14) {
  return useQuery({
    queryKey: ['technical-jobs', companyId, days],
    queryFn: () =>
      fetchApi<CompanyTechnicalJobsResponse>(
        `/api/v1/companies/${companyId}/technical-jobs?days=${days}`
      ),
    enabled: !!companyId,
  });
}
