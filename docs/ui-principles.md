# UI principles

Status: **Draft**. Final brand color and Arabic typeface are **pending the design-system review**:

- Color candidates: deep sea teal, warm saffron/sand, Nile green.
- Font candidates: IBM Plex Sans Arabic, Readex Pro, Cairo/Tajawal.

Decisions: ADR-0021 to ADR-0023.

## 1. Arabic-first, not "mirrored English" (ADR-0021)

- The **primary product language is Arabic**, written in a friendly Egyptian-leaning register for UI copy.
  - English is the second first-class language.
  - Further languages come later through community translation.
- **Screens are designed RTL first**, then adapted to LTR. Layout, iconography direction, reading order, number and date formatting, and empty states are all considered natively in Arabic.
- Engineering rules:
  - Use **CSS logical properties only** (`margin-inline-start`, `inset-inline-end`, …). Lint rules forbid physical left/right.
  - Icons that imply direction (arrows, chevrons, "back") are mirrored via `dir`. Icons that don't imply direction are never mirrored.
  - **Never hardcode strings.** CI fails on missing keys in any shipped locale.
  - Mixed-direction content gets proper bidi isolation: commit SHAs, branch names, paths and logs render LTR inside RTL layouts using `<bdi>` / `dir="ltr"`.
  - Logs and code are always LTR, in monospace.
- **Typography:** the Arabic and Latin faces are tuned together (x-height and weight matching), and the line-height accommodates Arabic diacritics.

## 2. Navigation model (ADR-0022)

- **One workspace.** The home screen is the org's projects and environments, shown as a calm overview of what is deployed where and what needs attention.
- **Slide-over panels.** Details (deployment, job logs, environment settings, approvals) open in panels that slide in from the inline-end edge. Panels stack, and closing them returns you exactly where you were. Users rarely change pages.
- **⌘K / Ctrl+K command palette** for everything: jump to a project, deploy, roll back, approve, search runs, switch org.
- **Motion:** transitions are short slides of 150–250 ms with eased curves, and they respect `prefers-reduced-motion`.
- Every view has a **deep-linkable URL**, including open panels, so links in notifications land in the right context.

## 3. Calm, low-density visual style

- The style is minimal: generous whitespace, strong typographic hierarchy, one accent color, and status shown with calm semantic colors **plus** icons and text (never color alone).
- **Progressive disclosure:**
  - Summaries come first and details appear on demand.
  - Tables are not the default view.
  - Show the ~5 things that matter and put the rest behind a panel.
- Dark and light themes follow the system setting, with a manual override.

## 4. Performance budgets

| Budget | Target |
|---|---|
| Initial JS (gzip) on first route | < 150 KB |
| Route-level code splitting | Mandatory, including charts and editors, which load lazily |
| Interaction response (INP) | < 100 ms |
| LCP on a typical VPS-hosted install | < 1.5 s |
| Log viewer | Virtualized; smooth at 100k+ lines; follow-tail toggle |
| Live updates | Patch caches incrementally from SSE events; no refetch storms |

## 5. Stack (ADR-0023)

- React with Vite and TypeScript.
- Tailwind CSS (logical utilities), with **shadcn/ui** components built on Radix primitives and owned in `packages/ui`.
- **Motion** for transitions.
- TanStack Query (object API) for server state. The API client is generated from OpenAPI.
- i18next, with ICU messages for Arabic plural forms.
- Accessibility target: **WCAG 2.2 AA**. Requirements include full keyboard support, visible focus, labelled icon buttons, `role="log"` with `aria-live` on live logs, and contrast checked in both themes.

## 6. Process

- **The design system comes before implementation.** Tokens, typography, components and key screens are reviewed and signed off before any production UI is built.
- Every component ships with RTL and LTR stories, both themes, and visual-regression tests.
