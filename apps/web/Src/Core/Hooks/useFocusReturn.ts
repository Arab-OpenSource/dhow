import { useRef } from 'react';

export interface IFocusReturn {
  onOpenAutoFocus: () => void;
  onCloseAutoFocus: (event: Event) => void;
}

export const useFocusReturn = (): IFocusReturn => {
  const opener = useRef<HTMLElement | null>(null);

  return {
    onOpenAutoFocus: () => {
      const active = document.activeElement;
      opener.current = active instanceof HTMLElement && active !== document.body ? active : null;
    },
    onCloseAutoFocus: (event) => {
      event.preventDefault();
      if (opener.current?.isConnected) opener.current.focus();
    },
  };
};
