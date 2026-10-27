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

  it('fills only the primary variant, and gives the others a line or nothing', () => {
    renderUi(
      <>
        <Button variant="primary">Primary</Button>
        <Button>Secondary</Button>
        <Button variant="ghost">Quiet</Button>
        <Button variant="danger" iconStart="trash">
          Delete
        </Button>
      </>,
    );
    expect(screen.getByRole('button', { name: 'Primary' })).toHaveClass('bg-accent', 'text-on-accent');
    expect(screen.getByRole('button', { name: 'Secondary' })).toHaveClass('border-line-2', 'bg-transparent');
    expect(screen.getByRole('button', { name: 'Secondary' })).not.toHaveClass('bg-accent');
    expect(screen.getByRole('button', { name: 'Quiet' })).toHaveClass('bg-transparent');
    expect(screen.getByRole('button', { name: 'Quiet' })).not.toHaveClass('border-line-2');
    expect(screen.getByRole('button', { name: 'Delete' })).toHaveClass('text-accent-coral');
  });

  it('is secondary by default and 40px tall, or 34px when small', () => {
    renderUi(
      <>
        <Button>Normal</Button>
        <Button size="sm">Small</Button>
      </>,
    );
    expect(screen.getByRole('button', { name: 'Normal' })).toHaveAttribute('data-variant', 'secondary');
    expect(screen.getByRole('button', { name: 'Normal' })).toHaveClass('h-btn', 'rounded-btn');
    expect(screen.getByRole('button', { name: 'Small' })).toHaveClass('h-btn-sm');
  });

  it('refuses a danger button without an icon at compile time', () => {
    // @ts-expect-error a destructive action always carries an icon
    renderUi(<Button variant="danger">Delete</Button>);
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
  });

  it('hides the optional icon from assistive technology', () => {
    renderUi(<Button iconStart="check">Done</Button>);
    expect(screen.getByRole('button', { name: 'Done' }).querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });
});
