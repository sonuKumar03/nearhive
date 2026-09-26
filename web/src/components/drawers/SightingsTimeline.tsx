import { Sighting } from '@/types';
import { ExternalLink, CheckCircle2, Globe, Database, Building, ChevronDown } from 'lucide-react';

interface SightingsTimelineProps {
  sightings: Sighting[];
}

export default function SightingsTimeline({ sightings }: SightingsTimelineProps) {
  if (!sightings || sightings.length === 0) {
    return (
      <div className="py-3 px-3 rounded-lg bg-slate-950/60 border border-slate-800 text-xs text-slate-400 text-center">
        No external sightings recorded for this office.
      </div>
    );
  }

  const getSourceIcon = (source: string) => {
    switch (source?.toLowerCase()) {
      case 'osm':
        return <Globe className="w-3.5 h-3.5 text-emerald-400" />;
      case 'wikidata':
        return <Database className="w-3.5 h-3.5 text-amber-400" />;
      case 'techparks':
        return <Building className="w-3.5 h-3.5 text-blue-400" />;
      default:
        return <CheckCircle2 className="w-3.5 h-3.5 text-slate-400" />;
    }
  };

  const getSourceLabel = (source: string) => {
    switch (source?.toLowerCase()) {
      case 'osm':
        return 'OpenStreetMap';
      case 'wikidata':
        return 'Wikidata Registry';
      case 'techparks':
        return 'Tech Parks Directory';
      default:
        return source.toUpperCase();
    }
  };

  // Group unique sources
  const uniqueSources = Array.from(new Set(sightings.map((s) => s.source.toLowerCase())));

  return (
    <div className="space-y-2.5">
      {/* Clean Verified Sources Row */}
      <div className="flex flex-wrap gap-2">
        {uniqueSources.map((source) => {
          const firstWithUrl = sightings.find((s) => s.source.toLowerCase() === source && s.source_url);
          return (
            <div
              key={source}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-200"
            >
              {getSourceIcon(source)}
              <span className="font-medium text-[11px]">{getSourceLabel(source)}</span>
              {firstWithUrl?.source_url && (
                <a
                  href={firstWithUrl.source_url}
                  target="_blank"
                  rel="noreferrer"
                  title="View Registry Entry"
                  className="text-amber-400 hover:text-amber-300 ml-0.5"
                >
                  <ExternalLink className="w-3 h-3" />
                </a>
              )}
            </div>
          );
        })}
      </div>

      {/* Collapsible Raw Audit Logs (collapsed by default so UI stays clean) */}
      <details className="group rounded-xl border border-slate-800/80 bg-slate-950/40 overflow-hidden text-xs">
        <summary className="px-3 py-2 text-[11px] text-slate-400 font-medium flex items-center justify-between cursor-pointer hover:text-slate-300 hover:bg-slate-900/40 transition-colors list-none">
          <span className="flex items-center gap-1.5">
            <span className="w-1.5 h-1.5 rounded-full bg-slate-500" />
            Raw Crawler Audit Logs ({sightings.length})
          </span>
          <ChevronDown className="w-3.5 h-3.5 transition-transform group-open:rotate-180 text-slate-400" />
        </summary>
        <div className="p-2.5 space-y-2 border-t border-slate-800/60 bg-slate-950/60 max-h-48 overflow-y-auto">
          {sightings.map((s, idx) => (
            <div key={s.id || idx} className="p-2 rounded-lg bg-slate-900/60 border border-slate-800/60 space-y-1">
              <div className="flex items-center justify-between">
                <span className="text-[10px] font-semibold text-slate-300 uppercase tracking-wider">{s.source}</span>
                <span className="text-[9px] font-mono text-slate-400">
                  {s.scraped_at ? new Date(s.scraped_at).toLocaleDateString() : 'Recent'}
                </span>
              </div>
              <p className="text-[10px] text-slate-400 truncate">{s.raw_address || s.company_name}</p>
              <div className="flex items-center justify-between text-[9px] font-mono text-slate-400">
                <span>{s.lat?.toFixed(4)}, {s.lng?.toFixed(4)}</span>
                {s.source_url && (
                  <a href={s.source_url} target="_blank" rel="noreferrer" className="text-amber-400 hover:underline">
                    View Link
                  </a>
                )}
              </div>
            </div>
          ))}
        </div>
      </details>
    </div>
  );
}
