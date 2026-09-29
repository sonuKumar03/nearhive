# Implementation Plan: NearHive UI Design System & Showcase

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a unified, accessible, and scalable dark-theme Design System for NearHive, including Tailwind CSS v4 design tokens, atomic UI primitives in `web/src/components/ui/`, an interactive Design System Showcase route at `/design-system`, and refactoring existing screens to consume the design system.

**Architecture:** 
- **Layer 1: Design Tokens & Utilities:** Centralize semantic theme tokens (surfaces, borders, primary amber accent, status colors, elevations, glows, stable scrollbars) into Tailwind v4 `@theme` in `globals.css` and configure the `cn` helper.
- **Layer 2: Atomic UI Primitives (`web/src/components/ui/`):** Implement composable, strictly typed UI components using `clsx` and `tailwind-merge` (`Button`, `Badge`, `SegmentedControl`, `Input`, `SearchInput`, `Select`, `Slider`, `Modal`, `Card`, `EmptyState`).
- **Layer 3: Interactive Showcase (`/design-system`):** Build a dedicated dark-themed showcase page at `web/src/app/design-system/page.tsx` displaying every token, component variant, interactive state, and modal preview.
- **Layer 4: Application Surface Migration:** Progressively refactor existing UI surfaces (`Sidebar`, `CompanyCard`, `JobCard`, `ScrapeModal`, `CompanyDetailDrawer`, `BackgroundScrapeWidget`, and `TopNav` in `page.tsx`) to replace bespoke inline styles with design system primitives.

**Tech Stack:** Next.js 15, React 19, Tailwind CSS v4, Lucide React, clsx, tailwind-merge.

---

## User Review Required

> [!IMPORTANT]
> **Tailwind CSS v4 Token Architecture**: NearHive uses Tailwind v4 (`@tailwindcss/postcss`). Design tokens will be defined via `@theme` in `globals.css` with CSS custom properties (`--color-surface-card`, `--color-border-subtle`, `--color-primary-amber`, etc.), ensuring full utility generation without a legacy `tailwind.config.js`.

> [!NOTE]
> **Showcase Route (`/design-system`)**: A standalone client route accessible at `http://localhost:3000/design-system` that serves as a live style guide and testing laboratory for developers and designers.

---

## Proposed Changes

### Component 1: Design Token Foundation & Utility Setup

#### [NEW] `web/src/lib/cn.ts`
- Export standard `cn(...inputs)` utility combining `clsx` and `tailwind-merge` for predictable conditional class composition.

#### [MODIFY] `web/src/app/globals.css`
- Define semantic dark theme tokens in `@theme`:
  - **Surfaces:** `--color-surface-base` (`#020617`), `--color-surface-subtle` (`#0b1329`), `--color-surface-card` (`#0f172a`), `--color-surface-overlay` (`#1e293b`).
  - **Borders:** `--color-border-subtle` (`#1e293b`), `--color-border-default` (`#334155`), `--color-border-focus` (`#f59e0b`).
  - **Accents:** Amber gradient primary palette (`--color-primary-400` to `--color-primary-600`), Emerald success, Rose danger, Sky info.
  - **Elevation & Glows:** `--shadow-glass`, `--shadow-glow-amber`, `--shadow-glow-emerald`.
  - **Scrollbar & Layout Utilities:** Stable scrollbar gutter default rules to prevent layout jumping.

---

### Component 2: Atomic UI Component Primitives (`web/src/components/ui/`)

Create modular, accessible UI primitives with TypeScript props and predictable variants:

#### [NEW] `web/src/components/ui/Button.tsx`
- **Variants:** `primary` (amber gradient with glow), `secondary` (dark slate card with border), `outline` (subtle border), `ghost` (clean hover), `danger` (rose-tinted micro-action).
- **Sizes:** `xs` (24px), `sm` (28px), `md` (36px), `lg` (44px).
- **Features:** Integrated spinner for `isLoading`, icon slots (`leftIcon`, `rightIcon`), fully accessible `disabled` and focus states.

#### [NEW] `web/src/components/ui/Badge.tsx`
- **Variants:** `amber` (running/accent), `emerald` (verified/complete), `rose` (failed/cancelled), `sky` (tech jobs), `slate` (neutral).
- **Features:** Optional animated pulsing live indicator dot (`pulsing: boolean`), sizes `sm` and `md`.

#### [NEW] `web/src/components/ui/SegmentedControl.tsx`
- Reusable iOS/Linear-style dark tab pill switcher.
- **Guarantees:** Strict box-model symmetry (0px layout shifts on toggle), fixed height, keyboard navigation (`ArrowLeft` / `ArrowRight`).

#### [NEW] `web/src/components/ui/Input.tsx` & `SearchInput.tsx`
- Dark glass styling with clearable `X` button, search icon prefix, focus ring (`ring-amber-500/40`), and error states.

#### [NEW] `web/src/components/ui/Select.tsx`
- Styled dark dropdown wrapper with chevron, landmark/state formatting, and high contrast focus.

#### [NEW] `web/src/components/ui/Slider.tsx`
- Range input with styled track, gradient fill, value tooltip, and quick preset buttons integration.

#### [NEW] `web/src/components/ui/Modal.tsx`
- Accessible dialog wrapper with `backdrop-blur`, Escape key listener, click-outside dismissal, sticky header, scrollable body, and action footer.

#### [NEW] `web/src/components/ui/Card.tsx`
- Glassmorphism dark container with subtle borders (`border-slate-800`), hover depth transitions, and optional active/selected highlight border.

#### [NEW] `web/src/components/ui/EmptyState.tsx`
- Structured empty/no-results component with circular icon container, heading, subtext, and optional CTA button.

---

### Component 3: Design System Showcase Route

#### [NEW] `web/src/app/design-system/page.tsx`
- Interactive showcase and style guide page accessible at `http://localhost:3000/design-system`:
  - **Color Tokens Section:** Swatches for surfaces, borders, primary amber ramp, and semantic status colors with contrast ratios.
  - **Typography Section:** Headers, body, and monospace coordinates/counters display.
  - **Button Matrix:** Interactive buttons across all 5 variants, 4 sizes, loading states, and icon combinations.
  - **Badge Gallery:** Static and pulsing live status badges across all colorways.
  - **Form Controls Playground:** Live `SegmentedControl`, `Input`, `SearchInput`, `Select`, and `Slider` with interactive state feedback.
  - **Card Variations:** Default, hoverable, and active selection cards.
  - **Empty States:** Sample zero-results and initial state layouts.
  - **Modal Trigger:** Live button to launch an example `Modal` using all primitives.

---

### Component 4: Refactor Existing Application Surfaces

Progressively migrate application components to consume the new design system primitives:

#### [MODIFY] `web/src/components/sidebar/Sidebar.tsx`
- Use `SegmentedControl` for `Offices` vs `Nearby Jobs` tabs.
- Use `SearchInput` for filter queries.
- Use `EmptyState` for zero offices / zero jobs states.

#### [MODIFY] `web/src/components/sidebar/CompanyCard.tsx` & `JobCard.tsx`
- Use `Card` and `Badge` primitives for verification status, work arrangement tags, and distance metrics.

#### [MODIFY] `web/src/components/drawers/ScrapeModal.tsx`
- Use `Modal`, `SegmentedControl`, `Button`, `Select`, `Slider`, and `Badge`.

#### [MODIFY] `web/src/components/drawers/CompanyDetailDrawer.tsx`
- Replace raw drawers and ad-hoc badge capsules with standard `Badge`, `Button`, `Card`, and typography tokens.

#### [MODIFY] `web/src/components/scrapers/BackgroundScrapeWidget.tsx`
- Use `Badge` and `Button` primitives with animated indicators.

#### [MODIFY] `web/src/app/page.tsx`
- Replace top navigation buttons (`Bangalore`, `Hyderabad`, etc.) and radius slider with `Button` and `Slider` primitives.

---

## Phased Implementation Plan

```mermaid
flowchart TD
    T1[Task 1: Design Tokens & cn Utility] --> T2[Task 2: Core Atoms - Button, Badge, Card, EmptyState]
    T2 --> T3[Task 3: Form Controls - Input, Select, Slider, SegmentedControl]
    T3 --> T4[Task 4: Composite Modal Primitive]
    T4 --> T5[Task 5: Interactive Showcase Route /design-system]
    T5 --> T6[Task 6: Refactor ScrapeModal & BackgroundScrapeWidget]
    T6 --> T7[Task 7: Refactor Sidebar, CompanyCard & JobCard]
    T7 --> T8[Task 8: Refactor CompanyDetailDrawer & TopNav]
    T8 --> T9[Task 9: End-to-End Test Verification]
```

### Phase 1: Tokens & Atoms (Tasks 1–2)
- Set up `cn.ts` and `@theme` in `globals.css`.
- Build `Button`, `Badge`, `Card`, and `EmptyState`.
- Verify with TypeScript typecheck (`npm run type-check`).

### Phase 2: Form Controls & Layout (Tasks 3–4)
- Build `SegmentedControl`, `Input`, `SearchInput`, `Select`, `Slider`, and `Modal`.
- Verify keyboard accessibility and zero-layout-shift behavior.

### Phase 3: Interactive Showcase Route (Task 5)
- Create `web/src/app/design-system/page.tsx`.
- Verify in browser via Chrome DevTools on `http://localhost:3000/design-system`.

### Phase 4: Screen Refactoring (Tasks 6–8)
- Progressively swap bespoke styles in `ScrapeModal`, `BackgroundScrapeWidget`, `Sidebar`, `CompanyCard`, `JobCard`, `CompanyDetailDrawer`, and `page.tsx`.

### Phase 5: Verification (Task 9)
- Verify directly on the live app (`http://localhost:3000`) and the showcase (`http://localhost:3000/design-system`).
- Run complete test suites (`tsc --noEmit`, `next build`, `pytest`, `go test`).

---

## Verification Plan

### Automated Tests
```bash
# 1. TypeScript compilation check
npm --prefix web run type-check

# 2. Production build verification
npm --prefix web run build

# 3. Python Discovery test suite
rtk env DATABASE_URL="postgresql://postgres:postgres@localhost:5432/nearhive?sslmode=disable" python-discovery/.venv/bin/pytest python-discovery/tests/

# 4. Go API test suite
rtk go test ./...
```

### Visual & Browser Verification
- Using `chrome-devtools-mcp` directly on `http://localhost:3000`:
  - Open `http://localhost:3000/design-system` and visually inspect the interactive showcase of all components.
  - Verify `Sidebar` tab toggle maintains 0.0px layout jump.
  - Verify `ScrapeModal` opens with exact design tokens, badges, and quick chips.
  - Verify `CompanyDetailDrawer` renders verified badges and action buttons correctly.
