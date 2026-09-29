# Redesign Clean-Up & Showcase Scrolling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable smooth scrolling on `/design-system` by removing root body overflow clipping, and clean up legacy defensive fallback shims across the UI components to treat NearHive as a completely new, modern greenfield application. Per user instruction: all changes in this run will remain staged in git without committing.

**Architecture:** 
1. Fix layout root scrolling: configure `web/src/app/design-system/page.tsx` with a self-contained scrolling viewport (`h-screen overflow-y-auto [scrollbar-gutter:stable]`).
2. Clean up legacy fallbacks: streamline `CompanyDetailDrawer`, `JobCard`, `CompanyCard`, and `ScrapeModal` to consume direct, typed fields without redundant legacy dual-field fallbacks.
3. Stage all modifications via `rtk git add` without committing.

**Tech Stack:** Next.js 15 (App Router), Tailwind CSS v4, TypeScript, React 19.

---

## Global Constraints
- Commit restriction: Keep all changes staged in git; do NOT commit during this run.
- Zero fallbacks: Clean, direct, modern API usage without defensive shims or legacy aliases.
- CLI execution: Prefix all shell commands with `rtk `.
