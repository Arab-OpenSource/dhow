import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { IconRail, type IRailItem } from '@/Core/Ui/IconRail';

const build = (onSelect = vi.fn()): { items: IRailItem[]; bottom: IRailItem[] } => ({
  items: [{ id: 'workspace', icon: 'project', label: 'Workspace', href: '/', current: true, onSelect }],
  bottom: [{ id: 'status', icon: 'health', label: 'System status', href: '/?stack=system-status', onSelect }],
});

describe('IconRail', () => {
  it('is a labelled navigation landmark that marks only the current page', () => {
    const { items, bottom } = build();
    renderUi(<IconRail label="Main navigation" items={items} bottom={bottom} />);
    expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Workspace' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'System status' })).not.toHaveAttribute('aria-current');
  });

  it('keeps real hrefs so links can be opened in a new tab', () => {
    const { items, bottom } = build();
    renderUi(<IconRail label="Main navigation" items={items} bottom={bottom} />);
    expect(screen.getByRole('link', { name: 'System status' })).toHaveAttribute('href', '/?stack=system-status');
  });

  it('handles a plain click in the app and leaves modified clicks to the browser', () => {
    const onSelect = vi.fn();
    const { items, bottom } = build(onSelect);
    renderUi(<IconRail label="Main navigation" items={items} bottom={bottom} />);
    const link = screen.getByRole('link', { name: 'System status' });
    const handledByApp: boolean[] = [];
    const record = (event: Event) => {
      handledByApp.push(event.defaultPrevented);
      event.preventDefault();
    };
    document.addEventListener('click', record);
    fireEvent.click(link);
    fireEvent.click(link, { ctrlKey: true });
    document.removeEventListener('click', record);
    expect(handledByApp).toEqual([true, false]);
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('shows the label as a tooltip on keyboard focus', async () => {
    const { items, bottom } = build();
    const { user } = renderUi(<IconRail label="Main navigation" items={items} bottom={bottom} />);
    await user.tab();
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Workspace');
  });
});
