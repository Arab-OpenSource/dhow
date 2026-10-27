# ADR-0022: Single workspace + slide-over panels + command palette

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Users want less navigation, less data clutter, and slide-style transitions.

## Decision
One workspace overview. Details open in stacked slide-over panels from the inline-end edge. ⌘K / Ctrl+K reaches every action. Every state is deep-linkable. Motion respects reduced-motion settings.

## Alternatives considered
A classic sidebar with pages; full-screen step-by-step slides.

## Consequences
Panel state must be URL-addressable. Requires careful focus management.
