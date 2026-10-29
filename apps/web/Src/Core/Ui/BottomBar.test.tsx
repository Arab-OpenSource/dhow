import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { BottomBar, type IBottomBarItem } from '@/Core/Ui/BottomBar';

const items = (onSelect = vi.fn()): IBottomBarItem[] => [
  { id: 'home', icon: 'home', label: 'Home', href: '/', onSelect },
  { id: 'jobs', icon: 'play', label: 'Jobs', href: '/jobs', current: true, onSelect },
  { id: 'runners', icon: 'server', label: 'Runners', href: '/runners', onSelect },
  { id: 'org', icon: 'users', label: 'Organization', href: '/org/members', onSelect },
  { id: 'account', icon: 'user', label: 'Account', href: '/account/profile', onSelect },
];

describe('BottomBar', () => {
  it('shows every item with an icon and a visible label', () => {
    renderUi(<BottomBar label="Sections" items={items()} />);
    const nav = screen.getByRole('navigation', { name: 'Sections' });
    const links = within(nav).getAllByRole('link');
    expect(links.map((link) => link.textContent)).toEqual(['Home', 'Jobs', 'Runners', 'Organization', 'Account']);
    for (const link of links) {
      expect(link.querySelector('svg')).not.toBeNull();
      expect(within(link).getByText(link.textContent)).not.toHaveClass('sr-only');
    }
  });

  it('marks the current section and shows only below the md breakpoint', () => {
    renderUi(<BottomBar label="Sections" items={items()} />);
    expect(screen.getByRole('link', { name: 'Jobs' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Home' })).not.toHaveAttribute('aria-current');
    expect(screen.getByRole('navigation')).toHaveClass('md:hidden');
  });

  it('opens a section in the app on a plain click and keeps the real href', () => {
    const onSelect = vi.fn();
    renderUi(<BottomBar label="Sections" items={items(onSelect)} />);
    const link = screen.getByRole('link', { name: 'Organization' });
    expect(link).toHaveAttribute('href', '/org/members');
    fireEvent.click(link);
    fireEvent.click(link, { metaKey: true });
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('renders an empty bar without links', () => {
    renderUi(<BottomBar label="Sections" items={[]} />);
    expect(within(screen.getByRole('navigation')).queryAllByRole('link')).toHaveLength(0);
  });
});
