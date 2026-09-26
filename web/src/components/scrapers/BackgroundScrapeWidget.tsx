'use client';

import { useState } from 'react';
import { useScrapeJobs, useCancelScraper } from '@/hooks/useScrapeJobs';
import { ScrapeTask, ScrapeJob } from '@/types';
import {
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  StopCircle,
  ExternalLink,
  X,
  Sparkles,
  ChevronUp,
  ChevronDown,
} from 'lucide-react';

interface BackgroundScrapeWidgetProps {
  activeJobIds: string[];
  onOpenDetails: () => void;
  onDismissJob: (id: string) => void;
  onDismissAll: () => void;
}

export default function BackgroundScrapeWidget({
  activeJobIds,
  onOpenDetails,
  onDismissJob,
  onDismissAll,
}: BackgroundScrapeWidgetProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const { data: jobsData } = useScrapeJobs();
  const cancelMutation = useCancelScraper();

  const allJobs = jobsData?.jobs || [];

  // Filter jobs that are either actively tracked in state or currently in-flight
  const relevantJobs = allJobs.filter(
    (j) => activeJobIds.includes(j.id) || j.status === 'running' || j.status === 'pending'
  );

  if (relevantJobs.length === 0) return null;

  const runningJobs = relevantJobs.filter((j) => j.status === 'running' || j.status === 'pending');
  const isAnyRunning = runningJobs.length > 0;
  const totalSightings = relevantJobs.reduce((acc, j) => acc + (j.sightings || 0), 0);

  // If only 1 job, render the single-job card
  if (relevantJobs.length === 1) {
    const job = relevantJobs[0];
    const isRunning = job.status === 'running' || job.status === 'pending';
    const isDone = job.status === 'done';
    const isCancelled = job.status === 'cancelled';

    const tasks: ScrapeTask[] = job.tasks || [];
    const completedTasks = tasks.filter(
      (t) => t.status === 'done' || t.status === 'cancelled' || t.status === 'failed'
    ).length;
    const totalTasks = Math.max(tasks.length, 3);
    const progressPercent = Math.round((completedTasks / totalTasks) * 100);

    return (
      <aside
        aria-label="Background Scraper Status"
        className={`fixed bottom-5 right-5 z-40 w-96 max-w-[calc(100vw-2.5rem)] rounded-2xl p-4 shadow-2xl backdrop-blur-xl border transition-all duration-300 animate-in slide-in-from-bottom-5 ${
          isRunning
            ? 'bg-slate-900/95 border-amber-500/50 shadow-amber-500/10'
            : isDone
            ? 'bg-slate-900/95 border-emerald-500/50 shadow-emerald-500/10'
            : isCancelled
            ? 'bg-slate-900/95 border-slate-700 shadow-slate-900/50'
            : 'bg-slate-900/95 border-rose-500/50 shadow-rose-500/10'
        }`}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-2">
            {isRunning ? (
              <div className="w-7 h-7 rounded-lg bg-amber-500/20 border border-amber-500/40 flex items-center justify-center text-amber-400 shrink-0">
                <RefreshCw className="w-3.5 h-3.5 animate-spin" />
              </div>
            ) : isDone ? (
              <div className="w-7 h-7 rounded-lg bg-emerald-500/20 border border-emerald-500/40 flex items-center justify-center text-emerald-400 shrink-0">
                <Sparkles className="w-3.5 h-3.5" />
              </div>
            ) : (
              <div className="w-7 h-7 rounded-lg bg-slate-800 border border-slate-700 flex items-center justify-center text-slate-400 shrink-0">
                <AlertCircle className="w-3.5 h-3.5" />
              </div>
            )}

            <div>
              <div className="flex items-center gap-1.5">
                <h4 className="font-bold text-xs text-slate-100">
                  {isRunning
                    ? 'Scraping Active'
                    : isDone
                    ? 'Scraping Completed'
                    : isCancelled
                    ? 'Scraping Cancelled'
                    : 'Scraping Failed'}
                </h4>
                <span
                  className={`text-[9px] font-mono font-semibold px-1.5 py-0.5 rounded-full uppercase ${
                    isRunning
                      ? 'bg-amber-500/20 text-amber-300'
                      : isDone
                      ? 'bg-emerald-500/20 text-emerald-300'
                      : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  {job.status}
                </span>
              </div>
              <p className="text-[11px] text-slate-400">
                Hub: <span className="font-semibold text-slate-200">{job.region || 'Coordinates'}</span>
              </p>
            </div>
          </div>

          <button
            onClick={() => onDismissJob(job.id)}
            className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800/80 transition-colors cursor-pointer"
            title="Dismiss status"
            aria-label="Dismiss status widget"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>

        <div className="mt-3 space-y-2">
          {isRunning && (
            <div>
              <div className="flex items-center justify-between text-[10px] text-slate-400 mb-1">
                <span>Crawl Pipeline Progress</span>
                <span className="font-mono font-semibold text-amber-400">{progressPercent}%</span>
              </div>
              <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-amber-500 to-amber-400 rounded-full transition-all duration-500"
                  style={{ width: `${Math.max(progressPercent, 12)}%` }}
                />
              </div>
            </div>
          )}

          <div className="p-2.5 rounded-xl bg-slate-950/70 border border-slate-800/80 flex items-center justify-between text-xs">
            <span className="text-slate-400 text-[11px]">Discovered Sightings:</span>
            <span className="font-mono font-bold text-amber-400 text-sm">
              {job.sightings} <span className="text-[10px] font-normal text-slate-400">records</span>
            </span>
          </div>

          {tasks.length > 0 && (
            <div className="flex items-center gap-1.5 flex-wrap pt-1">
              {tasks.map((task) => (
                <span
                  key={task.id}
                  className="text-[10px] font-mono px-2 py-0.5 rounded-md bg-slate-950 border border-slate-800 text-slate-300 flex items-center gap-1"
                >
                  {task.status === 'running' ? (
                    <RefreshCw className="w-2.5 h-2.5 animate-spin text-amber-400" />
                  ) : task.status === 'done' ? (
                    <CheckCircle2 className="w-2.5 h-2.5 text-emerald-400" />
                  ) : (
                    <AlertCircle className="w-2.5 h-2.5 text-slate-500" />
                  )}
                  <span className="uppercase">{task.source}</span>
                </span>
              ))}
            </div>
          )}
        </div>

        <div className="mt-3 pt-2.5 border-t border-slate-800/80 flex items-center justify-between gap-2">
          {isRunning ? (
            <button
              onClick={() => cancelMutation.mutate(job.id)}
              disabled={cancelMutation.isPending}
              className="px-2.5 py-1 text-[11px] font-semibold rounded-lg bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30 flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <StopCircle className="w-3 h-3" />
              <span>Cancel</span>
            </button>
          ) : (
            <span className="text-[10px] text-slate-400">
              {isDone ? 'Map updated' : 'Finished'}
            </span>
          )}

          <div className="flex items-center gap-2">
            <button
              onClick={onOpenDetails}
              className="px-3 py-1 text-[11px] font-semibold rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/60 flex items-center gap-1 transition-colors cursor-pointer"
            >
              <span>View Tasks</span>
              <ExternalLink className="w-2.5 h-2.5" />
            </button>

            {isDone && (
              <button
                onClick={() => onDismissJob(job.id)}
                className="px-3 py-1 text-[11px] font-semibold rounded-lg bg-emerald-500 hover:bg-emerald-400 text-slate-950 transition-colors cursor-pointer"
              >
                Done
              </button>
            )}
          </div>
        </div>
      </aside>
    );
  }

  // Multi-Job View (> 1 job)
  return (
    <aside
      aria-label="Multiple Background Scrapers Status"
      className="fixed bottom-5 right-5 z-40 w-96 max-w-[calc(100vw-2.5rem)] rounded-2xl p-4 shadow-2xl backdrop-blur-xl border border-amber-500/40 bg-slate-900/95 transition-all duration-300 animate-in slide-in-from-bottom-5"
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-amber-500/20 border border-amber-500/40 flex items-center justify-center text-amber-400 shrink-0">
            {isAnyRunning ? (
              <RefreshCw className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Sparkles className="w-3.5 h-3.5 text-emerald-400" />
            )}
          </div>

          <div>
            <div className="flex items-center gap-1.5">
              <h4 className="font-bold text-xs text-slate-100">
                {runningJobs.length > 0
                  ? `${runningJobs.length} Pipelines Scraping`
                  : 'All Pipelines Completed'}
              </h4>
              <span className="text-[9px] font-mono font-semibold px-1.5 py-0.5 rounded-full bg-amber-500/20 text-amber-300">
                {relevantJobs.length} JOBS
              </span>
            </div>
            <p className="text-[11px] text-slate-400">
              Total Discovered: <strong className="text-amber-400 font-mono">{totalSightings}</strong>
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1">
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800/80 transition-colors cursor-pointer"
            title={isExpanded ? 'Collapse list' : 'Expand list'}
            aria-label={isExpanded ? 'Collapse list' : 'Expand list'}
          >
            {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronUp className="w-3.5 h-3.5" />}
          </button>
          <button
            onClick={onDismissAll}
            className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800/80 transition-colors cursor-pointer"
            title="Dismiss all"
            aria-label="Dismiss all jobs"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Expandable Job List */}
      {isExpanded && (
        <div className="mt-3 pt-2.5 border-t border-slate-800/80 space-y-2 max-h-56 overflow-y-auto pr-1">
          {relevantJobs.map((j: ScrapeJob) => {
            const isRunning = j.status === 'running' || j.status === 'pending';
            return (
              <div
                key={j.id}
                className="p-2.5 rounded-xl bg-slate-950/80 border border-slate-800/80 flex items-center justify-between text-xs"
              >
                <div className="space-y-0.5">
                  <div className="flex items-center gap-1.5 font-medium text-slate-200">
                    {isRunning ? (
                      <RefreshCw className="w-3 h-3 text-amber-400 animate-spin" />
                    ) : j.status === 'done' ? (
                      <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                    ) : (
                      <AlertCircle className="w-3 h-3 text-slate-400" />
                    )}
                    <span>{j.region || 'Coordinates'}</span>
                  </div>
                  <span className="text-[10px] text-slate-400 font-mono">
                    {j.sightings} sightings • {j.status}
                  </span>
                </div>

                <div className="flex items-center gap-1.5">
                  {isRunning && (
                    <button
                      onClick={() => cancelMutation.mutate(j.id)}
                      className="px-2 py-0.5 text-[10px] font-semibold rounded bg-rose-500/20 text-rose-300 hover:bg-rose-500/30 transition-colors cursor-pointer"
                    >
                      Cancel
                    </button>
                  )}
                  <button
                    onClick={() => onDismissJob(j.id)}
                    className="p-1 rounded text-slate-500 hover:text-slate-300 cursor-pointer"
                  >
                    <X className="w-3 h-3" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Footer */}
      <div className="mt-3 pt-2.5 border-t border-slate-800/80 flex items-center justify-between gap-2">
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="text-[11px] text-amber-400 hover:underline cursor-pointer"
        >
          {isExpanded ? 'Hide Job Details' : `Show All ${relevantJobs.length} Jobs`}
        </button>

        <button
          onClick={onOpenDetails}
          className="px-3 py-1 text-[11px] font-semibold rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/60 flex items-center gap-1 transition-colors cursor-pointer"
        >
          <span>Manage in Modal</span>
          <ExternalLink className="w-2.5 h-2.5" />
        </button>
      </div>
    </aside>
  );
}
