export const findBackdrop = (): HTMLElement => {
  const node = document.querySelector<HTMLElement>('body > div[aria-hidden="true"].fixed');
  if (!node) throw new Error('the overlay backdrop is not on the page');
  return node;
};
