import { screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { CheckList } from '@/Core/Ui/CheckList';

type TPick = 'a' | 'b' | 'c';

const nth = <T,>(items: readonly T[], index: number): T => {
  const item = items[index];
  if (item === undefined) throw new Error(`missing item ${index}`);
  return item;
};


const Harness = () => {
  const [Values, setValues] = useState<TPick[]>(['a']);
  return (
    <CheckList
      label="Picks"
      items={[
        { value: 'a', title: 'First' },
        { value: 'b', title: 'Second', about: 'second item' },
        { value: 'c', title: 'Third', disabled: true },
      ]}
      values={Values}
      onChange={setValues}
    />
  );
};

describe('CheckList', () => {
  it('toggles each item on its own and keeps disabled items untouched', async () => {
    const { user } = renderUi(<Harness />);
    const group = screen.getByRole('group', { name: 'Picks' });
    expect(group).toBeInTheDocument();
    const boxes = screen.getAllByRole('checkbox');
    const [first, second, third] = [nth(boxes, 0), nth(boxes, 1), nth(boxes, 2)];
    expect(first).toHaveAttribute('aria-checked', 'true');
    expect(second).toHaveAttribute('aria-checked', 'false');
    await user.click(second);
    expect(second).toHaveAttribute('aria-checked', 'true');
    await user.click(first);
    expect(first).toHaveAttribute('aria-checked', 'false');
    expect(third).toBeDisabled();
    await user.click(third);
    expect(third).toHaveAttribute('aria-checked', 'false');
    expect(screen.getByText('second item')).toBeInTheDocument();
  });
});
