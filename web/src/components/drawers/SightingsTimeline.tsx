import { Sighting } from '@/types';
import { ExternalLink, CheckCircle2, Globe, Database, Building } from 'lucide-react';

interface SightingsTimelineProps {
  sightings: Sighting[];
}

export default function SightingsTimeline({ sightings }: SightingsTimelineProps) {
  if (!sightings || sightings.length === 0) {
    return (
      <div className="py-8 text-center text-xs text-slate-400">
        No external sightings recorded for this company.
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

  return (
    <div className="space-y-3">
      {sightings.map((s, idx) => (
        <div key={s.id || idx} className="p-3 rounded-xl bg-slate-950/70 border border-slate-800/80 space-y-1.5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-200 flex items-center gap-1.5">
              {getSourceIcon(s.source)}
              <span className="uppercase tracking-wider">{s.source}</span>
            </span>
            <span className="text-[10px] font-mono text-slate-400">
              {s.scraped_at ? new Date(s.scraped_at).toLocaleDateString() : 'Recent'}
            </span>
          </div>

          <p className="text-[11px] text-slate-300">{s.raw_address || s.company_name}</p>

          <div className="flex items-center justify-between pt-1 border-t border-slate-800/50 text-[10px]">
            <span className="font-mono text-slate-400">{s.lat?.toFixed(4)}, {s.lng?.toFixed(4)}</span>
            {s.source_url && (
              <a
                href={s.source_url}
                target="_blank"
                rel="noreferrer"
                className="text-amber-400 hover:text-amber-300 flex items-center gap-1"
              >
                <span>Source</span>
                <ExternalLink className="w-2.5 h-2.5" />
              </a>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
