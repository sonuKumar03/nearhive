import { CompanySearchResult, TechnicalJobSearchResult } from '@/types';
import CompanyCard from './CompanyCard';
import JobCard from './JobCard';
import { Search, Building2, Briefcase, X, SearchX } from 'lucide-react';

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
  selectedCompany?: CompanySearchResult | null;
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
  selectedCompany,
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
      <div className="p-3.5 border-b border-slate-800/80 bg-slate-900/60 space-y-2.5 shrink-0">
        {/* Two-tab segmented mode toggle */}
        <div
          className="flex items-center gap-1 p-1 bg-slate-950/80 rounded-xl border border-slate-800/90 shadow-inner"
          role="tablist"
          aria-label="Search results mode"
        >
          <button
            role="tab"
            aria-selected={!isJobs}
            onClick={() => onModeChange?.('companies')}
            className={`flex-1 h-8 px-3 text-xs rounded-lg transition-colors flex items-center justify-center gap-1.5 cursor-pointer font-medium border box-border ${
              !isJobs
                ? 'bg-slate-800 text-amber-400 font-semibold shadow-sm border-slate-700/60'
                : 'text-slate-400 border-transparent hover:text-slate-200 hover:bg-slate-900/40'
            }`}
          >
            <Building2 className="w-3.5 h-3.5 shrink-0" />
            <span className="truncate">Offices</span>
            <span
              className={`text-[10px] font-mono px-1.5 py-0.5 rounded-full border transition-colors shrink-0 ${
                !isJobs
                  ? 'bg-amber-400/15 text-amber-300 border-amber-400/30'
                  : 'bg-slate-900 text-slate-500 border-slate-800'
              }`}
            >
              {totalCount}
            </span>
          </button>
          <button
            role="tab"
            aria-selected={isJobs}
            onClick={() => onModeChange?.('jobs')}
            className={`flex-1 h-8 px-3 text-xs rounded-lg transition-colors flex items-center justify-center gap-1.5 cursor-pointer font-medium border box-border ${
              isJobs
                ? 'bg-slate-800 text-amber-400 font-semibold shadow-sm border-slate-700/60'
                : 'text-slate-400 border-transparent hover:text-slate-200 hover:bg-slate-900/40'
            }`}
          >
            <Briefcase className="w-3.5 h-3.5 shrink-0" />
            <span className="truncate">Nearby Jobs</span>
            <span
              className={`text-[10px] font-mono px-1.5 py-0.5 rounded-full border transition-colors shrink-0 ${
                isJobs
                  ? 'bg-amber-400/15 text-amber-300 border-amber-400/30'
                  : 'bg-slate-900 text-slate-500 border-slate-800'
              }`}
            >
              {effectiveJobsCount}
            </span>
          </button>
        </div>

        {/* Filter Input */}
        <div className="relative flex items-center">
          <Search className="w-3.5 h-3.5 text-slate-400 absolute left-3 pointer-events-none" />
          <input
            id="company-search-filter"
            aria-label={
              isJobs
                ? 'Filter nearby jobs by title or company'
                : 'Filter companies by name, industry, or tech park'
            }
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={
              isJobs ? 'Filter jobs by title or company...' : 'Filter by company, industry, or park...'
            }
            className="w-full bg-slate-950/80 border border-slate-800/90 hover:border-slate-700/80 focus:border-amber-500/60 rounded-xl py-2 pl-9 pr-8 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-1 focus:ring-amber-500/30 transition-all shadow-inner"
          />
          {searchQuery && (
            <button
              type="button"
              onClick={() => onSearchChange('')}
              className="absolute right-2.5 p-1 rounded-md text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
              aria-label="Clear filter"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Dual Tab Panels with preserved scroll states & zero jump */}
      <div className="flex-1 relative min-h-0 overflow-hidden">
        {/* Offices Panel */}
        <div
          className={`absolute inset-0 overflow-y-auto px-3.5 py-3 space-y-2.5 [scrollbar-gutter:stable] ${
            !isJobs ? 'block' : 'hidden'
          }`}
        >
          {isLoading ? (
            <div className="py-16 text-center space-y-3">
              <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin mx-auto" />
              <p className="text-xs text-slate-400 font-medium">Searching tech offices within radius...</p>
            </div>
          ) : companies.length === 0 ? (
            <div className="py-14 px-6 text-center space-y-3">
              <div className="w-12 h-12 rounded-2xl bg-slate-800/70 border border-slate-700/50 flex items-center justify-center mx-auto shadow-inner">
                <SearchX className="w-6 h-6 text-slate-400" />
              </div>
              <div className="space-y-1">
                <h4 className="font-semibold text-xs text-slate-200">No Tech Offices in this Radius</h4>
                <p className="text-[11px] text-slate-400 leading-relaxed max-w-[260px] mx-auto">
                  Expand the radius slider or run our automated discovery to crawl tech companies in this hub.
                </p>
              </div>
            </div>
          ) : (
            companies.map((c) => (
              <CompanyCard
                key={c.id + c.location_id}
                company={c}
                onClick={() => onSelectCompany(c)}
                isSelected={
                  selectedCompany?.id === c.id && selectedCompany?.location_id === c.location_id
                }
              />
            ))
          )}
        </div>

        {/* Nearby Jobs Panel */}
        <div
          className={`absolute inset-0 overflow-y-auto px-3.5 py-3 space-y-2.5 [scrollbar-gutter:stable] ${
            isJobs ? 'block' : 'hidden'
          }`}
        >
          {isLoadingJobs ? (
            <div className="py-16 text-center space-y-3">
              <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin mx-auto" />
              <p className="text-xs text-slate-400 font-medium">Searching nearby jobs within radius...</p>
            </div>
          ) : jobs.length === 0 ? (
            <div className="py-14 px-6 text-center space-y-3">
              <div className="w-12 h-12 rounded-2xl bg-slate-800/70 border border-slate-700/50 flex items-center justify-center mx-auto shadow-inner">
                <Briefcase className="w-6 h-6 text-slate-400" />
              </div>
              <div className="space-y-1">
                <h4 className="font-semibold text-xs text-slate-200">No Technical Jobs in this Radius</h4>
                <p className="text-[11px] text-slate-400 leading-relaxed max-w-[260px] mx-auto">
                  Expand the radius slider or run automated discovery to crawl verified company career boards.
                </p>
              </div>
            </div>
          ) : (
            jobs.map((j) => <JobCard key={j.id} job={j} />)
          )}
        </div>
      </div>
    </aside>
  );
}
