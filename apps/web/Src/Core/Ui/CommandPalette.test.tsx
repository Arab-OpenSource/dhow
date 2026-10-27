import { fireEvent, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { findBackdrop } from '@/Core/Testing/Overlay';
import { renderUi } from '@/Core/Testing/Render';
import { CommandPalette, type ICommandItem } from '@/Core/Ui/CommandPalette';

const ITEMS: ICommandItem[] = [
  { id: 'workspace', label: 'Go to workspace', icon: 'project', kind: 'Page' },
  { id: 'status', label: 'Open system status', icon: 'health', kind: 'Panel' },
  { id: 'language', label: 'Switch to English', kind: 'Action' },
];

const setup = (overrides: { onSelect?: (item: ICommandItem) => void; onOpenChange?: (open: boolean) => void } = {}) =>
  renderUi(
    <CommandPalette
      open
      onOpenChange={overrides.onOpenChange ?? vi.fn()}
      items={ITEMS}
      onSelect={overrides.onSelect ?? vi.fn()}
      label="Commands"
      placeholder="Search for a page or an action"
      emptyText="Nothing matches"
    />,
  );

describe('CommandPalette', () => {
  it('opens with the search field focused and every command listed', async () => {
    setup();
    const input = await screen.findByRole('combobox', { name: 'Commands' });
    await waitFor(() => {
      expect(input).toHaveFocus();
    });
    expect(screen.getAllByRole('option')).toHaveLength(3);
  });

  it('filters while typing and reports when nothing matches', async () => {
    const { user } = setup();
    await user.type(await screen.findByRole('combobox'), 'status');
    expect(screen.getAllByRole('option')).toHaveLength(1);
    expect(screen.getByRole('option')).toHaveTextContent('Open system status');
    await user.clear(screen.getByRole('combobox'));
    await user.type(screen.getByRole('combobox'), 'zzz');
    expect(screen.queryByRole('option')).not.toBeInTheDocument();
    expect(screen.getByText('Nothing matches')).toBeInTheDocument();
  });

  it('moves the active option with the arrow keys and selects it with Enter', async () => {
    const onSelect = vi.fn();
    const { user } = setup({ onSelect });
    const input = await screen.findByRole('combobox');
    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true');
    await user.keyboard('{ArrowDown}');
    const second = screen.getAllByRole('option')[1];
    expect(second).toHaveAttribute('aria-selected', 'true');
    expect(input).toHaveAttribute('aria-activedescendant', second?.id);
    await user.keyboard('{Enter}');
    expect(onSelect).toHaveBeenCalledWith(ITEMS[1]);
  });

  it('wraps around at both ends', async () => {
    const { user } = setup();
    await screen.findByRole('combobox');
    await user.keyboard('{ArrowUp}');
    expect(screen.getAllByRole('option')[2]).toHaveAttribute('aria-selected', 'true');
    await user.keyboard('{ArrowDown}');
    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true');
  });

  it('selects with the mouse and asks to close with Escape', async () => {
    const onSelect = vi.fn();
    const onOpenChange = vi.fn();
    const { user } = setup({ onSelect, onOpenChange });
    await user.click(await screen.findByRole('option', { name: /Switch to English/ }));
    expect(onSelect).toHaveBeenCalledWith(ITEMS[2]);
    await user.keyboard('{Escape}');
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('never injects a style element, so a strict style-src policy holds', async () => {
    const before = document.querySelectorAll('style').length;
    setup();
    await screen.findByRole('combobox');
    expect(document.querySelectorAll('style')).toHaveLength(before);
  });

  it('asks to close when the backdrop is pressed', async () => {
    const onOpenChange = vi.fn();
    setup({ onOpenChange });
    await screen.findByRole('combobox');
    fireEvent.click(findBackdrop());
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
