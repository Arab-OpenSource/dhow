import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { PageGroup } from '@/App/Layout/PageGroup';
import { renderUi } from '@/Core/Testing/Render';

describe('PageGroup', () => {
  it('names its region after its heading', () => {
    renderUi(
      <PageGroup title="General" lead="Name and slug">
        <p>body</p>
      </PageGroup>,
    );
    expect(screen.getByRole('region', { name: 'General' })).toHaveTextContent('body');
    expect(screen.getByRole('heading', { level: 2, name: 'General' })).toBeInTheDocument();
    expect(screen.getByText('Name and slug')).toBeInTheDocument();
  });

  it('colors a danger group coral', () => {
    renderUi(
      <PageGroup title="Danger zone" icon="alert" danger>
        <p>body</p>
      </PageGroup>,
    );
    expect(screen.getByRole('region', { name: 'Danger zone' })).toHaveAttribute('data-danger');
    expect(screen.getByRole('heading', { level: 2, name: 'Danger zone' })).toHaveClass('text-bad-fg');
  });
});
