import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { SLIDE_BODY, SlideContent, SlidePinned } from '@/Core/Ui/Slide';
import { Tabs } from '@/Core/Ui/Tabs';

const TABS = [
  { id: 'one', label: 'One' },
  { id: 'two', label: 'Two' },
] as const;

describe('Tabs', () => {
  it('never lets the tab strip shrink below its own height when the panel under it is long', () => {
    renderUi(<Tabs base="t" label="Sections" tabs={TABS} value="one" onChange={vi.fn()} />);
    const list = screen.getByRole('tablist', { name: 'Sections' });
    expect(list).toHaveClass('min-h-fit', 'shrink-0', 'overflow-x-auto');
  });

  it('marks the selected tab and moves with the arrow keys', async () => {
    const onChange = vi.fn();
    const { user } = renderUi(<Tabs base="t" label="Sections" tabs={TABS} value="one" onChange={onChange} />);
    expect(screen.getByRole('tab', { name: 'One' })).toHaveAttribute('aria-selected', 'true');
    screen.getByRole('tab', { name: 'One' }).focus();
    await user.keyboard('{End}');
    expect(onChange).toHaveBeenCalledWith('two');
  });
});

describe('the slide body', () => {
  it('sizes every row to its content, so a scrolling child can never collapse to its padding', () => {
    expect(SLIDE_BODY.split(' ')).toEqual(expect.arrayContaining(['grid', 'auto-rows-max', 'content-start', 'overflow-auto']));
  });

  it('pins a strip to the top of the scrolling body on an opaque background', () => {
    renderUi(
      <SlideContent>
        <SlidePinned>
          <Tabs base="t" label="Sections" tabs={TABS} value="one" onChange={vi.fn()} />
        </SlidePinned>
        <p>Panel</p>
      </SlideContent>,
    );
    const pinned = screen.getByRole('tablist').parentElement;
    expect(pinned).toHaveAttribute('data-slide-pinned');
    expect(pinned).toHaveClass('sticky', 'z-10', 'bg-canvas');
    expect(pinned?.parentElement).toHaveClass('flex', 'flex-col');
  });
});
