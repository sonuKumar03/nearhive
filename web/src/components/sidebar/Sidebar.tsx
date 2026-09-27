import { CompanySearchResult, TechnicalJobSearchResult } from '@/types';
import CompanyCard from './CompanyCard';
import JobCard from './JobCard';
import { Search, Building2, Briefcase } from 'lucide-react';

export type SidebarMode = 'companies' | 'jobs';

interface SidebarProps {
  companies: CompanySearchResult[];
  isLoading: boolean;
  totalCount: number;
  jobs?: TechnicalJobSearchResult[];
  isLoadingJobs?: boolean;
  totalJobsCount?: number;
  activeMode?: SidebarMode;
  onModeChange?: (mode: SidebarMode) => void;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onSelectCompany: (company: CompanySearchResult) => void;
  className?: string;
}

export default function Sidebar({
  companies,
  isLoading,
  totalCount,
  jobs = [],
  isLoadingJobs = false,
  totalJobsCount,
  activeMode = 'companies',
  onModeChange,
  searchQuery,
  onSearchChange,
  onSelectCompany,
  className = '',
}: SidebarProps) {
  const isJobs = activeMode === 'jobs';
  const effectiveJobsCount = totalJobsCount ?? jobs.length;

  return (
    <aside
      aria-label="Discovered tech companies and jobs list"
      className={`w-full md:w-[420px] bg-slate-900/95 border-r border-slate-800/80 backdrop-blur-md flex flex-col z-10 shrink-0 h-full ${className}`}
    >
      {/* Search & Mode Header */}
      <div className="p-3.5 border-b border-slate-800/80 space-y-2.5">
        {/* Two-tab mode toggle */}
        <div className="flex items-center gap-1 p-1 bg-slate-950/80 rounded-xl border border-slate-800" role="tablist" aria-label="Search results mode">
          <button
            role="tab"
            aria-selected={!isJobs}
            onClick={() => onModeChange?.('companies')}
            className={`flex-1 py-1.5 px-3 text-xs rounded-lg transition-colors flex items-center justify-center gap-1.5 cursor-pointer ${
              !isJobs
                ? 'bg-slate-800 text-amber-400 font-semibold shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Building2 className="w-3.5 h-3.5" />
            <span>Offices</span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded-full bg-slate-900 border border-slate-700">
              {totalCount}
            </span>
          </button>
          <button
            role="tab"
            aria-selected={isJobs}
            onClick={() => onModeChange?.('jobs')}
            className={`flex-1 py-1.5 px-3 text-xs rounded-lg transition-colors flex items-center justify-center gap-1.5 cursor-pointer ${
              isJobs
                ? 'bg-slate-800 text-amber-400 font-semibold shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Briefcase className="w-3.5 h-3.5" />
            <span>Nearby Jobs</span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded-full bg-slate-900 border border-slate-700">
              {effectiveJobsCount}
            </span>
          </button>
        </div>

        {/* Filter Input */}
        <div className="relative">
          <input
            id="company-search-filter"
            aria-label={isJobs ? "Filter nearby jobs by title or company" : "Filter companies by name, industry, or tech park"}
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={isJobs ? "Filter jobs by title or company..." : "Filter by company, industry, or tech park..."}
            className="w-full bg-slate-950/80 border border-slate-800 rounded-xl px-3 py-1.5 pl-8 text-xs text-slate-200 placeholder-slate-400 focus:outline-none focus:border-amber-500/50 transition-colors"
          />
          <Search className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-2.5" />
        </div>
      </div>

      {/* Cards List Container */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2.5 divide-y divide-slate-800/40">
        {!isJobs ? (
          isLoading ? (
            <div className="py-16 text-center space-y-3">
              <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin mx-auto" />
              <p className="text-xs text-slate-400 font-medium">Searching tech offices within radius...</p>
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
          )
        ) : (
          isLoadingJobs ? (
            <div className="py-16 text-center space-y-3">
              <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin mx-auto" />
              <p className="text-xs text-slate-400 font-medium">Searching nearby jobs within radius...</p>
            </div>
          ) : jobs.length === 0 ? (
            <div className="py-14 px-6 text-center space-y-3">
              <div className="w-12 h-12 rounded-2xl bg-slate-800/60 flex items-center justify-center text-2xl mx-auto border border-slate-700/50">
                💼
              </div>
              <h4 className="font-semibold text-xs text-slate-200">No Technical Jobs in this Radius</h4>
              <p className="text-[11px] text-slate-400 leading-relaxed">
                Expand the radius slider or run automated discovery to crawl verified company career boards.
              </p>
            </div>
          ) : (
            jobs.map((j) => (
              <JobCard key={j.id} job={j} />
            ))
          )
        )}
      </div>
    </aside>
  );
}
