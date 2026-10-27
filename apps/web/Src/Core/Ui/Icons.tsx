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
  | 'commit';

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
};
