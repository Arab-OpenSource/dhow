import type { ReactElement } from 'react';

export type TIconName =
  | 'project'
  | 'health'
  | 'search'
  | 'bell'
  | 'close'
  | 'check'
  | 'alert'
  | 'info'
  | 'queue'
  | 'commit'
  | 'back'
  | 'next'
  | 'plus'
  | 'settings'
  | 'database'
  | 'migrations';

export const MIRRORED_ICONS: readonly TIconName[] = ['back', 'next'];

export const ICONS: Record<TIconName, ReactElement> = {
  project: (
    <>
      <circle cx="12" cy="5" r="2" />
      <path d="M12 7v14" />
      <path d="M5 13a7 7 0 0 0 14 0" />
      <path d="M9 10h6" />
    </>
  ),
  health: <path d="M3 12h4l2.5-6 5 12 2.5-6h4" />,
  search: (
    <>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" />
    </>
  ),
  bell: (
    <>
      <path d="M6 16V11a6 6 0 0 1 12 0v5l1.5 2h-15z" />
      <path d="M10 21h4" />
    </>
  ),
  close: <path d="M6 6l12 12M18 6 6 18" />,
  check: <path d="m5 12 5 5 9-10" />,
  alert: (
    <>
      <path d="M12 3.5 21.5 20h-19z" />
      <path d="M12 10v4M12 17h.01" />
    </>
  ),
  info: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5M12 8h.01" />
    </>
  ),
  queue: (
    <>
      <path d="M7 3h10M7 21h10" />
      <path d="M8 3c0 4 8 5 8 9s-8 5-8 9" />
      <path d="M16 3c0 4-8 5-8 9s8 5 8 9" />
    </>
  ),
  commit: (
    <>
      <circle cx="12" cy="12" r="3.5" />
      <path d="M3 12h5.5M15.5 12H21" />
    </>
  ),
  back: <path d="M19 12H5M11 6l-6 6 6 6" />,
  next: <path d="m9 6 6 6-6 6" />,
  plus: <path d="M12 5v14M5 12h14" />,
  settings: (
    <>
      <path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1" />
      <circle cx="15" cy="6" r="2" />
      <circle cx="9" cy="12" r="2" />
      <circle cx="17" cy="18" r="2" />
    </>
  ),
  database: (
    <>
      <rect x="3.5" y="4" width="17" height="7" rx="2" />
      <rect x="3.5" y="13" width="17" height="7" rx="2" />
      <path d="M7.5 7.5h.01M7.5 16.5h.01" />
    </>
  ),
  migrations: (
    <>
      <path d="M12 3 20 7.5v9L12 21l-8-4.5v-9z" />
      <path d="m4 7.5 8 4.5 8-4.5" />
      <path d="M12 12v9" />
    </>
  ),
};
