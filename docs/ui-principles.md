# UI principles

Status: **Accepted**. The current visual direction is "control room" (ADR-0059, which amends ADR-0034).

Decisions: ADR-0021 to ADR-0023, ADR-0034 and ADR-0059.

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
- **Typography:** IBM Plex Sans Arabic for Arabic, IBM Plex Sans for Latin and numerals, and IBM Plex Mono for code and logs. All three are self-hosted (SIL OFL 1.1) in weights 400, 500 and 600.
  - Arabic has its own sizes, set on `:root:lang(ar)` by the generated tokens, so no class is needed.
  - The scale, Latin / Arabic in px (size / line height):

    | Role | Latin | Arabic |
    |---|---|---|
    | Page title | 24 / 34 | 26 / 38 |
    | Section and slide title | 18 / 28 | 19 / 30 |
    | Card title | 16 / 24 | 17 / 26 |
    | Body | 15 / 24 | 16 / 28 |
    | Secondary text | 14 / 22 | 15 / 24 |
    | Buttons and labels | 14 / 20 | 15 / 22 |
    | Small buttons | 13 / 20 | 14 / 20 |
    | Badges and dates | 13 / 20 | 13 / 20 |
    | Stats | 30 / 38 | 30 / 38 |
    | Code and logs | 13 / 22 | 13 / 22 |

  - Nothing is smaller than 13px, and 13px is only for badges, dates and code. Titles go up to weight 600.
- **Spacing:** a 4px grid. The page gutter is 32px (16px on phones), cards have 20px of padding (14px on phones), blocks are 16px apart, and the top bar is 60px high.

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
- Four dark system themes, plus custom ones. Text meets WCAG AA on every surface of every theme, and cards stand out with a hairline instead of a shadow.
- Status has its own colors, each with a background and a foreground: green for success, blue for running, violet for queued, amber for warnings and coral for failures.

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
