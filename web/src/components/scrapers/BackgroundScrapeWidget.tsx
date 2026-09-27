'use client';

import { useState } from 'react';
import { useScrapeJobs, useCancelScraper } from '@/hooks/useScrapeJobs';
import { useDiscoveryJobs, useCancelDiscovery } from '@/hooks/useDiscoveryJobs';
import {
  ScrapeTask,
  ScrapeJob,
  DiscoveryJob,
  DiscoverySourceRun,
} from '@/types';
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
  Building2,
  Briefcase,
  Compass,
  Clock,
} from 'lucide-react';
import { useLiveTimer, getJobRuntimeInfo, formatDurationMs } from '@/lib/runtime';

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

  // Discovery Jobs (Primary Python pipeline)
  const { data: discoveryJobsData } = useDiscoveryJobs();
  const cancelDiscoveryMutation = useCancelDiscovery();

  // Legacy Scrape Jobs (Go pipeline)
  const { data: legacyJobsData } = useScrapeJobs();
  const cancelLegacyMutation = useCancelScraper();

  const allDiscoveryJobs = discoveryJobsData?.jobs || [];
  const relevantDiscoveryJobs = allDiscoveryJobs.filter(
    (j) => activeJobIds.includes(j.id) || j.status === 'running' || j.status === 'pending'
  );

  const allLegacyJobs = legacyJobsData?.jobs || [];
  const relevantLegacyJobs = allLegacyJobs.filter(
    (j) => activeJobIds.includes(j.id) || j.status === 'running' || j.status === 'pending'
  );

  const runningDiscovery = relevantDiscoveryJobs.filter(
    (j) => j.status === 'running' || j.status === 'pending'
  );
  const runningLegacy = relevantLegacyJobs.filter(
    (j) => j.status === 'running' || j.status === 'pending'
  );
  const isAnyRunning = runningDiscovery.length > 0 || runningLegacy.length > 0;
  const now = useLiveTimer(isAnyRunning);

  const totalRelevantCount = relevantDiscoveryJobs.length + relevantLegacyJobs.length;
  if (totalRelevantCount === 0) return null;

  // Render single Discovery job
  if (relevantDiscoveryJobs.length === 1 && relevantLegacyJobs.length === 0) {
    const job = relevantDiscoveryJobs[0];
    const isRunning = job.status === 'running' || job.status === 'pending';
    const isDone = job.status === 'completed';
    const isPartial = job.status === 'partial';
    const isCancelled = job.status === 'cancelled';
    const runtime = getJobRuntimeInfo(job, now);

    const sourceRuns: DiscoverySourceRun[] = job.source_runs || [];
    const completedSources = sourceRuns.filter(
      (sr) =>
        sr.status === 'completed' ||
        sr.status === 'partial' ||
        sr.status === 'failed' ||
        sr.status === 'cancelled'
    ).length;
    const totalSources = Math.max(sourceRuns.length, 1);
    const progressPercent = Math.round((completedSources / totalSources) * 100);

    return (
      <aside
        aria-label="Background Discovery Status"
        className={`fixed bottom-5 right-5 z-40 w-96 max-w-[calc(100vw-2.5rem)] rounded-2xl p-4 shadow-2xl backdrop-blur-xl border transition-all duration-300 animate-in slide-in-from-bottom-5 ${
          isRunning
            ? 'bg-slate-900/95 border-amber-500/50 shadow-amber-500/10'
            : isDone
            ? 'bg-slate-900/95 border-emerald-500/50 shadow-emerald-500/10'
            : isPartial
            ? 'bg-slate-900/95 border-amber-500/40 shadow-amber-500/10'
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
            ) : isPartial ? (
              <div className="w-7 h-7 rounded-lg bg-amber-500/20 border border-amber-500/40 flex items-center justify-center text-amber-400 shrink-0">
                <AlertCircle className="w-3.5 h-3.5" />
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
                    ? 'Discovery Active'
                    : isDone
                    ? 'Discovery Completed'
                    : isPartial
                    ? 'Partial Discovery'
                    : isCancelled
                    ? 'Discovery Cancelled'
                    : 'Discovery Failed'}
                </h4>
                <span
                  className={`text-[9px] font-mono font-semibold px-1.5 py-0.5 rounded-full uppercase ${
                    isRunning
                      ? 'bg-amber-500/20 text-amber-300'
                      : isDone
                      ? 'bg-emerald-500/20 text-emerald-300'
                      : isPartial
                      ? 'bg-amber-500/20 text-amber-300'
                      : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  {job.status}
                </span>
              </div>
              <p className="text-[11px] text-slate-400">
                Target:{' '}
                <span className="font-semibold text-slate-200">
                  {job.lat.toFixed(3)}, {job.lng.toFixed(3)}
                </span>{' '}
                ({job.radius_km} km)
              </p>
              {runtime.text && (
                <p className="text-[10px] font-mono text-slate-400 flex items-center gap-1 mt-0.5">
                  <Clock className={`w-2.5 h-2.5 ${runtime.isRunning ? 'text-amber-400 animate-spin' : 'text-slate-500'}`} />
                  <span className={runtime.isRunning ? 'text-amber-300 font-medium' : 'text-slate-300'}>
                    {runtime.text}
                  </span>
                </p>
              )}
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
          {isRunning && sourceRuns.length > 0 && (
            <div>
              <div className="flex items-center justify-between text-[10px] text-slate-400 mb-1">
                <span>Sources Progress</span>
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

          {/* Counters */}
          <div className="grid grid-cols-3 gap-1.5 p-2 rounded-xl bg-slate-950/70 border border-slate-800/80 text-center">
            <div className="space-y-0.5">
              <span className="text-[10px] text-slate-400 block flex items-center justify-center gap-1">
                <Building2 className="w-2.5 h-2.5 text-amber-400" />
                Companies
              </span>
              <span className="font-mono font-bold text-amber-400 text-xs">
                {job.company_count}
              </span>
            </div>
            <div className="space-y-0.5 border-x border-slate-800/60">
              <span className="text-[10px] text-slate-400 block flex items-center justify-center gap-1">
                <Briefcase className="w-2.5 h-2.5 text-indigo-400" />
                Jobs
              </span>
              <span className="font-mono font-bold text-indigo-400 text-xs">
                {job.job_count}
              </span>
            </div>
            <div className="space-y-0.5">
              <span className="text-[10px] text-slate-400 block flex items-center justify-center gap-1">
                <Compass className="w-2.5 h-2.5 text-slate-400" />
                Evidence
              </span>
              <span className="font-mono font-bold text-slate-200 text-xs">
                {job.evidence_count}
              </span>
            </div>
          </div>

          {/* Source runs progress */}
          {sourceRuns.length > 0 && (
            <div className="flex items-center gap-1.5 flex-wrap pt-1">
              {sourceRuns.map((sr) => (
                <span
                  key={sr.id}
                  className="text-[10px] font-mono px-2 py-0.5 rounded-md bg-slate-950 border border-slate-800 text-slate-300 flex items-center gap-1"
                >
                  {sr.status === 'running' ? (
                    <RefreshCw className="w-2.5 h-2.5 animate-spin text-amber-400" />
                  ) : sr.status === 'completed' ? (
                    <CheckCircle2 className="w-2.5 h-2.5 text-emerald-400" />
                  ) : sr.status === 'partial' ? (
                    <AlertCircle className="w-2.5 h-2.5 text-amber-400" />
                  ) : (
                    <AlertCircle className="w-2.5 h-2.5 text-slate-500" />
                  )}
                  <span className="uppercase font-semibold">{sr.source}</span>
                  <span className="text-amber-400">({sr.company_count}c/{sr.job_count}j)</span>
                </span>
              ))}
            </div>
          )}
        </div>

        <div className="mt-3 pt-2.5 border-t border-slate-800/80 flex items-center justify-between gap-2">
          {isRunning ? (
            <button
              onClick={() => cancelDiscoveryMutation.mutate(job.id)}
              disabled={cancelDiscoveryMutation.isPending}
              className="px-2.5 py-1 text-[11px] font-semibold rounded-lg bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30 flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <StopCircle className="w-3 h-3" />
              <span>Cancel</span>
            </button>
          ) : (
            <span className="text-[10px] text-slate-400">
              {isDone ? 'Map updated' : isPartial ? 'Partial results' : 'Finished'}
            </span>
          )}

          <div className="flex items-center gap-2">
            <button
              onClick={onOpenDetails}
              className="px-3 py-1 text-[11px] font-semibold rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/60 flex items-center gap-1 transition-colors cursor-pointer"
            >
              <span>Manage</span>
              <ExternalLink className="w-2.5 h-2.5" />
            </button>

            {(isDone || isPartial) && (
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

  // Render single legacy Go scrape job
  if (relevantDiscoveryJobs.length === 0 && relevantLegacyJobs.length === 1) {
    const job = relevantLegacyJobs[0];
    const isRunning = job.status === 'running' || job.status === 'pending';
    const isDone = job.status === 'done';
    const isCancelled = job.status === 'cancelled';
    const legacyRuntime = getJobRuntimeInfo(job, now);
    const tasks: ScrapeTask[] = job.tasks || [];

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
                  {isRunning ? 'Scraping Active' : isDone ? 'Scraping Completed' : 'Scraping Finished'}
                </h4>
                <span className="text-[9px] font-mono font-semibold px-1.5 py-0.5 rounded-full bg-slate-800 text-slate-300 uppercase">
                  {job.status}
                </span>
              </div>
              <p className="text-[11px] text-slate-400">
                Hub: <span className="font-semibold text-slate-200">{job.region || 'Coordinates'}</span>
              </p>
              {legacyRuntime.text && (
                <p className="text-[10px] font-mono text-slate-400 flex items-center gap-1 mt-0.5">
                  <Clock className={`w-2.5 h-2.5 ${legacyRuntime.isRunning ? 'text-amber-400 animate-spin' : 'text-slate-500'}`} />
                  <span className={legacyRuntime.isRunning ? 'text-amber-300 font-medium' : 'text-slate-300'}>
                    {legacyRuntime.text}
                  </span>
                </p>
              )}
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
              onClick={() => cancelLegacyMutation.mutate(job.id)}
              disabled={cancelLegacyMutation.isPending}
              className="px-2.5 py-1 text-[11px] font-semibold rounded-lg bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30 flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <StopCircle className="w-3 h-3" />
              <span>Cancel</span>
            </button>
          ) : (
            <span className="text-[10px] text-slate-400">Finished</span>
          )}

          <div className="flex items-center gap-2">
            <button
              onClick={onOpenDetails}
              className="px-3 py-1 text-[11px] font-semibold rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700/60 flex items-center gap-1 transition-colors cursor-pointer"
            >
              <span>Manage</span>
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

  // Multi-Job View (> 1 job across discovery and legacy)
  const totalCompanies = relevantDiscoveryJobs.reduce((acc, j) => acc + (j.company_count || 0), 0);
  const totalJobs = relevantDiscoveryJobs.reduce((acc, j) => acc + (j.job_count || 0), 0);
  const totalSightings = relevantLegacyJobs.reduce((acc, j) => acc + (j.sightings || 0), 0);

  return (
    <aside
      aria-label="Multiple Background Pipelines Status"
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
                {isAnyRunning
                  ? `${runningDiscovery.length + runningLegacy.length} Pipelines Active`
                  : 'All Pipelines Completed'}
              </h4>
              <span className="text-[9px] font-mono font-semibold px-1.5 py-0.5 rounded-full bg-amber-500/20 text-amber-300">
                {totalRelevantCount} JOBS
              </span>
            </div>
            <p className="text-[11px] text-slate-400">
              Found: <strong className="text-amber-400 font-mono">{totalCompanies}</strong> co,{' '}
              <strong className="text-indigo-400 font-mono">{totalJobs}</strong> jobs
              {totalSightings > 0 && (
                <span>, <strong className="text-slate-300 font-mono">{totalSightings}</strong> legacy</span>
              )}
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
          {/* Discovery Jobs */}
          {relevantDiscoveryJobs.map((j: DiscoveryJob) => {
            const isRunning = j.status === 'running' || j.status === 'pending';
            const runtime = getJobRuntimeInfo(j, now);
            return (
              <div
                key={j.id}
                className="p-2.5 rounded-xl bg-slate-950/80 border border-slate-800/80 flex items-center justify-between text-xs"
              >
                <div className="space-y-0.5">
                  <div className="flex items-center gap-1.5 font-medium text-slate-200">
                    {isRunning ? (
                      <RefreshCw className="w-3 h-3 text-amber-400 animate-spin" />
                    ) : j.status === 'completed' ? (
                      <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                    ) : (
                      <AlertCircle className="w-3 h-3 text-amber-400" />
                    )}
                    <span>
                      Discovery ({j.lat.toFixed(2)}, {j.lng.toFixed(2)})
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 text-[10px] text-slate-400 font-mono">
                    <span>
                      {j.company_count} co • {j.job_count} jobs
                    </span>
                    {runtime.text ? (
                      <>
                        <span>•</span>
                        <span className={`flex items-center gap-0.5 ${runtime.isRunning ? 'text-amber-300 font-medium' : 'text-slate-400'}`}>
                          <Clock className={`w-2.5 h-2.5 ${runtime.isRunning ? 'text-amber-400 animate-spin' : 'text-slate-500'}`} />
                          <span>{runtime.text}</span>
                        </span>
                      </>
                    ) : (
                      <span>• {j.status}</span>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-1.5">
                  {isRunning && (
                    <button
                      onClick={() => cancelDiscoveryMutation.mutate(j.id)}
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

          {/* Legacy Go Scrape Jobs */}
          {relevantLegacyJobs.map((j: ScrapeJob) => {
            const isRunning = j.status === 'running' || j.status === 'pending';
            const legacyRuntime = getJobRuntimeInfo(j, now);
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
                    <span>Legacy {j.region || 'Coordinates'}</span>
                  </div>
                  <div className="flex items-center gap-1.5 text-[10px] text-slate-400 font-mono">
                    <span>{j.sightings} sightings</span>
                    {legacyRuntime.text ? (
                      <>
                        <span>•</span>
                        <span className={`flex items-center gap-0.5 ${legacyRuntime.isRunning ? 'text-amber-300 font-medium' : 'text-slate-400'}`}>
                          <Clock className={`w-2.5 h-2.5 ${legacyRuntime.isRunning ? 'text-amber-400 animate-spin' : 'text-slate-500'}`} />
                          <span>{legacyRuntime.text}</span>
                        </span>
                      </>
                    ) : (
                      <span>• {j.status}</span>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-1.5">
                  {isRunning && (
                    <button
                      onClick={() => cancelLegacyMutation.mutate(j.id)}
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
          {isExpanded ? 'Hide Details' : `Show All ${totalRelevantCount} Pipelines`}
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
