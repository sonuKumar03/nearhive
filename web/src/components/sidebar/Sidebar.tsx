import { CompanySearchResult, TechnicalJobSearchResult } from '@/types';
import CompanyCard from './CompanyCard';
import JobCard from './JobCard';
import { SegmentedControl, SearchInput, EmptyState } from '@/components/ui';
import { Building2, Briefcase, SearchX } from 'lucide-react';

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
      className={`w-full md:w-[420px] bg-[#0d0d11]/95 border-r border-white/[0.08] backdrop-blur-md flex flex-col z-10 shrink-0 h-full ${className}`}
    >
      {/* Search & Mode Header */}
      <div className="p-3.5 border-b border-white/[0.08] bg-[#131318]/70 space-y-2.5 shrink-0">
        {/* Two-tab segmented mode toggle using Design System primitive */}
        <SegmentedControl
          value={activeMode}
          onChange={(val) => onModeChange?.(val as SidebarMode)}
          options={[
            {
              value: 'companies',
              label: 'Offices',
              icon: <Building2 className="w-3.5 h-3.5" />,
              count: totalCount,
            },
            {
              value: 'jobs',
              label: 'Nearby Jobs',
              icon: <Briefcase className="w-3.5 h-3.5" />,
              count: effectiveJobsCount,
            },
          ]}
        />

        {/* Filter Input using SearchInput primitive */}
        <SearchInput
          id="company-search-filter"
          aria-label={
            isJobs
              ? 'Filter nearby jobs by title or company'
              : 'Filter companies by name, industry, or tech park'
          }
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          onClear={() => onSearchChange('')}
          placeholder={
            isJobs ? 'Filter jobs by title or company...' : 'Filter by company, industry, or park...'
          }
        />
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
            <EmptyState
              icon={<SearchX className="w-6 h-6 text-slate-400" />}
              title="No Tech Offices in this Radius"
              description="Expand the radius slider or run our automated discovery to crawl tech companies in this hub."
            />
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
            <EmptyState
              icon={<Briefcase className="w-6 h-6 text-slate-400" />}
              title="No Technical Jobs in this Radius"
              description="Expand the radius slider or run automated discovery to crawl verified company career boards."
            />
          ) : (
            jobs.map((j) => <JobCard key={j.id} job={j} />)
          )}
        </div>
      </div>
    </aside>
  );
}
