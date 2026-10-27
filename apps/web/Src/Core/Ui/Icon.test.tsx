import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Icon } from '@/Core/Ui/Icon';
import { ICONS, type TIconName } from '@/Core/Ui/Icons';

describe('Icon', () => {
  it('is decorative without a label', () => {
    const { container } = render(<Icon name="search" />);
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });

  it('is an image with a label', () => {
    render(<Icon name="search" label="Search" />);
    expect(screen.getByRole('img', { name: 'Search' })).toBeInTheDocument();
  });

  it('draws every icon on the 24px grid with the 1.6px stroke', () => {
    for (const name of Object.keys(ICONS) as TIconName[]) {
      const { container, unmount } = render(<Icon name={name} />);
      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
      expect(svg).toHaveAttribute('stroke-width', '1.6');
      expect(svg?.childElementCount).toBeGreaterThan(0);
      unmount();
    }
  });
});
