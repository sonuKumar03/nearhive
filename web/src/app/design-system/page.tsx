'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import {
  Button,
  Badge,
  SegmentedControl,
  Input,
  SearchInput,
  Select,
  Slider,
  Card,
  EmptyState,
  Modal,
} from '@/components/ui';
import {
  Building2,
  Briefcase,
  Search,
  MapPin,
  Sparkles,
  ArrowLeft,
  Radar,
  CheckCircle2,
  AlertTriangle,
  ExternalLink,
} from 'lucide-react';

export default function DesignSystemShowcasePage() {
  const [segmentedTab, setSegmentedTab] = useState<'offices' | 'jobs'>('offices');
  const [searchValue, setSearchValue] = useState('Google');
  const [inputValue, setInputValue] = useState('Hyderabad, Telangana');
  const [inputError, setInputError] = useState(false);
  const [selectedCity, setSelectedCity] = useState('hyderabad');
  const [radiusKm, setRadiusKm] = useState(15);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [buttonLoading, setButtonLoading] = useState(false);

  const coverageArea = Math.round(Math.PI * radiusKm * radiusKm);

  return (
    <div className="h-screen overflow-y-auto bg-[#09090b] text-zinc-100 p-6 md:p-12 font-sans selection:bg-indigo-500/30 selection:text-indigo-200 [scrollbar-gutter:stable]">
      <div className="max-w-5xl mx-auto space-y-12">
        {/* Header */}
        <header className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-8 border-b border-white/[0.08]">
          <div>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-indigo-500/10 border border-indigo-500/30 text-indigo-400">
                <Sparkles className="w-5 h-5" />
              </div>
              <h1 className="text-2xl font-bold tracking-tight text-white">
                NearHive Design System
              </h1>
            </div>
            <p className="text-sm text-zinc-400 mt-1 max-w-xl">
              Foundational tokens, accessible UI component primitives, and interactive playground in modern Obsidian & Electric Indigo.
            </p>
          </div>

          <Link href="/">
            <Button variant="secondary" size="sm" leftIcon={<ArrowLeft className="w-4 h-4" />}>
              Return to Map
            </Button>
          </Link>
        </header>

        {/* Section 1: Color Tokens */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">
            1. Theme & Semantic Tokens (Role-Based Variables)
          </h2>
          <div className="grid grid-cols-2 sm:grid-cols-4 md:grid-cols-7 gap-3">
            <div className="rounded-xl border border-border-default bg-surface-base p-3 space-y-1">
              <div className="h-10 rounded-lg bg-surface-card border border-border-default" />
              <div className="text-xs font-semibold text-zinc-200">Surface Card</div>
              <div className="text-[10px] font-mono text-zinc-500">--color-surface-card</div>
            </div>
            <div className="rounded-xl border border-border-default bg-surface-base p-3 space-y-1">
              <div className="h-10 rounded-lg bg-primary shadow-md shadow-primary/25" />
              <div className="text-xs font-semibold text-primary-text">Primary</div>
              <div className="text-[10px] font-mono text-zinc-500">--color-primary</div>
            </div>
            <div className="rounded-xl border border-border-default bg-surface-base p-3 space-y-1">
              <div className="h-10 rounded-lg bg-secondary border border-border-default" />
              <div className="text-xs font-semibold text-zinc-300">Secondary</div>
              <div className="text-[10px] font-mono text-zinc-500">--color-secondary</div>
            </div>
            <div className="rounded-xl border border-border-default bg-surface-base p-3 space-y-1">
              <div className="h-10 rounded-lg bg-success shadow-md shadow-success/25" />
              <div className="text-xs font-semibold text-success-text">Success</div>
              <div className="text-[10px] font-mono text-zinc-500">--color-success</div>
            </div>
            <div className="rounded-xl border border-border-default bg-surface-base p-3 space-y-1">
              <div className="h-10 rounded-lg bg-destructive shadow-md shadow-destructive/25" />
              <div className="text-xs font-semibold text-destructive-text">Destructive</div>
              <div className="text-[10px] font-mono text-zinc-500">--color-destructive</div>
            </div>
            <div className="rounded-xl border border-border-default bg-surface-base p-3 space-y-1">
              <div className="h-10 rounded-lg bg-warning shadow-md shadow-warning/25" />
              <div className="text-xs font-semibold text-warning-text">Warning</div>
              <div className="text-[10px] font-mono text-zinc-500">--color-warning</div>
            </div>
            <div className="rounded-xl border border-border-default bg-surface-base p-3 space-y-1">
              <div className="h-10 rounded-lg bg-info shadow-md shadow-info/25" />
              <div className="text-xs font-semibold text-info-text">Info</div>
              <div className="text-[10px] font-mono text-zinc-500">--color-info</div>
            </div>
          </div>
        </section>

        {/* Section 2: Buttons Matrix */}
        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">
              2. Button Primitives
            </h2>
            <Button
              variant="outline"
              size="xs"
              onClick={() => setButtonLoading((prev) => !prev)}
            >
              Toggle Loading State
            </Button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-5 gap-4 rounded-2xl border border-border-default bg-surface-card/60 p-6">
            <div className="space-y-2">
              <div className="text-xs text-zinc-400 font-medium">Primary</div>
              <Button variant="primary" size="md" isLoading={buttonLoading} leftIcon={<Radar className="w-4 h-4" />}>
                Start Discovery
              </Button>
            </div>
            <div className="space-y-2">
              <div className="text-xs text-zinc-400 font-medium">Secondary</div>
              <Button variant="secondary" size="md" isLoading={buttonLoading} leftIcon={<Building2 className="w-4 h-4" />}>
                View Offices
              </Button>
            </div>
            <div className="space-y-2">
              <div className="text-xs text-zinc-400 font-medium">Outline</div>
              <Button variant="outline" size="md" isLoading={buttonLoading}>
                Cancel Run
              </Button>
            </div>
            <div className="space-y-2">
              <div className="text-xs text-zinc-400 font-medium">Ghost</div>
              <Button variant="ghost" size="md" isLoading={buttonLoading} rightIcon={<ExternalLink className="w-3.5 h-3.5" />}>
                Details
              </Button>
            </div>
            <div className="space-y-2">
              <div className="text-xs text-zinc-400 font-medium">Danger</div>
              <Button variant="danger" size="md" isLoading={buttonLoading}>
                Dismiss
              </Button>
            </div>
          </div>

          <div className="flex items-center gap-3 pt-2">
            <span className="text-xs text-zinc-500">Sizes:</span>
            <Button size="xs" variant="secondary">Extra Small (xs)</Button>
            <Button size="sm" variant="secondary">Small (sm)</Button>
            <Button size="md" variant="secondary">Medium (md)</Button>
            <Button size="lg" variant="secondary">Large (lg)</Button>
          </div>
        </section>

        {/* Section 3: Badges & Tags */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">
            3. Badges & Semantic Status Tags
          </h2>
          <div className="rounded-2xl border border-border-default bg-surface-card/60 p-6 flex flex-wrap gap-4 items-center">
            <Badge variant="primary" pulsing>
              Primary / Active
            </Badge>
            <Badge variant="secondary">
              Secondary / Neutral
            </Badge>
            <Badge variant="success" icon={<CheckCircle2 className="w-3.5 h-3.5" />}>
              Confirmed (Success)
            </Badge>
            <Badge variant="warning">
              Probable (Warning)
            </Badge>
            <Badge variant="destructive">
              Failed (Destructive)
            </Badge>
            <Badge variant="info">
              Hiring (Info)
            </Badge>
            <Badge variant="primary" size="md" pulsing>
              Engine Crawling (md)
            </Badge>
          </div>
        </section>

        {/* Section 4: Form Controls */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">
            4. Form Controls & Navigation
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 rounded-2xl border border-white/[0.08] bg-[#121216]/60 p-6">
            {/* Segmented Control */}
            <div className="space-y-2">
              <div className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">
                Segmented Tabs (0px Jump)
              </div>
              <SegmentedControl
                value={segmentedTab}
                onChange={setSegmentedTab}
                options={[
                  { value: 'offices', label: 'Offices', icon: <Building2 className="w-3.5 h-3.5" />, count: 1715 },
                  { value: 'jobs', label: 'Nearby Jobs', icon: <Briefcase className="w-3.5 h-3.5" />, count: 322 },
                ]}
              />
              <p className="text-[11px] text-zinc-500">
                Active tab: <span className="text-indigo-400 font-mono font-medium">{segmentedTab}</span>
              </p>
            </div>

            {/* Clearable SearchInput */}
            <div className="space-y-2">
              <div className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">
                Search Input (Clearable)
              </div>
              <SearchInput
                value={searchValue}
                onChange={(e) => setSearchValue(e.target.value)}
                onClear={() => setSearchValue('')}
                placeholder="Search companies, tech parks..."
              />
              <p className="text-[11px] text-zinc-500">
                Value: <span className="text-indigo-400 font-mono font-medium">{searchValue || '(empty)'}</span>
              </p>
            </div>

            {/* Standard Input with Error toggle */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">
                  Text Input
                </span>
                <button
                  type="button"
                  onClick={() => setInputError(!inputError)}
                  className="text-[11px] text-zinc-500 hover:text-zinc-300 underline cursor-pointer"
                >
                  {inputError ? 'Clear error' : 'Simulate error'}
                </button>
              </div>
              <Input
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                leftIcon={<MapPin className="w-3.5 h-3.5" />}
                error={inputError ? 'Invalid coordinate or region selected' : undefined}
                placeholder="Enter city or hub coordinates"
              />
            </div>

            {/* Custom Select */}
            <div className="space-y-2">
              <Select
                id="city-picker"
                label="City Hub Selector"
                value={selectedCity}
                onChange={(e) => setSelectedCity(e.target.value)}
                options={[
                  { value: 'hyderabad', label: 'Hyderabad (Telangana) — Hitec City' },
                  { value: 'bangalore', label: 'Bangalore (Karnataka) — Whitefield' },
                  { value: 'pune', label: 'Pune (Maharashtra) — Hinjawadi' },
                ]}
              />
            </div>

            {/* Range Slider */}
            <div className="space-y-2 md:col-span-2 pt-2">
              <Slider
                value={radiusKm}
                onChange={setRadiusKm}
                min={1}
                max={30}
                label="Discovery Radius Control"
                badgeText={`${radiusKm} km (~${coverageArea} km² coverage)`}
                presets={[
                  { label: '5 km', value: 5, desc: 'Core Hub' },
                  { label: '15 km', value: 15, desc: 'Tech Corridor' },
                  { label: '25 km', value: 25, desc: 'Greater Metro' },
                ]}
                scaleLabels={['1 km (Hyper-local)', '15 km (Tech Corridor)', '30 km (Full Metro)']}
              />
            </div>
          </div>
        </section>

        {/* Section 5: Cards & Containers */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">
            5. Card Primitives
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Card>
              <div className="text-xs font-semibold text-zinc-200">Default Obsidian Card</div>
              <p className="text-[11px] text-zinc-400 mt-1">
                Standard dark surface container with subtle border highlights for list items.
              </p>
            </Card>
            <Card hoverable>
              <div className="text-xs font-semibold text-zinc-200">Hoverable Card</div>
              <p className="text-[11px] text-zinc-400 mt-1">
                Subtle border brightening and smooth elevation transition on hover.
              </p>
            </Card>
            <Card selected>
              <div className="text-xs font-semibold text-indigo-300">Selected Card</div>
              <p className="text-[11px] text-indigo-200/80 mt-1">
                Highlighted with electric indigo border ring and soft glow when active on map.
              </p>
            </Card>
          </div>
        </section>

        {/* Section 6: Empty State & Modal Launcher */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">
            6. Empty State & Modal Component
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card>
              <EmptyState
                icon={<Search className="w-6 h-6" />}
                title="No Tech Offices in this Radius"
                description="Expand the radius slider or run our automated discovery crawler to map companies in this hub."
                action={
                  <Button variant="primary" size="sm" onClick={() => setIsModalOpen(true)}>
                    Open Discovery
                  </Button>
                }
              />
            </Card>

            <Card className="flex flex-col items-center justify-center text-center p-8 space-y-4">
              <div className="w-12 h-12 rounded-2xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400">
                <Radar className="w-6 h-6" />
              </div>
              <div>
                <h3 className="text-sm font-semibold text-white">Modal Component Preview</h3>
                <p className="text-xs text-zinc-400 mt-1 max-w-[280px]">
                  Test the composite modal dialog featuring backdrop blur, Escape listener, and header/footer slots.
                </p>
              </div>
              <Button variant="primary" onClick={() => setIsModalOpen(true)}>
                Launch Sample Modal
              </Button>
            </Card>
          </div>
        </section>

        {/* Sample Modal Instance */}
        <Modal
          isOpen={isModalOpen}
          onClose={() => setIsModalOpen(false)}
          title="Sample Design System Modal"
          subtitle="All typography, controls, and buttons inherit unified design tokens."
          icon={<Sparkles className="w-5 h-5 text-indigo-400" />}
          footer={
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setIsModalOpen(false)}>
                Cancel
              </Button>
              <Button variant="primary" size="sm" onClick={() => setIsModalOpen(false)}>
                Confirm Action
              </Button>
            </div>
          }
        >
          <div className="space-y-4 text-xs text-zinc-300">
            <p>
              This modal is composed purely using the new atomic design system primitives:
              <span className="font-mono text-indigo-400"> Modal</span>,
              <span className="font-mono text-indigo-400"> SegmentedControl</span>,
              <span className="font-mono text-indigo-400"> Button</span>, and
              <span className="font-mono text-indigo-400"> Badge</span>.
            </p>
            <div className="p-4 rounded-xl bg-[#09090b] border border-white/[0.08] space-y-2">
              <div className="flex items-center justify-between">
                <span className="font-semibold text-white">Live Status</span>
                <Badge variant="emerald" pulsing>Active Engine</Badge>
              </div>
              <p className="text-zinc-400 text-[11px]">
                Escape key closes dialog. Click backdrop closes dialog. Scrollbar is gutter-stable.
              </p>
            </div>
          </div>
        </Modal>
      </div>
    </div>
  );
}
