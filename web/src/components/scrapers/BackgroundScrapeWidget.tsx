'use client';

import { useDiscoveryJobs } from '@/hooks/useDiscoveryJobs';

interface Props {
  activeJobIds: string[];
  onOpenDetails: () => void;
  onDismissAll: () => void;
}

export default function BackgroundScrapeWidget({ activeJobIds, onOpenDetails, onDismissAll }: Props) {
  const { data } = useDiscoveryJobs();
  const active = (data?.jobs || []).filter((job) => job.status === 'queued' || job.status === 'in_progress');
  const recent = (data?.jobs || []).filter((job) => activeJobIds.includes(job.id));
  if (!active.length && !recent.length) return null;
  return <aside className="fixed bottom-6 right-6 z-40 rounded-xl border border-amber-500/40 bg-slate-900 p-4 text-sm text-slate-100 shadow-xl">
    <button type="button" onClick={onOpenDetails} className="font-semibold text-amber-400">{active.length ? `${active.length} discovery run${active.length === 1 ? '' : 's'} active` : 'Discovery finished'}</button>
    {!active.length && <button type="button" onClick={onDismissAll} aria-label="Dismiss discovery status" className="ml-4 text-slate-400 hover:text-white">✕</button>}
  </aside>;
}
