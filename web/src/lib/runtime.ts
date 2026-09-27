import { useEffect, useState } from 'react';

/**
 * Format total seconds into a readable string (e.g. "45s", "2m 14s", "1h 12m 5s").
 */
export function formatDurationSeconds(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds < 0) return '0s';
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = Math.floor(totalSeconds % 60);

  if (hours > 0) {
    return `${hours}h ${minutes}m ${seconds}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }
  return `${seconds}s`;
}

/**
 * Format milliseconds into a readable string (e.g. 1500 -> "2s", 108879 -> "1m 49s").
 */
export function formatDurationMs(ms: number | undefined | null): string {
  if (!ms || ms <= 0) return '0s';
  return formatDurationSeconds(Math.round(ms / 1000));
}

export interface JobRuntimeInfo {
  text: string;
  shortText: string;
  elapsedSeconds: number;
  isRunning: boolean;
  isPending: boolean;
  isCompleted: boolean;
}

/**
 * Extract runtime info for a background job directly from backend-computed fields.
 */
export function getJobRuntimeInfo(
  job: {
    status: string;
    duration_text?: string;
    duration_ms?: number;
    elapsed_seconds?: number;
    started_at?: string;
    finished_at?: string;
    created_at?: string;
  },
  now?: number
): JobRuntimeInfo {
  const isRunning = job.status === 'running';
  const isPending = job.status === 'pending';
  const isCompleted = job.status === 'completed' || job.status === 'done';

  // Live client-side ticking for running/pending states if `now` is provided
  if (isRunning && typeof now === 'number' && now > 0) {
    const startTime = job.started_at
      ? new Date(job.started_at).getTime()
      : job.created_at
      ? new Date(job.created_at).getTime()
      : 0;
    if (startTime > 0) {
      const elapsedSeconds = Math.max(0, Math.floor((now - startTime) / 1000));
      const duration = formatDurationSeconds(elapsedSeconds);
      return {
        text: `Running for ${duration}`,
        shortText: duration,
        elapsedSeconds,
        isRunning: true,
        isPending: false,
        isCompleted: false,
      };
    }
  }

  if (isPending && typeof now === 'number' && now > 0) {
    const startTime = job.created_at ? new Date(job.created_at).getTime() : 0;
    if (startTime > 0) {
      const elapsedSeconds = Math.max(0, Math.floor((now - startTime) / 1000));
      const duration = formatDurationSeconds(elapsedSeconds);
      return {
        text: `Queued for ${duration}`,
        shortText: `queued ${duration}`,
        elapsedSeconds,
        isRunning: false,
        isPending: true,
        isCompleted: false,
      };
    }
  }

  // Backend authoritative text and elapsed time
  if (job.duration_text) {
    const shortText = job.duration_text.replace(
      /^(Running for|Queued for|Took|Failed after|Cancelled after|Partial after)\s+/,
      ''
    );
    return {
      text: job.duration_text,
      shortText,
      elapsedSeconds:
        job.elapsed_seconds ?? (job.duration_ms ? Math.round(job.duration_ms / 1000) : 0),
      isRunning,
      isPending,
      isCompleted,
    };
  }

  // Fallback to backend duration_ms if duration_text is omitted
  if (typeof job.duration_ms === 'number' && job.duration_ms >= 0) {
    const duration = formatDurationMs(job.duration_ms);
    let text = duration;
    if (isRunning) text = `Running for ${duration}`;
    else if (isPending) text = `Queued for ${duration}`;
    else if (isCompleted) text = `Took ${duration}`;
    else if (job.status === 'failed') text = `Failed after ${duration}`;
    else if (job.status === 'cancelled') text = `Cancelled after ${duration}`;

    return {
      text,
      shortText: duration,
      elapsedSeconds: Math.round(job.duration_ms / 1000),
      isRunning,
      isPending,
      isCompleted,
    };
  }

  return {
    text: '',
    shortText: '',
    elapsedSeconds: 0,
    isRunning,
    isPending,
    isCompleted,
  };
}

/**
 * React hook that yields current timestamp (Date.now()) ticking every intervalMs
 * only when enabled (e.g. when any active job is present).
 */
export function useLiveTimer(enabled: boolean = true, intervalMs: number = 1000): number {
  const [now, setNow] = useState<number>(() => Date.now());

  useEffect(() => {
    if (!enabled) return;
    const timer = setInterval(() => {
      setNow(Date.now());
    }, intervalMs);
    return () => clearInterval(timer);
  }, [enabled, intervalMs]);

  return now;
}
