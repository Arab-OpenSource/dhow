import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { LinkRow } from '@/Modules/Organization/Components/LinkRow';

describe('LinkRow', () => {
  it('marks the selected row and follows plain clicks in the app', async () => {
    const onSelect = vi.fn();
    const { user } = renderUi(
      <ul>
        <LinkRow href="/org/members/a" current title="Mona" onSelect={onSelect} />
        <LinkRow href="/org/members/b" current={false} title="Hana" onSelect={vi.fn()} />
      </ul>,
    );
    expect(screen.getByRole('link', { current: 'page' })).toHaveTextContent('Mona');
    expect(screen.getByRole('link', { name: 'Hana' })).not.toHaveAttribute('aria-current');
    expect(screen.getByRole('link', { name: 'Hana' })).toHaveAttribute('href', '/org/members/b');
    await user.click(screen.getByRole('link', { name: 'Mona' }));
    expect(onSelect).toHaveBeenCalledOnce();
  });
});
