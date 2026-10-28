import { create } from 'zustand';

export const ORGANIZATION_KEY = 'dhow.organization';
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

const readStored = (): string | null => {
  try {
    const value = window.localStorage.getItem(ORGANIZATION_KEY);
    return value !== null && UUID.test(value) ? value : null;
  } catch {
    return null;
  }
};

const writeStored = (value: string | null): void => {
  try {
    if (value === null) window.localStorage.removeItem(ORGANIZATION_KEY);
    else window.localStorage.setItem(ORGANIZATION_KEY, value);
  } catch {
    return;
  }
};

interface IOrganizationState {
  CurrentId: string | null;
  Set: (id: string | null) => void;
}

export const useOrganizationStore = create<IOrganizationState>((set) => ({
  CurrentId: readStored(),
  Set: (id) => {
    writeStored(id);
    set({ CurrentId: id });
  },
}));
