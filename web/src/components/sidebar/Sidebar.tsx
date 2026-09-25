import { CompanySearchResult } from '@/types';
import CompanyCard from './CompanyCard';
import { Search } from 'lucide-react';

interface SidebarProps {
  companies: CompanySearchResult[];
  isLoading: boolean;
  totalCount: number;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onSelectCompany: (company: CompanySearchResult) => void;
}

export default function Sidebar({
  companies,
  isLoading,
  totalCount,
  searchQuery,
  onSearchChange,
  onSelectCompany,
}: SidebarProps) {
  return (
    <aside className="w-full sm:w-96 md:w-[420px] bg-slate-900/95 border-r border-slate-800/80 backdrop-blur-md flex flex-col z-10 shrink-0 h-full">
      {/* Search Header */}
      <div className="p-3.5 border-b border-slate-800/80 space-y-2.5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm text-slate-200">Discovered Tech Offices</span>
            <span className="text-xs font-mono font-semibold px-2 py-0.5 rounded-full bg-slate-800 text-amber-400 border border-slate-700">
              {totalCount}
            </span>
          </div>
        </div>

        {/* Filter Input */}
        <div className="relative">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Filter by company, industry, or tech park..."
            className="w-full bg-slate-950/80 border border-slate-800 rounded-xl px-3 py-1.5 pl-8 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-amber-500/50 transition-colors"
          />
          <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-2.5" />
        </div>
      </div>

      {/* Cards List Container */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2.5 divide-y divide-slate-800/40">
        {isLoading ? (
          <div className="py-16 text-center space-y-3">
            <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin mx-auto" />
            <p className="text-xs text-slate-400 font-medium">Querying PostGIS ST_DWithin index...</p>
          </div>
        ) : companies.length === 0 ? (
          <div className="py-14 px-6 text-center space-y-3">
            <div className="w-12 h-12 rounded-2xl bg-slate-800/60 flex items-center justify-center text-2xl mx-auto border border-slate-700/50">
              🔍
            </div>
            <h4 className="font-semibold text-xs text-slate-200">No Tech Offices in this Radius</h4>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              Expand the radius slider or run our automated scraper to discover tech companies in this hub.
            </p>
          </div>
        ) : (
          companies.map((c) => (
            <CompanyCard key={c.id + c.location_id} company={c} onClick={() => onSelectCompany(c)} />
          ))
        )}
      </div>
    </aside>
  );
}
