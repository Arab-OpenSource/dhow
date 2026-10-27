const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), ' +
  'textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

const stop = (event: KeyboardEvent): void => {
  event.preventDefault();
  event.stopPropagation();
};

export const wrapTab = (event: KeyboardEvent): void => {
  const section = event.currentTarget;
  if (event.key !== 'Tab' || !(section instanceof HTMLElement)) return;
  const items = [...section.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((item) => item.tabIndex >= 0);
  const first = items[0];
  const last = items[items.length - 1];
  if (!first || !last) {
    stop(event);
    return;
  }
  const active = document.activeElement;
  if (event.shiftKey && (active === first || active === section)) {
    stop(event);
    last.focus();
  } else if (!event.shiftKey && active === last) {
    stop(event);
    first.focus();
  }
};
