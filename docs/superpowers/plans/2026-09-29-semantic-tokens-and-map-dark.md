# Semantic Color Tokens & Dark Map Flash Elimination Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:**
1. Refactor design tokens in `globals.css` and UI primitives (`Button`, `Badge`, `Card`, `SegmentedControl`, `Input`, `Select`, `Slider`, `Modal`, `EmptyState`) to use standard **semantic variable names** (`primary`, `secondary`, `surface`, `border`, `muted`, `accent`, `success`, `destructive`, `warning`, `info`) so that switching color themes requires modifying only token values in CSS, never component class names.
2. Fix the painful "white splash" on the Leaflet map when navigating between locations or loading tiles by overriding Leaflet's `#ddd` container background to pitch black (`#09090b`) and switching to native CartoDB Dark Matter tiles with smooth tile buffering.

**Architecture:**
- **Layer 1: Semantic Theme Abstraction (`globals.css`):**
  Defines semantic CSS variables with standard design system names (`--primary`, `--primary-hover`, `--primary-foreground`, `--primary-glow`, `--secondary`, `--muted`, `--surface-base`, `--surface-card`, `--surface-elevated`, `--border-default`, `--border-subtle`, `--success`, `--destructive`, `--warning`, `--info`).
- **Layer 2: Decoupled UI Component Primitives (`web/src/components/ui/`):**
  Components reference semantic CSS variables (`bg-primary`, `text-primary`, `border-border`, `bg-card`, etc.) instead of hardcoded color hues like `indigo`, `amber`, or `slate`.
- **Layer 3: Seamless Dark Map Rendering (`ClientMap.tsx`, `globals.css`):**
  - Override `.leaflet-container`, `.leaflet-tile-pane`, and `.leaflet-tile` backgrounds to `var(--surface-base)` (`#09090b !important`).
  - Use native CartoDB Dark Matter raster tiles (`https://{s}.basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}{r}.png`) with `keepBuffer: 12` and `updateWhenIdle: false`, eliminating tile loading flashes and CSS inversion artifacts.
  - Dynamically style the epicenter circle and marker using the semantic `--primary` token.
