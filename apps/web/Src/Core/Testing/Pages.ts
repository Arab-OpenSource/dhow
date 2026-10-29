import { screen } from '@testing-library/react';

export const findPage = async (title: string, timeout = 5000): Promise<HTMLElement> => {
  const heading = await screen.findByRole('heading', { level: 1, name: title, hidden: true }, { timeout });
  const main = heading.closest('main');
  if (main === null) throw new Error(`the page "${title}" is not inside the main area`);
  return main;
};
