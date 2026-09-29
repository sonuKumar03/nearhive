'use client';

import { useDiscoveryJobs } from '@/hooks/useDiscoveryJobs';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Search, X, ArrowRight } from 'lucide-react';

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

  return (
    <aside className="fixed bottom-6 right-6 z-40 flex items-center gap-3 rounded-2xl border border-indigo-500/30 bg-[#121216]/95 p-3.5 text-sm text-zinc-100 shadow-2xl shadow-black/80 backdrop-blur-md animate-in slide-in-from-bottom duration-300">
      <div className="flex items-center gap-2.5">
        {active.length > 0 ? (
          <Badge variant="indigo" pulsing size="sm">
            <span className="font-semibold">{active.length} active</span>
          </Badge>
        ) : (
          <Badge variant="emerald" size="sm" icon={<span>✓</span>}>
            Finished
          </Badge>
        )}

        <button
          type="button"
          onClick={onOpenDetails}
          className="font-semibold text-xs text-zinc-200 hover:text-indigo-400 flex items-center gap-1.5 transition-colors cursor-pointer"
        >
          <span>
            {active.length
              ? `Discovery running (${active.length} ${active.length === 1 ? 'task' : 'tasks'})`
              : 'Discovery complete'}
          </span>
          <ArrowRight className="w-3 h-3 text-indigo-400" />
        </button>
      </div>

      {!active.length && (
        <button
          type="button"
          onClick={onDismissAll}
          aria-label="Dismiss discovery status"
          className="p-1 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors cursor-pointer"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      )}
    </aside>
  );
}
