import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { MasterDetail } from '@/Core/Ui/MasterDetail';

describe('MasterDetail', () => {
  it('shows only the list when nothing is selected', () => {
    const { container } = renderUi(<MasterDetail listLabel="Runners" list={<p>list</p>} />);
    expect(screen.getByRole('region', { name: 'Runners' })).toHaveTextContent('list');
    expect(container.querySelector('[data-master-detail]')).toBeNull();
  });

  it('puts the detail beside the list on wide screens and hides the list below them', () => {
    const { container } = renderUi(
      <MasterDetail listLabel="Runners" detailLabel="runner-01" list={<p>list</p>} detail={<p>detail</p>} />,
    );
    expect(container.firstElementChild).toHaveClass('xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]');
    expect(container.querySelector('[data-master-list]')).toHaveClass('max-xl:hidden');
    expect(screen.getByRole('region', { name: 'runner-01' })).toHaveTextContent('detail');
  });
});
