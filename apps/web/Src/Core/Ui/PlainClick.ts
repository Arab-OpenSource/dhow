import type { MouseEvent } from 'react';

export const isPlainClick = (event: MouseEvent<HTMLAnchorElement>): boolean =>
  event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;

export const followInApp =
  (onSelect: () => void) =>
  (event: MouseEvent<HTMLAnchorElement>): void => {
    if (!isPlainClick(event)) return;
    event.preventDefault();
    onSelect();
  };
