import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { Breadcrumbs } from '@/Core/Ui/Breadcrumbs';

describe('Breadcrumbs', () => {
  it('shows the path in order, links what can be opened and marks the current page', async () => {
    const onSelect = vi.fn();
    const { user } = renderUi(
      <Breadcrumbs
        label="Page path"
        crumbs={[
          { id: 'group', label: 'Run' },
          { id: 'jobs', label: 'Jobs', href: '/jobs', onSelect },
          { id: 'detail', label: 'Job details' },
        ]}
      />,
    );
    const nav = screen.getByRole('navigation', { name: 'Page path' });
    expect(within(nav).getAllByRole('listitem').map((item) => item.textContent)).toEqual(['Run', 'Jobs', 'Job details']);
    expect(within(nav).queryByRole('link', { name: 'Run' })).not.toBeInTheDocument();
    expect(within(nav).getByText('Job details').closest('[aria-current]')).toHaveAttribute('aria-current', 'page');
    await user.click(within(nav).getByRole('link', { name: 'Jobs' }));
    expect(onSelect).toHaveBeenCalledOnce();
  });

  it('never links the last crumb, even when it has an href', () => {
    renderUi(<Breadcrumbs label="Page path" crumbs={[{ id: 'jobs', label: 'Jobs', href: '/jobs', onSelect: vi.fn() }]} />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(screen.getByText('Jobs').closest('[aria-current]')).toHaveAttribute('aria-current', 'page');
  });

  it('renders an empty list when there is no path', () => {
    renderUi(<Breadcrumbs label="Page path" crumbs={[]} />);
    expect(within(screen.getByRole('navigation')).queryAllByRole('listitem')).toHaveLength(0);
  });
});
