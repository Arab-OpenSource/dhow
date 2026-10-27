import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { Button } from '@/Core/Ui/Button';

describe('Button', () => {
  it('is a non-submitting button by default and fires onClick', async () => {
    const onClick = vi.fn();
    const { user } = renderUi(<Button onClick={onClick}>Publish now</Button>);
    const button = screen.getByRole('button', { name: 'Publish now' });
    expect(button).toHaveAttribute('type', 'button');
    await user.click(button);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('activates from the keyboard', async () => {
    const onClick = vi.fn();
    const { user } = renderUi(<Button onClick={onClick}>Refresh</Button>);
    await user.tab();
    expect(screen.getByRole('button', { name: 'Refresh' })).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('does not fire when disabled', async () => {
    const onClick = vi.fn();
    const { user } = renderUi(
      <Button disabled onClick={onClick}>
        Refresh
      </Button>,
    );
    await user.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(onClick).not.toHaveBeenCalled();
  });

  it('uses the aurora fill only for the aurora variant', () => {
    renderUi(
      <>
        <Button variant="aurora">Primary</Button>
        <Button>Secondary</Button>
      </>,
    );
    expect(screen.getByRole('button', { name: 'Primary' })).toHaveClass('bg-aurora', 'text-on-light');
    expect(screen.getByRole('button', { name: 'Secondary' })).toHaveClass('edge');
    expect(screen.getByRole('button', { name: 'Secondary' })).not.toHaveClass('bg-aurora');
  });

  it('hides the optional icon from assistive technology', () => {
    renderUi(<Button iconStart="check">Done</Button>);
    expect(screen.getByRole('button', { name: 'Done' }).querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });
});
