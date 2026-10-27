export const PANEL_IDS = ['system-status', 'notifications'] as const;

export type TPanelId = (typeof PANEL_IDS)[number];

export const UNKNOWN_PANEL = 'unknown';

export type TPanelParam = TPanelId | typeof UNKNOWN_PANEL;

export interface IShellSearch {
  panel?: TPanelParam;
}

const isPanelId = (value: unknown): value is TPanelId => PANEL_IDS.some((id) => id === value);

export const parseShellSearch = (search: Record<string, unknown>): IShellSearch => {
  const raw = search.panel;
  if (raw === undefined) return {};
  return { panel: isPanelId(raw) ? raw : UNKNOWN_PANEL };
};
