import { fireEvent, screen, waitFor } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';

import { findBackdrop } from '@/Core/Testing/Overlay';
import { renderUi } from '@/Core/Testing/Render';
import { SlideOver } from '@/Core/Ui/SlideOver';

const Harness = ({ initiallyOpen = false }: { initiallyOpen?: boolean }) => {
  const [Open, setOpen] = useState(initiallyOpen);
  return (
    <>
      <button
        type="button"
        onClick={() => {
          setOpen(true);
        }}
      >
        Open status
      </button>
      <SlideOver
        open={Open}
        onClose={() => {
          setOpen(false);
        }}
        title="System status"
        subtitle="Live checks"
        closeLabel="Close panel"
        footer={<button type="button">Refresh</button>}
      >
        <p>Panel body</p>
      </SlideOver>
    </>
  );
};

describe('SlideOver', () => {
  it('is a labelled modal dialog that takes focus when it opens', async () => {
    const { user } = renderUi(<Harness />);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Open status' }));
    const dialog = await screen.findByRole('dialog', { name: 'System status' });
    expect(dialog).toHaveAccessibleDescription('Live checks');
    expect(screen.getByText('Panel body')).toBeInTheDocument();
    await waitFor(() => {
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    });
  });

  it('closes with Escape and returns focus to the control that opened it', async () => {
    const { user } = renderUi(<Harness />);
    const opener = screen.getByRole('button', { name: 'Open status' });
    await user.click(opener);
    await screen.findByRole('dialog');
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(opener).toHaveFocus();
  });

  it('closes from the close button', async () => {
    const { user } = renderUi(<Harness initiallyOpen />);
    await user.click(await screen.findByRole('button', { name: 'Close panel' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('keeps keyboard focus inside while open', async () => {
    const { user } = renderUi(<Harness initiallyOpen />);
    const dialog = await screen.findByRole('dialog');
    for (let press = 0; press < 5; press += 1) {
      await user.tab();
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    }
  });

  it('closes when the backdrop is pressed', async () => {
    renderUi(<Harness initiallyOpen />);
    await screen.findByRole('dialog');
    fireEvent.click(findBackdrop());
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('never injects a style element, so a strict style-src policy holds', async () => {
    const before = document.querySelectorAll('style').length;
    renderUi(<Harness initiallyOpen />);
    await screen.findByRole('dialog');
    expect(document.querySelectorAll('style')).toHaveLength(before);
  });
});
