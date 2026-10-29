import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { SectionNav } from '@/Core/Ui/SectionNav';

describe('SectionNav', () => {
  it('links the pages of a section and marks the open one', () => {
    const onSelect = vi.fn();
    renderUi(
      <SectionNav
        label="Account sections"
        items={[
          { id: 'profile', label: 'Profile', href: '/account/profile', current: true, onSelect },
          { id: 'security', label: 'Security', href: '/account/security', onSelect },
        ]}
      />,
    );
    const nav = screen.getByRole('navigation', { name: 'Account sections' });
    expect(within(nav).getByRole('link', { name: 'Profile' })).toHaveAttribute('aria-current', 'page');
    const security = within(nav).getByRole('link', { name: 'Security' });
    expect(security).toHaveAttribute('href', '/account/security');
    fireEvent.click(security);
    fireEvent.click(security, { shiftKey: true });
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('renders no links when the section has no pages', () => {
    renderUi(<SectionNav label="Account sections" items={[]} />);
    expect(within(screen.getByRole('navigation')).queryAllByRole('link')).toHaveLength(0);
  });
});
